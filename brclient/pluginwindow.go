package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

type pluginWindow struct {
	initless

	uid   clientintf.PluginID
	alias string
	me    string // nick of the local user
	as    *appState

	errMsg string
	debug  string

	estSize uint64

	viewport viewport.Model
}

func (pw *pluginWindow) renderPluginString(id string, embedStr string) error {
	// if pw.estSize+uint64(len(data)) >= rpc.MaxChunkSize {
	// 	return fmt.Errorf("file too big to embed")
	// }
	pw.viewport.SetContent(embedStr)
	pw.as.sendMsg(repaintActiveChat{})

	return nil
}

func (pw *pluginWindow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case currentTimeChanged:
		pw.as.footerInvalidate()

	case msgPasteErr:
		pw.as.diagMsg("Unable to paste: %v", msg)
	}

	// Handlers when the main post typing form is active.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyF2:
			// cmds = pw.ew.activate()

		case msg.Type == tea.KeyEsc:
			// Cancel post.
			return newMainWindowState(pw.as)

		}

	default:
		// Handle other messages.
		pw.viewport, cmd = pw.viewport.Update(msg)
		cmds = appendCmd(cmds, cmd)
	}

	return pw, batchCmds(cmds)
}

func (pw *pluginWindow) headerView(styles *theme) string {
	msg := " Plugin Window - F2 to Embed/Link File"
	headerMsg := styles.header.Render(msg)
	spaces := styles.header.Render(strings.Repeat(" ",
		max(0, pw.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (pw *pluginWindow) footerView() string {
	return pw.as.footerView(pw.as.styles.Load(), pw.debug)
}

func (pw pluginWindow) View() string {
	styles := pw.as.styles.Load()

	var b strings.Builder
	b.WriteString(pw.headerView(styles))
	b.WriteString("\n\n")

	b.WriteString(pw.viewport.View())

	b.WriteString("\n\n")

	if pw.errMsg != "" {
		b.WriteString(pw.errMsg)
	} else {
		estSizeMsg := fmt.Sprintf(" Estimated post size: %s.", hbytes(int64(pw.estSize)))
		if pw.estSize > rpc.MaxChunkSize {
			// estSizeMsg = styles.err.Render(estSizeMsg)
			b.WriteString(estSizeMsg)
		}
		b.WriteString(estSizeMsg)
	}

	b.WriteString("\n\n")

	b.WriteString(pw.footerView())

	return b.String()
}

func (fw *pluginWindow) renderPlugin() {
	if fw.as.winW > 0 && fw.as.winH > 0 {
		fw.viewport.YPosition = 4
		fw.viewport.Width = fw.as.winW
		fw.viewport.Height = fw.as.winH - 4
	}

	var minOffset, maxOffset int
	b := new(strings.Builder)

	fw.viewport.SetContent(b.String())

	// Ensure the currently selected index is visible.
	if fw.viewport.YOffset > minOffset {
		// Move viewport up until top of selected item is visible.
		fw.viewport.SetYOffset(minOffset)
	} else if bottom := fw.viewport.YOffset + fw.viewport.Height; bottom < maxOffset {
		// Move viewport down until bottom of selected item is visible.
		fw.viewport.SetYOffset(fw.viewport.YOffset + (maxOffset - bottom))
	}
}

func newPluginWindow(as *appState, uid *clientintf.PluginID) (*pluginWindow, tea.Cmd) {
	as.markWindowSeen(activeCWPlugin)
	// pw := pluginWindow{as: as, uid: *uid}

	pw := as.findOrNewPluginWindow(*uid, "")
	pw.renderPlugin()
	return pw, nil
}
