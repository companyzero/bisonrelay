package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

type newPostWindow struct {
	initless
	as *appState

	errMsg string
	debug  string

	focusIdx int
	textArea *textAreaModel

	embedContent map[string][]byte
	ew           *embedWidget

	estSize uint64
}

func (pw *newPostWindow) updateTextAreaSize() {
	// marginHeight is header+footer+estimated length comment
	marginHeight := 2 + 2 + 3
	pw.textArea.SetWidth(pw.as.winW)
	pw.textArea.SetHeight(pw.as.winH - marginHeight)
}

func (pw *newPostWindow) addEmbedCB(id string, data []byte, embedStr string) error {
	if pw.estSize+uint64(len(data)) >= rpc.MaxChunkSize {
		return fmt.Errorf("file too big to embed")
	}

	if id != "" && data != nil {
		pw.embedContent[id] = data
	}

	pw.textArea.InsertString(embedStr)
	return nil
}

func (pw *newPostWindow) createPost(post string) {
	// Replace pseudo-data with data.
	fullPost := replaceEmbeds(post, func(args embeddedArgs) string {
		data := string(args.data)
		if strings.HasPrefix(data, "[content ") {
			id := data[9 : len(args.data)-1]
			args.data = pw.embedContent[id]
		}

		return args.String()

	})
	go pw.as.createPost(fullPost)
}

func (pw newPostWindow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(pw.as, msg); ss != nil {
		return ss, cmd
	}

	// Common handlers for both main post area and embed form.

	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // resize window
		pw.as.winW = msg.Width
		pw.as.winH = msg.Height
		pw.updateTextAreaSize()

	case currentTimeChanged:
		pw.as.footerInvalidate()

	case msgPasteErr:
		pw.as.diagMsg("Unable to paste: %v", msg)
	}

	if pw.ew.active() {
		_, cmd = pw.ew.Update(msg)
		return pw, cmd
	}

	// Handlers when the main post typing form is active.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyF2:
			cmds = pw.ew.activate()

		case msg.Type == tea.KeyEsc:
			// Cancel post.
			return newMainWindowState(pw.as)

		case pw.focusIdx == 1 && msg.Type == tea.KeyEnter:
			post := pw.textArea.Value()
			if post != "" {
				go pw.createPost(post)
			}

			return newMainWindowState(pw.as)

		case msg.Type == tea.KeyTab:
			pw.focusIdx = (pw.focusIdx + 1) % 2
			if pw.focusIdx == 0 {
				pw.textArea.Focus()
			} else {
				pw.textArea.Blur()
			}

		default:
			pw.textArea, cmd = pw.textArea.Update(msg)
			cmds = appendCmd(cmds, cmd)

			var err error
			post := pw.textArea.Value()
			pw.estSize, err = clientintf.EstimatePostSize(post, "")
			if err != nil {
				pw.errMsg = err.Error()
			}
		}

	default:
		// Handle other messages.
		pw.textArea, cmd = pw.textArea.Update(msg)
		cmds = appendCmd(cmds, cmd)
	}

	return pw, batchCmds(cmds)
}

func (pw *newPostWindow) headerView() string {
	msg := " Create Post - F2 to Embed/Link File"
	headerMsg := pw.as.styles.header.Render(msg)
	spaces := pw.as.styles.header.Render(strings.Repeat(" ",
		max(0, pw.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (pw *newPostWindow) footerView() string {
	return pw.as.footerView(pw.debug)
}

func (pw newPostWindow) View() string {
	var b strings.Builder

	b.WriteString(pw.headerView())
	b.WriteString("\n\n")

	if pw.ew.active() {
		b.WriteString(pw.ew.View())
		b.WriteString(pw.footerView())
		return b.String()
	}

	b.WriteString(pw.textArea.View())
	b.WriteString("\n\n")

	if pw.errMsg != "" {
		b.WriteString(pw.as.styles.err.Render(pw.errMsg))
	} else {
		estSizeMsg := fmt.Sprintf(" Estimated post size: %s.", hbytes(int64(pw.estSize)))
		if pw.estSize > rpc.MaxChunkSize {
			estSizeMsg = pw.as.styles.err.Render(estSizeMsg)
		}
		b.WriteString(estSizeMsg)
	}
	if pw.focusIdx == 1 {
		b.WriteString(pw.as.styles.focused.Render(" [ Submit ]"))
	} else {
		b.WriteString(" [ Submit ]")
	}
	b.WriteString("\n\n")

	b.WriteString(pw.footerView())

	return b.String()
}

func newNewPostWindow(as *appState) (newPostWindow, tea.Cmd) {
	var cmds []tea.Cmd

	t := newTextAreaModel(as.styles)
	t.Placeholder = "Post"
	t.CharLimit = 0
	t.FocusedStyle.Prompt = as.styles.focused
	t.FocusedStyle.Text = as.styles.focused
	t.BlurredStyle.Prompt = as.styles.noStyle
	t.BlurredStyle.Text = as.styles.noStyle
	t.Focus()

	nw := newPostWindow{
		as:           as,
		textArea:     t,
		embedContent: make(map[string][]byte),
	}

	nw.ew = newEmbedWidget(as, nw.addEmbedCB)
	nw.updateTextAreaSize()
	return nw, batchCmds(cmds)
}
