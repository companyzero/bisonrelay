package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	genericlist "github.com/bahlo/generic-list-go"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/mdembeds"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/zpay32"
)

var (
	// The following parameters are only used during invoice decoding
	// when rendering chat items.
	mainnetParams = chaincfg.MainNetParams()
	testnetParams = chaincfg.TestNet3Params()
	simnetParams  = chaincfg.SimNetParams()
)

type formField struct {
	typ       string
	name      string
	label     string
	regexp    string
	regExpStr string
	err       error
	value     interface{}
}

func (ff *formField) inputable() bool {
	switch ff.typ {
	case "intinput", "txtinput":
		return true
	default:
		return false
	}

}

func (ff *formField) viewable() bool {
	switch ff.typ {
	case "intinput", "submit", "txtinput":
		return true
	default:
		return false
	}
}

func checkRegex(s string, regExp string) error {
	re, err := regexp.Compile(regExp)
	if err != nil {
		return fmt.Errorf("provided regexp (%s) not not valid: %v",
			regExp, err)
	}
	if !re.MatchString(s) {
		return fmt.Errorf("invalid field characters: not valid: %s %s",
			s, regExp)
	}
	return nil
}
func (ff *formField) resetInputModel(m *textinput.Model) {
	switch ff.typ {
	case "txtinput":
		m.SetValue(fmt.Sprintf("%s", ff.value))
	case "intinput":
		m.SetValue(fmt.Sprintf("%d", ff.value))
	}
}

func (ff *formField) updateInputModel(m *textinput.Model, msg tea.Msg) bool {
	oldVal := m.Value()
	switch ff.typ {
	case "txtinput":
		*m, _ = m.Update(msg)
		newVal := m.Value()
		ff.value = newVal
		if ff.regexp != "" {
			ff.err = checkRegex(newVal, ff.regexp)
		}
		return newVal != oldVal
	case "intinput":
		*m, _ = m.Update(msg)
		newVal := m.Value()
		if newVal == "" {
			ff.value = int64(0)
		} else if i, err := strconv.ParseInt(newVal, 10, 64); err == nil {
			ff.value = i
		} else {
			m.SetValue(oldVal)
			newVal = oldVal
		}
		return newVal != oldVal
	default:
	}

	return false
}

func (ff *formField) view() string {
	var b strings.Builder
	if ff.label != "" {
		b.WriteString(ff.label)
		if ff.inputable() {
			b.WriteString(": ")
		}
	}
	switch ff.typ {
	case "txtinput":
		b.WriteString(fmt.Sprintf("%s", ff.value))
		if ff.err != nil {
			b.WriteString(fmt.Sprintf("\n Invalid %s: %s", ff.label, ff.regExpStr))
		}
	case "intinput":
		b.WriteString(fmt.Sprintf("%d", ff.value))
	default:
		if ff.value != nil {
			b.WriteString(fmt.Sprintf("%v", ff.value))
		}
	}

	return b.String()
}

var formFieldPattern = regexp.MustCompile(`([\w]+)="([^"]*)"`)

func parseFormField(line string) *formField {
	matches := formFieldPattern.FindAllStringSubmatch(line, -1)

	ff := &formField{}
	var value string
	hasValue := false
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		switch m[1] {
		case "type":
			ff.typ = m[2]
		case "name":
			ff.name = m[2]
		case "value":
			value = m[2]
			hasValue = true
		case "label":
			ff.label = m[2]
		case "regexp":
			ff.regexp = m[2]
		case "regexpstr":
			ff.regExpStr = m[2]
		}
	}

	if ff.typ == "" {
		return nil
	}

	switch ff.typ {
	case "txtinput":
		ff.value = value
	case "intinput":
		ff.value, _ = strconv.ParseInt(value, 10, 64)
	default:
		if hasValue {
			ff.value = value
		}
	}

	return ff
}

type formEl struct {
	fields []*formField
}

// asyncTarget returns the id of the target when the form has an asynctarget
// field.
func (f *formEl) asyncTarget() string {
	for _, ff := range f.fields {
		if ff.typ == "asynctarget" && ff.value != nil {
			if target, ok := ff.value.(string); ok {
				return target
			}
		}
	}

	return ""
}

func (f *formEl) action() string {
	for _, ff := range f.fields {
		if ff.typ == "action" && ff.value != nil {
			action, _ := ff.value.(string)
			return action
		}
	}
	return ""
}

func (f *formEl) toJson() (json.RawMessage, error) {
	m := make(map[string]interface{})
	for _, ff := range f.fields {
		if ff.name == "" || ff.value == nil {
			continue
		}
		m[ff.name] = ff.value
	}

	res, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(res), nil
}

// chatMsgEl is one element of a chat msg AST.
type chatMsgEl struct {
	text      string
	embed     *mdembeds.EmbeddedArgs
	mention   *string
	payReq    *zpay32.Invoice
	url       *string
	link      *string
	form      *formEl
	formField *formField
}

type chatMsgElLine struct {
	genericlist.List[chatMsgEl]
}

func (l *chatMsgElLine) splitMentions(mention string) {
	re, err := regexp.Compile(`\b` + mention + `\b`)
	if err != nil {
		return
	}
	for el := l.Front(); el != nil; el = el.Next() {
		s := el.Value.text
		if s == "" {
			continue
		}

		positions := re.FindAllStringIndex(s, -1)
		if len(positions) == 0 {
			continue
		}

		// Replace el with new elements.
		lastEnd := 0
		for _, pos := range positions {
			prefix := s[lastEnd:pos[0]]
			l.InsertBefore(chatMsgEl{text: prefix}, el)

			mention := s[pos[0]:pos[1]]
			l.InsertBefore(chatMsgEl{mention: &mention}, el)
			lastEnd = pos[1]
		}
		suffix := s[lastEnd:]
		newEl := l.InsertBefore(chatMsgEl{text: suffix}, el)
		l.Remove(el)
		el = newEl
	}
}

func (l *chatMsgElLine) splitEmbeds() {
	for el := l.Front(); el != nil; el = el.Next() {
		s := el.Value.text
		if s == "" {
			continue
		}

		embedPositions := mdembeds.FindAllStringIndex(s)
		if len(embedPositions) == 0 {
			continue
		}

		// Copy [prefix]--embed[data]-
		var lastEnd int
		for _, embedPos := range embedPositions {
			prefix := s[lastEnd:embedPos[0]]
			l.InsertBefore(chatMsgEl{text: prefix}, el)

			args := mdembeds.ParseEmbedArgs(s[embedPos[0]:embedPos[1]])
			l.InsertBefore(chatMsgEl{embed: &args}, el)

			lastEnd = embedPos[1]
		}

		// Copy last [suffix]
		newEl := l.InsertBefore(chatMsgEl{text: s[lastEnd:]}, el)
		l.Remove(el)
		el = newEl
	}
}

var urlRegexp = regexp.MustCompile(`\b(https?|lnpay)://[^\s]*\b`)

func (l *chatMsgElLine) splitURLs() {
	for el := l.Front(); el != nil; el = el.Next() {
		s := el.Value.text
		if s == "" {
			continue
		}

		positions := urlRegexp.FindAllStringIndex(s, -1)
		if len(positions) == 0 {
			continue
		}

		// Replace el with new elements.
		lastEnd := 0
		for _, pos := range positions {
			prefix := s[lastEnd:pos[0]]
			l.InsertBefore(chatMsgEl{text: prefix}, el)

			url := s[pos[0]:pos[1]]

			var inv *zpay32.Invoice
			var err error
			if strings.HasPrefix(url, "lnpay://lnsdcr") {
				inv, err = zpay32.Decode(url[8:], simnetParams)
			} else if strings.HasPrefix(url, "lnpay://lntdcr") {
				inv, err = zpay32.Decode(url[8:], testnetParams)
			} else if strings.HasPrefix(url, "lnpay://lndcr") {
				inv, err = zpay32.Decode(url[8:], mainnetParams)
			}
			if err != nil {
				internalLog.Warnf("Unable to decode invoice %q: %v", s, err)
			}
			if inv != nil && inv.PaymentHash == nil {
				// Ignore when paymentHash is nil as there's nothing to pay.
				inv = nil
			}
			if inv != nil {
				url = url[8:]
			}

			l.InsertBefore(chatMsgEl{url: &url, payReq: inv}, el)
			lastEnd = pos[1]
		}
		suffix := s[lastEnd:]
		newEl := l.InsertBefore(chatMsgEl{text: suffix}, el)
		l.Remove(el)
		el = newEl
	}
}

var (
	// linksRegexp is a regexp that detects markdown links.
	linksRegexp = regexp.MustCompile(`\[[^\[\]]*\]\([^()]*\)`)

	// descrPathRegexp is a regexp that splits markdown links into both
	// link and description.
	descrPathRegexp = regexp.MustCompile(`\[([^\[\]]*)\]\(([^()]*)\)`)
)

func (l *chatMsgElLine) splitLinks() {
	for el := l.Front(); el != nil; el = el.Next() {
		s := el.Value.text
		if s == "" {
			continue
		}

		positions := linksRegexp.FindAllStringIndex(s, -1)
		if len(positions) == 0 {
			continue
		}

		// Replace el with new elements.
		lastEnd := 0
		for _, pos := range positions {
			prefix := s[lastEnd:pos[0]]
			l.InsertBefore(chatMsgEl{text: prefix}, el)

			rawLink := s[pos[0]:pos[1]]
			link := descrPathRegexp.FindAllStringSubmatch(rawLink, -1)
			cmel := chatMsgEl{
				text: link[0][1],
				link: &(link[0][2]),
			}

			l.InsertBefore(cmel, el)
			lastEnd = pos[1]
		}
		suffix := s[lastEnd:]
		newEl := l.InsertBefore(chatMsgEl{text: suffix}, el)
		l.Remove(el)
		el = newEl
	}
}

func (l *chatMsgElLine) parseLine(line, mention string) {
	l.PushBack(chatMsgEl{text: line})
	l.splitEmbeds()
	l.splitURLs()
	l.splitLinks()
	if mention != "" {
		l.splitMentions(mention)
	}
}

func parseMsgLine(line string, mention string) *chatMsgElLine {
	res := &chatMsgElLine{}
	res.parseLine(line, mention)
	return res
}

var (
	sectionStartRegexp = regexp.MustCompile(`--section id=([\w]+) --`)
	sectionEndRegexp   = regexp.MustCompile(`--/section--`)
)

func parseMsgIntoElements(msg string, mention string) []*chatMsgElLine {
	// First, break into lines.
	lines := strings.Split(msg, "\n")
	res := make([]*chatMsgElLine, 0, len(lines))
	var form *formEl
	for _, line := range lines {
		switch {
		case line == "--form--":
			form = &formEl{}
		case line == "--/form--":
			form = nil
		case sectionStartRegexp.MatchString(line):
			// Skip section start line
		case sectionEndRegexp.MatchString(line):
			// Skip section end line
		case form != nil:
			ff := parseFormField(line)
			if ff != nil {
				form.fields = append(form.fields, ff)
			}
			if ff != nil && ff.viewable() {
				msgEl := chatMsgEl{form: form, formField: ff}
				el := &chatMsgElLine{}
				el.PushBack(msgEl)
				res = append(res, el)
			}

		default:
			res = append(res, parseMsgLine(line, mention))
		}
	}
	return res
}

type chatMsg struct {
	ts       time.Time
	sent     bool
	msg      string
	elements []*chatMsgElLine
	mine     bool
	internal bool
	help     bool
	from     string
	fromUID  *clientintf.UserID
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

	initTime time.Time // When the cw was created and history read.

	pageSess      clientintf.PagesSessionID
	page          *clientdb.FetchedResource
	isPage        bool
	pageRequested *[]string
	pageSpinner   spinner.Model

	selEl         *chatMsgEl
	selElIndex    int
	maxSelectable int

	unreadIdx int
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

func (cw *chatWindow) appendHistoryMsg(msg *chatMsg) {
	if msg == nil {
		return
	}
	cw.Lock()
	cw.msgs = append(cw.msgs, msg)
	cw.unreadIdx = len(cw.msgs)
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

func (cw *chatWindow) newInternalMsg(msg string, args ...interface{}) *chatMsg {
	m := &chatMsg{
		internal: true,
		elements: parseMsgIntoElements(fmt.Sprintf(msg, args...), ""),
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

func (cw *chatWindow) replaceAsyncTargetWithLoading(asyncTargetID string) {
	cw.Lock()
	defer cw.Unlock()

	if len(cw.msgs) == 0 {
		return
	}
	if cw.page == nil {
		return
	}

	data := cw.page.Response.Data

	reStartPattern := `--section id=` + asyncTargetID + ` --\n`
	reStart, err := regexp.Compile(reStartPattern)
	if err != nil {
		// Skip invalid ids.
		return
	}

	startPos := reStart.FindIndex(data)
	if startPos == nil {
		// Did not find the target location.
		return
	}

	endPos := sectionEndRegexp.FindIndex(data[startPos[1]:])
	if endPos == nil {
		// Unterminated section.
		return
	}
	endPos[0] += startPos[1] // Convert to absolute index

	// Copy the rest of the string to an aux buffer.
	aux := append([]byte(nil), data[endPos[0]:]...)

	// Create the new buffer, replacing the contents inside
	// the section with this response.
	data = data[0:startPos[1]]
	data = append(data, []byte("(⏳ Loading response)\n")...)
	data = append(data, aux...)

	msg := cw.msgs[0]
	msg.elements = parseMsgIntoElements(string(data), "")
	cw.page.Response.Data = data
}

func (cw *chatWindow) replacePage(nick string, fr clientdb.FetchedResource, history []*clientdb.FetchedResource) {
	cw.Lock()
	var msg *chatMsg
	if len(cw.msgs) == 0 {
		msg = &chatMsg{}
		cw.msgs = []*chatMsg{msg}
	} else {
		msg = cw.msgs[0]
	}

	// Replace async targets.
	var data, aux []byte
	if len(history) > 0 || fr.AsyncTargetID != "" {
		// If there is history, this is loading from disk, so use only
		// whats in the slice. Otherwise, replace the response data.
		var toProcess []*clientdb.FetchedResource
		if len(history) > 0 {
			data = history[0].Response.Data
			toProcess = history[1:]
		} else {
			data = cw.page.Response.Data
			toProcess = []*clientdb.FetchedResource{&fr}
		}

		// Process the async targets.
		for _, asyncRes := range toProcess {
			reStartPattern := `--section id=` + asyncRes.AsyncTargetID + ` --\n`
			reStart, err := regexp.Compile(reStartPattern)
			if err != nil {
				// Skip invalid ids.
				continue
			}

			startPos := reStart.FindIndex(data)
			if startPos == nil {
				// Did not find the target location.
				continue
			}

			endPos := sectionEndRegexp.FindIndex(data[startPos[1]:])
			if endPos == nil {
				// Unterminated section.
				continue
			}
			endPos[0] += startPos[1] // Convert to absolute index

			// Copy the rest of the string to an aux buffer.
			aux = append(aux, data[endPos[0]:]...)

			// Create the new buffer, replacing the contents inside
			// the section with this response.
			data = data[0:startPos[1]]
			data = append(data, asyncRes.Response.Data...)
			data = append(data, aux...)
			aux = aux[:0]
		}
	} else {
		data = fr.Response.Data
	}

	msg.elements = parseMsgIntoElements(string(data), "")
	msg.fromUID = &fr.UID
	if len(history) > 0 {
		cw.page = history[0]
	} else if fr.AsyncTargetID == "" {
		cw.page = &fr
	}
	cw.page.Response.Data = data
	if history != nil || fr.AsyncTargetID == "" {
		// Only reset the selected element index when replacing the
		// entire page.
		cw.selElIndex = 0
	}
	cw.pageRequested = nil
	cw.alias = fmt.Sprintf("%v/%v", nick, strings.Join(fr.Request.Path, "/"))
	cw.Unlock()
}

func (cw *chatWindow) newHistoryMsg(from, msg string, fromUID *zkidentity.ShortID,
	ts time.Time, mine, internal bool) *chatMsg {
	m := &chatMsg{
		mine: mine,
		//msg: msg,
		elements: parseMsgIntoElements(msg, cw.me),
		ts:       ts,
		from:     from,
		fromUID:  fromUID,
		sent:     true,
		internal: internal,
	}
	cw.appendHistoryMsg(m)
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

func (cw *chatWindow) changeSelected(delta int) bool {
	cw.Lock()
	defer cw.Unlock()

	if cw.selElIndex == 0 && delta < 0 {
		return false
	}
	if cw.selElIndex >= cw.maxSelectable-1 && delta > 0 {
		return false
	}

	cw.selElIndex += delta
	return true
}

// writeWrappedWithStyle writes s to b using the style with wrapping at winW.
// Returns the new offset.
func writeWrappedWithStyle(b *strings.Builder, offset, winW int, style lipgloss.Style, s string) int {
	words := strings.SplitAfter(s, " ")
	var line string
	for _, w := range words {
		if len(line)+offset+len(w)+1 > winW {
			b.WriteString(style.Render(line))
			b.WriteRune('\n')
			line = ""
			offset = 0
		}
		line += w
	}
	b.WriteString(style.Render(line))
	offset += len(line)
	return offset
}

func writeWrappedURL(b *strings.Builder, offset, winW int, url string) int {
	if offset+len(url) > winW {
		b.WriteRune('\n')
		offset = 0
		if len(url) > winW {
			url = url[:winW]
		}
	}

	// This is supposed to work, but doesn't when the URL is long.
	// Bubbletea bug or terminal bug? see:
	// https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
	//
	//        OSC 8      url      ST     text     OSC 8   ST
	// s := "\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"
	b.WriteString(url)
	offset += len(url)
	if offset > winW {
		offset = 0
	}
	return offset
}

func writePayReq(b *strings.Builder, offset, winW int, payReq *zpay32.Invoice, style lipgloss.Style, as *appState) int {
	amt := payReqStrAmount(payReq)

	status, ok := as.payReqStatuses.Load(*payReq.PaymentHash)
	if ok && status == lnrpc.Payment_IN_FLIGHT {
		s := fmt.Sprintf("[ %s payment in-flight ]", amt)
		return writeWrappedWithStyle(b, offset, winW, style, s)
	}
	if ok && status == lnrpc.Payment_FAILED {
		s := fmt.Sprintf("[ %s payment failed ]", amt)
		return writeWrappedWithStyle(b, offset, winW, style, s)
	}
	if ok && status == lnrpc.Payment_SUCCEEDED {
		s := fmt.Sprintf("[ %s payment succeeded ]", amt)
		return writeWrappedWithStyle(b, offset, winW, style, s)
	}

	if isPayReqExpired(payReq) {
		s := fmt.Sprintf("[ %s invoice expired ]", amt)
		return writeWrappedWithStyle(b, offset, winW, style, s)
	}

	var descr string
	if payReq.Description != nil && (*payReq.Description != "") {
		descr = fmt.Sprintf(" (%q)", *payReq.Description)
	}

	s := fmt.Sprintf("[ Pay %s invoice%s ]", amt, descr)
	return writeWrappedWithStyle(b, offset, winW, style, s)
}

func (cw *chatWindow) renderMsgElements(winW int, as *appState, elements []*chatMsgElLine,
	fromUID *clientintf.UserID, style lipgloss.Style, b *strings.Builder, offset int) {

	styles := as.styles.Load()

	formFieldErrs := false
	// Loop through hard newlines.
	for _, line := range elements {
		// Style each element.
		for elel := line.Front(); elel != nil; elel = elel.Next() {
			el := elel.Value
			var s string
			if el.embed != nil {
				args := el.embed
				if args.Alt != "" {
					s = strescape.Content(args.Alt)
					s += " "
				}

				s += as.prettyArgs(args)

				style := styles.embed
				if cw.maxSelectable == cw.selElIndex {
					style = styles.focused
					args.Uid = fromUID
					cw.selEl = &el
				}
				cw.maxSelectable += 1
				offset = writeWrappedWithStyle(b, offset, winW, style, s)
			} else if el.link != nil {
				style := styles.embed
				if cw.maxSelectable == cw.selElIndex {
					style = styles.focused
					cw.selEl = &el
				}
				cw.maxSelectable += 1
				s := el.text
				offset = writeWrappedWithStyle(b, offset, winW, style, s)
			} else if el.mention != nil {
				style := styles.mention
				s = *el.mention
				offset = writeWrappedWithStyle(b, offset, winW, style, s)
			} else if el.payReq != nil {
				style := styles.embed
				if cw.maxSelectable == cw.selElIndex {
					style = styles.focused
					cw.selEl = &el
				}
				cw.maxSelectable += 1
				offset = writePayReq(b, offset, winW, el.payReq, style, as)
			} else if el.url != nil {
				offset = writeWrappedURL(b, offset, winW, *el.url)
			} else if el.formField != nil && el.formField.label != "" {
				style := styles.embed
				if cw.maxSelectable == cw.selElIndex {
					style = styles.focused
					cw.selEl = &el
				}
				// Don't allow submitting if any errors
				if el.formField.err != nil {
					formFieldErrs = true
				}
				if el.formField.typ == "submit" && formFieldErrs {
					style = styles.blurred
				}
				cw.maxSelectable += 1
				s := el.formField.view()
				offset = writeWrappedWithStyle(b, offset, winW, style, s)
			} else {
				s = el.text
				offset = writeWrappedWithStyle(b, offset, winW, style, s)
			}

			// Uncomment to debug element separtions.
			// b.WriteString("¶")
			// offset += 1
		}
		b.WriteRune('\n')
		offset = 0
	}

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

	cw.renderMsgElements(winW, as, msg.elements, msg.fromUID, style, b, offset)
}

func (cw *chatWindow) renderPageHeader(as *appState, b *strings.Builder) {
	if cw.page == nil {
		return
	}

	var reqPath string
	if cw.pageRequested != nil {
		reqPath = strings.Join(*cw.pageRequested, "/")
	} else {
		reqPath = strings.Join(cw.page.Request.Path, "/")
	}

	nick, _ := as.c.UserNick(cw.page.UID)
	fmt.Fprintf(b, "Source : %s (%s)\n", strescape.Nick(nick), cw.page.UID)
	fmt.Fprintf(b, "Path   : %s\n", strescape.Nick(reqPath))
	fmt.Fprintf(b, "%s\n", strings.Repeat("\xe2\x80\x95", as.winW))
	if cw.pageRequested != nil {
		fmt.Fprintf(b, "%s Fetching page...\n", cw.pageSpinner.View())
	}
}

func (cw *chatWindow) renderPage(winW int, as *appState, b *strings.Builder) {
	style := as.styles.Load().msg

	cw.renderPageHeader(as, b)
	if len(cw.msgs) > 0 {
		// msg[0] is the parsed contents of the page.
		cw.renderMsgElements(winW, as, cw.msgs[0].elements, cw.msgs[0].fromUID, style, b, 0)
	}
}

func (cw *chatWindow) renderContent(winW int, styles *theme, as *appState) string {
	cw.Lock()

	// TODO: estimate total length to perform only a single alloc.
	cw.maxSelectable = 0
	b := new(strings.Builder)

	if cw.page != nil {
		cw.renderPage(winW, as, b)
		cw.Unlock()
		return b.String()
	}

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
