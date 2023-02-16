package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrd/dcrutil/v4"
)

// chatMsgEl is one element of a chat msg AST.
type chatMsgEl struct {
	text    string
	embed   *embeddedArgs
	mention *string
}

type chatMsgElLine []chatMsgEl

func parseMsgLine(line string, mention string) chatMsgElLine {
	// Setup mention splitting.
	splitMentions := func(s string) []chatMsgEl {
		return []chatMsgEl{{text: s}}
	}
	if mention != "" {
		re, err := regexp.Compile(`\b` + mention + `\b`)
		if err == nil {
			splitMentions = func(s string) []chatMsgEl {
				positions := re.FindAllStringIndex(s, -1)
				if len(positions) == 0 {
					return []chatMsgEl{{text: s}}
				}

				var res []chatMsgEl
				lastEnd := 0
				for _, pos := range positions {
					prefix := s[lastEnd:pos[0]]
					res = append(res, chatMsgEl{text: prefix})

					mention := s[pos[0]:pos[1]]
					res = append(res, chatMsgEl{mention: &mention})
					lastEnd = pos[1]
				}
				res = append(res, chatMsgEl{text: s[lastEnd:]})
				return res
			}
		}
	}

	// Split embeds.
	var res chatMsgElLine
	embedPositions := embedRegexp.FindAllStringIndex(line, -1)
	if len(embedPositions) == 0 {
		return splitMentions(line)
	}

	// Copy [prefix]--embed[data]-
	var lastEnd int
	for _, embedPos := range embedPositions {
		prefix := line[lastEnd:embedPos[0]]
		res = append(res, splitMentions(prefix)...)

		args := parseEmbedArgs(line[embedPos[0]:embedPos[1]])
		res = append(res, chatMsgEl{embed: &args})

		lastEnd = embedPos[1]
	}

	// Copy last [suffix]
	res = append(res, splitMentions(line[lastEnd:])...)
	return res
}

func parseMsgIntoElements(msg string, mention string) []chatMsgElLine {
	// First, break into lines.
	lines := strings.Split(msg, "\n")
	res := make([]chatMsgElLine, 0, len(lines))
	for _, line := range lines {
		res = append(res, parseMsgLine(line, mention))
	}
	return res
}

type chatMsg struct {
	ts       time.Time
	sent     bool
	msg      string
	elements []chatMsgElLine
	mine     bool
	internal bool
	help     bool
	from     string
	fromUID  *clientintf.UserID
	post     *rpc.PostMetadata
}

type chatWindow struct {
	sync.Mutex
	uid          clientintf.UserID
	isGC         bool
	msgs         []*chatMsg
	alias        string
	me           string // nick of the local user
	gc           zkidentity.ShortID
	selEmbedArgs embeddedArgs
	selEmbed     int
	maxEmbeds    int
	unreadIdx    int
}

func (cw *chatWindow) empty() bool {
	cw.Lock()
	empty := len(cw.msgs) == 0
	cw.Unlock()
	return empty
}

func (cw *chatWindow) appendMsg(msg *chatMsg) {
	if msg == nil {
		return
	}
	cw.Lock()
	resetUnreadIdx := (msg.mine || msg.internal || msg.help) &&
		cw.unreadIdx == len(cw.msgs)
	cw.msgs = append(cw.msgs, msg)
	if resetUnreadIdx {
		cw.unreadIdx = len(cw.msgs)
	}
	cw.Unlock()
}

func (cw *chatWindow) newUnsentPM(msg string) *chatMsg {
	m := &chatMsg{
		mine:     true,
		elements: parseMsgIntoElements(msg, ""),
		//msg:  msg,
		ts:   time.Now(),
		from: cw.me,
	}
	cw.appendMsg(m)
	return m
}

func (cw *chatWindow) newInternalMsg(msg string) *chatMsg {
	m := &chatMsg{
		internal: true,
		elements: parseMsgIntoElements(msg, ""),
		//msg:      msg,
		ts: time.Now(),
	}
	cw.appendMsg(m)
	return m
}

func (cw *chatWindow) manyHelpMsgs(f func(printf)) {
	pf := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		m := &chatMsg{
			help:     true,
			elements: parseMsgIntoElements(msg, ""),
			//msg:  msg,
			ts: time.Now(),
		}
		cw.msgs = append(cw.msgs, m)
	}

	cw.Lock()
	f(pf)
	cw.Unlock()
}

func (cw *chatWindow) newHelpMsg(f string, args ...interface{}) {
	cw.manyHelpMsgs(func(pf printf) {
		pf(f, args...)
	})
}

func (cw *chatWindow) newRecvdMsg(from, msg string, fromUID *zkidentity.ShortID, ts time.Time) *chatMsg {

	m := &chatMsg{
		mine: false,
		//msg: msg,
		elements: parseMsgIntoElements(msg, cw.me),
		ts:       ts,
		from:     from,
		fromUID:  fromUID,
	}
	cw.appendMsg(m)
	return m
}

func (cw *chatWindow) setMsgSent(msg *chatMsg) {
	cw.Lock()
	msg.sent = true
	// TODO: move to end of messages and update time?
	cw.Unlock()
}

func (cw *chatWindow) markAllRead() {
	cw.Lock()
	cw.unreadIdx = len(cw.msgs)
	cw.Unlock()
}

func (cw *chatWindow) unreadCount() int {
	cw.Lock()
	count := len(cw.msgs) - cw.unreadIdx
	cw.Unlock()
	return count
}

func (cw *chatWindow) renderPost(winW int, styles *theme, b *strings.Builder, msg *chatMsg) {
	b.WriteString(styles.timestamp.Render(msg.ts.Format("15:04:05 ")))
	b.WriteString("<")
	b.WriteString(styles.nick.Render(cw.alias))
	b.WriteString("> ")

	post := msg.post
	title := clientintf.PostTitle(msg.post)
	if title == "" {
		// Assume client already checked this exists.
		title = post.Attributes[rpc.RMPIdentifier]
	}
	b.WriteString(styles.help.Render(fmt.Sprintf("Received post %q", title)))
	b.WriteString("\n")
}

func (cw *chatWindow) changeSelectedEmbed(delta int) bool {
	cw.Lock()
	defer cw.Unlock()

	if cw.selEmbed == 0 && delta < 0 {
		return false
	}
	if cw.selEmbed >= cw.maxEmbeds-1 && delta > 0 {
		return false
	}

	cw.selEmbed += delta
	return true
}

// writeWrappedWithStyle writes s to b using the style with wrapping at winW.
// Returns the new offset.
func writeWrappedWithStyle(b *strings.Builder, offset, winW int, style lipgloss.Style, s string) int {
	words := strings.Split(s, " ")
	var line string
	for _, w := range words {
		if len(line)+offset+len(w)+1 > winW {
			b.WriteString(style.Render(line))
			b.WriteRune('\n')
			line = ""
			offset = 0
		}
		line += w + " "
	}
	b.WriteString(style.Render(line))
	offset += len(line)
	return offset
}

func (cw *chatWindow) renderMsg(winW int, styles *theme, b *strings.Builder, as *appState, msg *chatMsg) {
	prefix := styles.timestamp.Render(msg.ts.Format("15:04:05 "))
	if msg.help {
		prefix += " "
	} else if msg.internal {
		prefix += "* "
	} else {
		prefix += "<"
		if msg.mine {
			prefix += styles.nickMe.Render(cw.me)
		} else if cw.isGC {
			prefix += styles.nickGC.Render(msg.from)
		} else {
			prefix += styles.nick.Render(msg.from)
		}
		prefix += "> "
	}

	style := styles.msg
	if msg.help {
		style = styles.help
	} else if (msg.mine || msg.internal) && !msg.sent {
		style = styles.unsent
	}

	b.WriteString(prefix)
	offset := lipgloss.Width(prefix)

	// Loop through hard newlines.
	for _, line := range msg.elements {
		// Style each element.
		for _, el := range line {
			var s string
			if el.embed != nil {
				args := el.embed
				if args.alt != "" {
					s = strescape.Content(args.alt)
					s += " "
				}

				switch {
				case args.download.IsEmpty() && (len(args.data) == 0):
					s += "[Empty link and data]"
				case args.download.IsEmpty() && args.typ == "":
					s += "[Embedded untyped data]"
				case args.download.IsEmpty():
					s += fmt.Sprintf("[Embedded data of type %q]", args.typ)
				default:
					filePath, err := as.c.HasDownloadedFile(args.download)
					filename := strescape.PathElement(args.filename)
					if filePath != "" {
						filename = filepath.Base(filePath)
					} else if filename == "" {
						filename = args.download.ShortLogID()
					}
					if err != nil {
						s += fmt.Sprintf("[Error checking file: %v", err)
					} else if filePath != "" {
						s += fmt.Sprintf("[File %s]", filename)
					} else {
						eRate := as.exchangeRate()
						dcrCost := dcrutil.Amount(int64(args.cost))
						usdCost := eRate.DCRPrice * dcrCost.ToCoin()
						s += fmt.Sprintf("[Download File %s (size:%s cost:%0.8f DCR / %0.8f USD)]",
							filename,
							hbytes(int64(args.size)),
							dcrCost.ToCoin(), usdCost)
					}
				}

				style := as.styles.embed
				if cw.maxEmbeds == cw.selEmbed {
					style = as.styles.focused
					args.uid = msg.fromUID
					cw.selEmbedArgs = *args
				}
				cw.maxEmbeds += 1
				offset = writeWrappedWithStyle(b, offset, winW, style, s)
			} else if el.mention != nil {
				style := styles.mention
				s = *el.mention
				offset = writeWrappedWithStyle(b, offset, winW, style, s)
			} else {
				s = el.text
				offset = writeWrappedWithStyle(b, offset, winW, style, s)
			}
		}
		b.WriteRune('\n')
	}
}

func (cw *chatWindow) renderContent(winW int, styles *theme, as *appState) string {
	cw.Lock()

	// TODO: estimate total length to perform only a single alloc.
	cw.maxEmbeds = 0
	b := new(strings.Builder)
	for i, msg := range cw.msgs {
		if i == cw.unreadIdx {
			unreadMsg := []byte(" unread ――――――――")
			l := utf8.RuneCount(unreadMsg)
			b.WriteString(strings.Repeat("―", winW-l))
			b.Write(unreadMsg)
			b.WriteRune('\n')
		}
		if msg.post != nil {
			cw.renderPost(winW, styles, b, msg)
			continue
		}

		cw.renderMsg(winW, styles, b, as, msg)
	}
	cw.Unlock()

	return b.String()
}
