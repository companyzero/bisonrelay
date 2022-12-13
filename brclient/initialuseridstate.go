package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type initialUIDState struct {
	initless
	as *appState

	focusIndex int
	inputs     []textinput.Model
}

func (ws *initialUIDState) updateFocused(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	i := ws.focusIndex
	if i < len(ws.inputs) {
		ws.inputs[i], cmd = ws.inputs[i].Update(msg)
	}
	return cmd
}

func (ws initialUIDState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(ws.as, msg); ss != nil {
		return ss, cmd
	}

	// Main handler for the initialUIDState. Only returns early if we're
	// switching the state, otherwise returns the updated state at the end
	// of the function.
	handleOnFocused := false
	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // resize window
		ws.as.winW = msg.Width
		ws.as.winH = msg.Height

	case tea.KeyMsg:
		switch msg.String() {

		case "tab", "shift+tab", "enter", "up", "down":
			// Set focus to next input
			s := msg.String()

			// Did the user press enter while the submit button was focused?
			// If so, exit.
			if s == "enter" && ws.focusIndex == len(ws.inputs) {
				go func() {
					ws.as.clientIDChan <- getClientIDReply{
						nick: ws.inputs[0].Value(),
						name: ws.inputs[0].Value(),
					}
				}()
				return initStepState{
					as: ws.as,
				}, nil
			}

			// Cycle indexes
			if s == "up" || s == "shift+tab" {
				ws.focusIndex--
			} else {
				ws.focusIndex++
			}

			if ws.focusIndex > len(ws.inputs) {
				ws.focusIndex = 0
			} else if ws.focusIndex < 0 {
				ws.focusIndex = len(ws.inputs)
			}

			for i := 0; i <= len(ws.inputs)-1; i++ {
				if i == ws.focusIndex {
					// Set focused state
					cmd = ws.inputs[i].Focus()
					cmds = appendCmd(cmds, cmd)
					ws.inputs[i].PromptStyle = ws.as.styles.focused
					ws.inputs[i].TextStyle = ws.as.styles.focused
					continue
				}
				// Remove focused state
				ws.inputs[i].Blur()
				ws.inputs[i].PromptStyle = ws.as.styles.noStyle
				ws.inputs[i].TextStyle = ws.as.styles.noStyle
			}
		default:
			// Handle other keys in the focused input.
			handleOnFocused = true
		}
	default:
		// Handle blink and other msgs in the focused input.
		handleOnFocused = true
	}

	if handleOnFocused {
		cmd := ws.updateFocused(msg)
		cmds = appendCmd(cmds, cmd)
	}

	return ws, batchCmds(cmds)
}

func (ws initialUIDState) headerView() string {
	msg := " Local User Information"
	headerMsg := ws.as.styles.header.Render(msg)
	spaces := ws.as.styles.header.Render(strings.Repeat(" ",
		max(0, ws.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (ws initialUIDState) footerView() string {
	footerMsg := fmt.Sprintf(
		" [%s] ",
		time.Now().Format("15:04"),
	)
	fs := ws.as.styles.footer
	spaces := fs.Render(strings.Repeat(" ",
		max(0, ws.as.winW-lipgloss.Width(footerMsg))))
	return fs.Render(footerMsg + spaces)
}

func (ws initialUIDState) View() string {
	var b strings.Builder

	b.WriteString(ws.headerView())
	b.WriteString("\n\n")
	b.WriteString("Type the information needed about the local user.\n\n")

	for i := range ws.inputs {
		b.WriteString(ws.inputs[i].View())
		if i < len(ws.inputs)-1 {
			b.WriteRune('\n')
		}
	}

	btnStyle := ws.as.styles.blurred
	if ws.focusIndex == len(ws.inputs) {
		btnStyle = ws.as.styles.focused
	}
	button := btnStyle.Render("[ Submit ]")
	fmt.Fprintf(&b, "\n\n%s\n\n", button)

	nbLines := 1 + len(ws.inputs)*2 + 4 + 2
	for i := 0; i < ws.as.winH-nbLines; i++ {
		b.WriteString("\n")
	}
	b.WriteString(ws.footerView())

	return b.String()
}

func newInitialUIDState(as *appState) (initialUIDState, tea.Cmd) {
	inputs := make([]textinput.Model, 1)
	var t textinput.Model
	for i := range inputs {
		t = textinput.New()
		t.CursorStyle = as.styles.cursor
		t.CharLimit = 32

		switch i {
		case 0:
			t.Placeholder = "Nickname"
			t.Focus()
			t.PromptStyle = as.styles.focused
			t.TextStyle = as.styles.focused
		}

		inputs[i] = t
	}
	cmd := inputs[0].SetCursorMode(textinput.CursorBlink)

	return initialUIDState{
		as:     as,
		inputs: inputs,
	}, cmd
}
