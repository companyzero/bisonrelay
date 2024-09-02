package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

const (
	PluginInput = "input"
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

	lastUpdate time.Time // Track the last update time
}

func (pw *pluginWindow) renderPluginString(id string, embedStr string) error {
	pw.viewport.SetContent(embedStr)
	pw.as.sendMsg(repaintActiveChat{})
	return nil
}

// Convert tea.Msg to []byte.
func msgToBytes(msg tea.Msg) ([]byte, error) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		return []byte(m.String()), nil
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

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		pw.as.winW = msg.Width
		pw.as.winH = msg.Height

	case currentTimeChanged:
		pw.as.footerInvalidate()

	case msgPasteErr:
		pw.as.diagMsg("Unable to paste: %v", msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyF2:
			pw.disableHighPerformanceRendering()

		case msg.Type == tea.KeyEsc:
			pw.disableHighPerformanceRendering()
			pw.as.changeActiveWindowToPrevActive()

			return newMainWindowState(pw.as)
		default:
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
		// Throttle Sync calls to avoid flickering
		if pw.viewport.HighPerformanceRendering {
			now := time.Now()
			if now.Sub(pw.lastUpdate) > time.Millisecond*50 {
				cmds = appendCmd(cmds, viewport.Sync(pw.viewport))
				pw.lastUpdate = now
			}
		} else {
			// In low-performance mode, ensure the viewport updates normally
			pw.viewport, cmd = pw.viewport.Update(msg)
			cmds = appendCmd(cmds, cmd)
		}
	}

	return pw, batchCmds(cmds)
}

func (pw *pluginWindow) clearViewportArea() {
	// Clear the area occupied by the viewport
	b := new(strings.Builder)

	pw.viewport.SetContent(b.String())
	pw.viewport.Height = 0
	pw.viewport.Width = 0

	pw.as.sendMsg(viewport.Sync(pw.viewport))
}

func (pw *pluginWindow) disableHighPerformanceRendering() {
	if pw.viewport.HighPerformanceRendering {
		// Clear the viewport area and reset dimensions
		pw.clearViewportArea()

		// Disable high-performance rendering
		pw.viewport.HighPerformanceRendering = false

		// Set content to ensure the viewport has valid data
		pw.viewport.SetContent(pw.viewport.View())

	}
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
			b.WriteString(estSizeMsg)
		}
		b.WriteString(estSizeMsg)
	}

	b.WriteString("\n\n")

	b.WriteString(pw.footerView())

	return b.String()
}

func newPluginWindow(as *appState, uid *clientintf.PluginID) (*pluginWindow, tea.Cmd) {
	as.markWindowSeen(activeCWPlugin)

	pw := as.findOrNewPluginWindow(*uid, "")
	if pw.as.winW > 0 && pw.as.winH > 0 {
		pw.viewport.YPosition = 4
		pw.viewport.Width = pw.as.winW
		pw.viewport.Height = pw.as.winH - 4
	}

	// if commenting this line out, problems with rendering starts to happen.
	pw.viewport.HighPerformanceRendering = true
	return pw, nil
}
