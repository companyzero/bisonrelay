package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type lnOpenChannelWindow struct {
	initless
	as *appState

	form        formHelper
	requesting  bool
	openChanErr error
	isManual    bool
}

const (
	hub0Server = "hub0.bisonrelay.org:9735"
	hub0PubKey = "03bd03386d7b2efe80ae46d6c8cfcfdfcf9c9297a465ac0d48c110d11ae58ed509"
)

func (pw *lnOpenChannelWindow) openChannel() {
	var addr, pubKey string
	if pw.isManual {
		addr = normalizeAddress(pw.form.inputs[1].(*textInputHelper).Value(), defaultLNPort)
		pubKey = pw.form.inputs[2].(*textInputHelper).Value()
	} else {
		addr = hub0Server
		pubKey = hub0PubKey
	}
	amount := pw.form.inputs[0].(*textInputHelper).Value()

	as := pw.as
	pw.requesting = true
	go func() {
		err := as.openChannel(amount, pubKey, addr)
		as.sendMsg(lnOpenChanResult{err: err})
	}()
}

func (pw lnOpenChannelWindow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(pw.as, msg); ss != nil {
		return ss, cmd
	}

	// Return to main window on ESC.
	if isEscMsg(msg) {
		if pw.isManual {
			return newLNOpenChannelWindow(pw.as, false)
		}
		return newMainWindowState(pw.as)
	}

	// Handle generic messages.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyF2 {
			return newLNOpenChannelWindow(pw.as, true)
		}
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

	// Handle messages when inputing form data.
	switch msg := msg.(type) {
	case msgSubmitForm:
		pw.openChannel()

	case tea.KeyMsg:
		pw.form, cmd = pw.form.Update(msg)
		return pw, cmd
	}

	return pw, cmd
}

func (pw lnOpenChannelWindow) headerView() string {
	msg := " Open Outbound LN Channel for Sending Payments"
	if !pw.isManual {
		msg += " - Press F2 for manual entry"
	}
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

	b.WriteString("\n")
	nbLines += 1

	balance, _, _ := pw.as.channelBalance()
	b.WriteString(fmt.Sprintf("Total wallet balance: %s\n\n", balance))
	nbLines += 2

	if pw.requesting {
		b.WriteString("Attempting to initiate channel opening...")
	} else {
		b.WriteString(pw.form.View())
		nbLines += pw.form.lineCount()

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

func newLNOpenChannelWindow(as *appState, isManual bool) (lnOpenChannelWindow, tea.Cmd) {
	form := newFormHelper(as.styles,
		newTextInputHelper(as.styles,
			tihWithPrompt("Amount: "),
		),
	)
	if isManual {
		form.AddInputs(
			newTextInputHelper(as.styles,
				tihWithPrompt("Server:Port: "),
			),
			newTextInputHelper(as.styles,
				tihWithPrompt("Server PubKey: "),
			),
		)
	}
	form.AddInputs(
		newButtonHelper(as.styles,
			btnWithLabel(" [ Add Outbound Capacity ]"),
			btnWithTrailing("\n"),
			btnWithFixedMsgAction(msgSubmitForm{}),
		),
	)

	cmds := form.setFocus(0)
	return lnOpenChannelWindow{
		as:       as,
		form:     form,
		isManual: isManual,
	}, batchCmds(cmds)
}
