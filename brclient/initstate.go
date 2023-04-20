package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// initStepState tracks what needs to be initialized before the app can
// properly start.
type initStepState struct {
	as *appState

	msgConfCert *msgConfirmServerCert
	viewport    viewport.Model
	focusIdx    int
}

func (ins initStepState) Init() tea.Cmd {
	return nil
}

func (ins *initStepState) updateLogLines() {
	if ins.as.winW > 0 && ins.as.winH > 0 {
		ins.viewport.YPosition = 4
		ins.viewport.Width = ins.as.winW
		ins.viewport.Height = ins.as.winH - 5
	}

	nbLines := ins.as.winH - 5
	ins.viewport.SetContent(ins.as.lastLogLines(nbLines))
	ins.viewport.GotoBottom()
}

func (ins initStepState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if err := isQuitMsg(msg); err != nil {
		return ins, tea.Quit
	}

	if ins.msgConfCert != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case msg.Type == tea.KeyLeft, msg.Type == tea.KeyRight:
				ins.focusIdx = (ins.focusIdx + 1) % 2
				return ins, nil
			case msg.Type == tea.KeyEnter:
				replyChan := ins.msgConfCert.replyChan
				ins.msgConfCert = nil
				var reply error
				if ins.focusIdx == 1 {
					reply = fmt.Errorf("user rejected server certificates")
				}
				go func() { replyChan <- reply }()
				ins.msgConfCert = nil
				return ins, nil
			}
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // resize window
		if ins.as.winW == 0 {
			// Found the window size. We can start the services
			// now.
			cmds = appendCmd(cmds, ins.as.runAsCmd)
		}

		ins.as.winW = msg.Width
		ins.as.winH = msg.Height

		ins.updateLogLines()

	case getClientID:
		// Client lib sent a msg requesting local client
		// nick/name.
		return newInitialUIDState(ins.as)

	case connState:
		if msg != connStateOffline {
			ins.as.diagMsg("Local client ID: %s", ins.as.c.PublicID())

			// Initial connection to server!
			//
			// Check if there's an ongoing onboarding.
			ostate, _ := ins.as.onboardingState()
			if ostate != nil {
				return newOnboardScreen(ins.as)
			}

			// Skip fund and channel stages in a restored wallet
			// to allow a chance for using an SCB.
			isRestore := ins.as.isRestore
			needsFunds, needsSendChan := ins.as.setupNeedsFlags()

			if !isRestore && len(ins.as.c.AddressBook()) == 0 {
				// Client has no addressbook entries,
				// therefore this is likely a new, empty
				// client. Send to the onboarding screen.
				return newStartOnboardScreen(ins.as)
			}
			if !isRestore && needsFunds {
				// Client has entries, so it's likely just a
				// wallet that emptied its funds. Send to the
				// request fund screen.
				return newLNFundWalletWindow(ins.as)
			}
			if !isRestore && needsSendChan {
				return newLNOpenChannelWindow(ins.as, false)
			}

			return newMainWindowState(ins.as)
		}

	case msgConfirmServerCert:
		ins.msgConfCert = &msg
		return ins, nil

	case logUpdated:
		ins.updateLogLines()

	default:
		ins.viewport, cmd = ins.viewport.Update(ins)
		cmds = appendCmd(cmds, cmd)
	}

	return ins, batchCmds(cmds)
}

func (ins initStepState) headerView() string {
	msg := " Initializing Client"
	headerMsg := ins.as.styles.header.Render(msg)
	spaces := ins.as.styles.header.Render(strings.Repeat(" ",
		max(0, ins.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (ins initStepState) footerView() string {
	footerMsg := fmt.Sprintf(
		" [%s] ",
		time.Now().Format("15:04"),
	)
	fs := ins.as.styles.footer
	spaces := fs.Render(strings.Repeat(" ",
		max(0, ins.as.winW-lipgloss.Width(footerMsg))))
	return fs.Render(footerMsg + spaces)
}

func (ins initStepState) View() string {
	var msg, content string
	switch {
	case ins.as.winW == 0:
		msg = "Initializing client..."
		content = ins.viewport.View()

	case ins.msgConfCert != nil:
		msg = "Confirm Server Certificates"
		conf := ins.msgConfCert
		var b strings.Builder
		wln := func(format string, args ...interface{}) {
			b.WriteString(fmt.Sprintf(format, args...))
			b.WriteString("\n")
		}
		wln("Outer Certificate: %s", fingerprintDER(conf.cs.PeerCertificates))
		wln("Inner Certificate: %s", conf.svrID.Identity)
		wln("")
		wln("Accept Certificates and continue connecting to server?")
		wln("")
		yesBtn := "[Yes]"
		noBtn := "[No]"
		switch ins.focusIdx {
		case 0:
			yesBtn = ins.as.styles.focused.Render(yesBtn)
		case 1:
			noBtn = ins.as.styles.focused.Render(noBtn)
		}
		wln("%s %s", yesBtn, noBtn)
		wln(strings.Repeat("\n", ins.as.winH-13))
		content = b.String()

	default:
		msg = "Waiting initial server connection..."
		content = ins.viewport.View()
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s",
		ins.headerView(),
		ins.as.styles.focused.Render(msg),
		content,
		ins.footerView(),
	)
}

func newInitStepState(as *appState, msgConfCert *msgConfirmServerCert) initStepState {
	ins := initStepState{as: as, msgConfCert: msgConfCert}
	ins.updateLogLines()
	return ins
}
