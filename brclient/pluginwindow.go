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

const PluginInput = "input"

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

// Convert tea.Msg to []byte.
func msgToBytes(msg tea.Msg) ([]byte, error) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		return []byte(fmt.Sprintf("%s", m.String())), nil
	case tea.WindowSizeMsg:
		return []byte(fmt.Sprintf("WindowSizeMsg: %dx%d", m.Width, m.Height)), nil
	// Add more case statements as needed for other tea.Msg types.
	default:
		return nil, fmt.Errorf("unsupported message type: %T", msg)
	}
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
		default:
			// Convert `msg` to `[]byte` if possible.
			msgBytes, err := msgToBytes(msg)
			if err != nil {
				pw.errMsg = fmt.Sprintf("Failed to convert msg to bytes: %v", err)
			} else {
				if err := pw.as.pluginAction(pw, pw.uid, PluginInput, msgBytes); err != nil {
					pw.errMsg = fmt.Sprintf("Plugin action error: %v", err)
				}
			}
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

func (pw *pluginWindow) renderPlugin() {

	if pw.as.winW > 0 && pw.as.winH > 0 {
		pw.viewport.YPosition = 4
		pw.viewport.Width = pw.as.winW
		pw.viewport.Height = pw.as.winH - 4
	}

	var minOffset, maxOffset int
	b := new(strings.Builder)

	pw.viewport.SetContent(b.String())

	// Ensure the currently selected index is visible.
	if pw.viewport.YOffset > minOffset {
		// Move viewport up until top of selected item is visible.
		pw.viewport.SetYOffset(minOffset)
	} else if bottom := pw.viewport.YOffset + pw.viewport.Height; bottom < maxOffset {
		// Move viewport down until bottom of selected item is visible.
		pw.viewport.SetYOffset(pw.viewport.YOffset + (maxOffset - bottom))
	}
}

func newPluginWindow(as *appState, uid *clientintf.PluginID) (*pluginWindow, tea.Cmd) {
	as.markWindowSeen(activeCWPlugin)
	// pw := pluginWindow{as: as, uid: *uid}

	pw := as.findOrNewPluginWindow(*uid, "")
	pw.renderPlugin()
	return pw, nil
}
