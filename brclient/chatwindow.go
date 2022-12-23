package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/muesli/reflow/wordwrap"
)

type chatMsg struct {
	ts       time.Time
	sent     bool
	msg      string
	mine     bool
	internal bool
	help     bool
	from     string
	post     *rpc.PostMetadata
}

type chatWindow struct {
	sync.Mutex
	uid   clientintf.UserID
	isGC  bool
	msgs  []*chatMsg
	alias string
	me    string // nick of the local user
	gc    zkidentity.ShortID
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
	cw.msgs = append(cw.msgs, msg)
	cw.Unlock()
}

func (cw *chatWindow) newUnsentPM(msg string) *chatMsg {
	m := &chatMsg{
		mine: true,
		msg:  msg,
		ts:   time.Now(),
		from: cw.me,
	}
	cw.appendMsg(m)
	return m
}

func (cw *chatWindow) newInternalMsg(msg string) *chatMsg {
	m := &chatMsg{
		internal: true,
		msg:      msg,
		ts:       time.Now(),
	}
	cw.appendMsg(m)
	return m
}

func (cw *chatWindow) manyHelpMsgs(f func(printf)) {
	pf := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		m := &chatMsg{
			help: true,
			msg:  msg,
			ts:   time.Now(),
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

func (cw *chatWindow) newRecvdMsg(from, msg string, ts time.Time) *chatMsg {
	m := &chatMsg{
		mine: false,
		msg:  msg,
		ts:   ts,
		from: from,
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

func (cw *chatWindow) renderContent(winW int, styles *theme) string {
	cw.Lock()
	// TODO: estimate total length to perform only a single alloc.

	b := new(strings.Builder)
	for _, msg := range cw.msgs {

		if msg.post != nil {
			cw.renderPost(winW, styles, b, msg)
			continue
		}

		prefix := styles.timestamp.Render(msg.ts.Format("15:04:05 "))
		wrapW := winW
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

		// Render the entire msg. Needed because the prefix breaks
		// styling in the first line when wrapping is needed.
		renderedMsg := style.Render(msg.msg)
		lines := strings.Split(prefix+renderedMsg, "\n")
		for _, line := range lines {
			// Wrap on the window.
			wrapper := wordwrap.NewWriter(wrapW)
			wrapper.Breakpoints = []rune{}
			wrapper.Write([]byte(line))
			wrapper.Close()
			wrapped := string(wrapper.Bytes())

			for _, line := range strings.Split(wrapped, "\n") {
				// Highlight mentions (legacy method).
				mp := -1
				if !msg.mine && !msg.internal && !msg.help {
					mp = mentionPosition(cw.me, line)
				}
				if mp > -1 {
					line = style.Render(line[:mp]) +
						styles.mention.Render(cw.me) +
						style.Render(line[mp+len(cw.me):])
				} else {
					line = style.Render(line)
				}
				b.WriteString(line)
				b.WriteRune('\n')
			}
		}
	}
	cw.Unlock()

	return b.String()
}
