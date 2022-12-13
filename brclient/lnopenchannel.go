package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type lnOpenChannelWindow struct {
	initless
	as *appState

	focusIndex  int
	inputs      []textinput.Model
	requesting  bool
	openChanErr error
	isSetup     bool // Whether this was triggered during initial setup
}

func (pw *lnOpenChannelWindow) setFocus(fi int) []tea.Cmd {
	var cmds []tea.Cmd
	if fi > len(pw.inputs) {
		return nil
	}

	pw.focusIndex = fi
	for i := 0; i <= len(pw.inputs)-1; i++ {
		if i == pw.focusIndex {
			// Set focused state
			cmd := pw.inputs[i].Focus()
			cmds = appendCmd(cmds, cmd)
			pw.inputs[i].PromptStyle = pw.as.styles.focused
			pw.inputs[i].TextStyle = pw.as.styles.focused
			continue
		}
		// Remove focused state
		pw.inputs[i].Blur()
		pw.inputs[i].PromptStyle = pw.as.styles.noStyle
		pw.inputs[i].TextStyle = pw.as.styles.noStyle
	}

	return cmds
}

func (pw *lnOpenChannelWindow) updateFocused(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	i := pw.focusIndex
	if i < 0 || i >= len(pw.inputs) {
		return cmd
	}

	pw.inputs[i], cmd = pw.inputs[i].Update(msg)

	return cmd
}

func (pw *lnOpenChannelWindow) cycleFocused(msg tea.KeyMsg) []tea.Cmd {
	// Cycle indexes
	s := msg.String()
	fi := pw.focusIndex
	if s == "up" || s == "shift+tab" {
		fi--
	} else {
		fi++
	}

	if fi > len(pw.inputs) {
		fi = 0
	} else if fi < 0 {
		fi = len(pw.inputs)
	}

	return pw.setFocus(fi)
}

func (pw *lnOpenChannelWindow) openChannel() {
	amount := pw.inputs[0].Value()
	pubKey := pw.inputs[1].Value()
	addr := pw.inputs[2].Value()
	as := pw.as
	pw.requesting = true
	go func() {
		err := as.openChannel(amount, pubKey, addr)
		as.sendMsg(lnOpenChanResult{err: err})
	}()
}

func (pw lnOpenChannelWindow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(pw.as, msg); ss != nil {
		return ss, cmd
	}

	// Return to main window on ESC.
	if isEscMsg(msg) {
		return newMainWindowState(pw.as)
	}

	// Handle generic messages.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // resize window
		pw.as.winW = msg.Width
		pw.as.winH = msg.Height
		return pw, nil

	case lnOpenChanResult:
		pw.openChanErr = msg.err
		pw.requesting = false
		if msg.err == nil {
			return newMainWindowState(pw.as)
		}
		return pw, nil
	}

	// Handle messages when inputting form data.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "enter", "up", "down":
			// Did the user press enter while the submit button was
			// focused?  If so, exit.
			if msg.String() == "enter" && pw.focusIndex == len(pw.inputs) {
				pw.openChannel()
				return pw, nil
			}

			cmds := pw.cycleFocused(msg)
			return pw, batchCmds(cmds)
		}
	}

	cmd := pw.updateFocused(msg)
	return pw, cmd
}

func (pw lnOpenChannelWindow) headerView() string {
	msg := " Open Outbound LN Channel for Sending Payments"
	headerMsg := pw.as.styles.header.Render(msg)
	spaces := pw.as.styles.header.Render(strings.Repeat(" ",
		max(0, pw.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (pw lnOpenChannelWindow) footerView() string {
	footerMsg := fmt.Sprintf(
		" [%s] ",
		time.Now().Format("15:04"),
	)
	fs := pw.as.styles.footer
	spaces := fs.Render(strings.Repeat(" ",
		max(0, pw.as.winW-lipgloss.Width(footerMsg))))
	return fs.Render(footerMsg + spaces)
}

func (pw lnOpenChannelWindow) View() string {
	var b strings.Builder

	b.WriteString(pw.headerView())
	b.WriteString("\n\n")
	b.WriteString("To send payments through the Lightning Network, your LN wallet\n")
	b.WriteString("needs to open a channel to some other, existing LN node.\n")
	b.WriteString("\n")
	b.WriteString("The channel locks your funds with the other node and you'll\n")
	b.WriteString("need to cooperate to redeem them back.\n")
	b.WriteString("\n")
	b.WriteString("Enter the following information to add send capacity.\n")
	nbLines := 10

	if pw.isSetup {
		b.WriteString("A well-known hub is suggested to get your wallet started.\n")
		b.WriteString("Note that you are _NOT_ required to use this specific hub.\n")
		nbLines += 2
	}
	b.WriteString("\n")
	nbLines += 1

	balance, _, _ := pw.as.channelBalance()
	b.WriteString(fmt.Sprintf("Total wallet balance: %s\n\n", balance))
	nbLines += 2

	if pw.requesting {
		b.WriteString("Attempting to initiate channel opening...")
	} else {
		for i := range pw.inputs {
			b.WriteString(pw.inputs[i].View())
			b.WriteString("\n")
		}
		nbLines += len(pw.inputs)

		btnStyle := pw.as.styles.blurred
		if pw.focusIndex == len(pw.inputs) {
			btnStyle = pw.as.styles.focused
		}
		button := btnStyle.Render("[ Submit ]")
		fmt.Fprintf(&b, "\n%s\n\n", button)
		nbLines += 3

		if pw.openChanErr != nil {
			b.WriteString(pw.as.styles.err.Render(pw.openChanErr.Error()))
			b.WriteString("\n")
			nbLines += 1
		}
	}

	for i := 0; i < pw.as.winH-nbLines-1; i++ {
		b.WriteString("\n")
	}
	b.WriteString(pw.footerView())

	return b.String()
}

func newLNOpenChannelWindow(as *appState, isSetup bool) (lnOpenChannelWindow, tea.Cmd) {
	inputs := make([]textinput.Model, 3)
	var t textinput.Model
	for i := range inputs {
		t = textinput.New()
		t.CursorStyle = as.styles.cursor

		switch i {
		case 0:
			t.Placeholder = "Amount"
			t.Focus()
			t.PromptStyle = as.styles.focused
			t.TextStyle = as.styles.focused
		case 1:
			t.Placeholder = "Public Key"
			t.PromptStyle = as.styles.noStyle
			t.TextStyle = as.styles.noStyle

			if isSetup && as.network == "mainnet" {
				t.SetValue("03bd03386d7b2efe80ae46d6c8cfcfdfcf9c9297a465ac0d48c110d11ae58ed509")
			}
			t.Blur()

		case 2:
			t.Placeholder = "Server:Port"
			t.PromptStyle = as.styles.noStyle
			t.TextStyle = as.styles.noStyle

			if isSetup && as.network == "mainnet" {
				t.SetValue("hub0.bisonrelay.org:9735")

			}
			t.Blur()
		}

		inputs[i] = t
	}
	cmd := inputs[0].SetCursorMode(textinput.CursorBlink)

	return lnOpenChannelWindow{
		as:      as,
		inputs:  inputs,
		isSetup: isSetup,
	}, cmd
}
