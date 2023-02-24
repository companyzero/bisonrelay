package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrlnd/lnrpc"
)

type lnFundWalletWindow struct {
	initless
	as *appState

	unconfFunds dcrutil.Amount

	address string
}

func (ws *lnFundWalletWindow) resultModel() (tea.Model, tea.Cmd) {
	_, needsSendChan := ws.as.setupNeedsFlags()
	if needsSendChan {
		return newLNOpenChannelWindow(ws.as, false)
	}

	return newMainWindowState(ws.as)
}

func (ws lnFundWalletWindow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
	)

	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(ws.as, msg); ss != nil {
		return ss, cmd
	}

	// Main handler for the lnFundWalletWindow. Only returns early if we're
	// switching the state, otherwise returns the updated state at the end
	// of the function.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // resize window
		ws.as.winW = msg.Width
		ws.as.winH = msg.Height

	case *lnrpc.NewAddressResponse:
		ws.address = msg.Address
		return ws, nil

	case tea.KeyMsg:
		switch msg.String() {

		case "tab", "shift+tab", "enter", "up", "down":
			// Set focus to next input
			s := msg.String()

			// Did the user press enter while the submit button was focused?
			// If so, exit.
			if s == "enter" {
				return ws.resultModel()
			}
		default:
		}

	case msgUnconfirmedFunds:
		ws.unconfFunds = dcrutil.Amount(msg)

	case msgConfirmedFunds:
		return ws.resultModel()

	default:
	}

	return ws, batchCmds(cmds)
}

func (ws lnFundWalletWindow) headerView() string {
	msg := "Fund the LN wallet"
	headerMsg := ws.as.styles.header.Render(msg)
	spaces := ws.as.styles.header.Render(strings.Repeat(" ",
		max(0, ws.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (ws lnFundWalletWindow) footerView() string {
	footerMsg := fmt.Sprintf(
		" [%s] ",
		time.Now().Format("15:04"),
	)
	fs := ws.as.styles.footer
	spaces := fs.Render(strings.Repeat(" ",
		max(0, ws.as.winW-lipgloss.Width(footerMsg))))
	return fs.Render(footerMsg + spaces)
}

func (ws lnFundWalletWindow) View() string {
	var b strings.Builder

	b.WriteString(ws.headerView())
	b.WriteString("\n\n")
	b.WriteString("The underlying wallet has no on-chain funds.\n")
	b.WriteString("Please send funds to the following on-chain address:\n")
	b.WriteString("\n")
	b.WriteString(ws.address)
	b.WriteString("\n\n")
	b.WriteString("This is needed so that you can open channels to pay for\n")
	b.WriteString("sending and receiving messages\n")

	nbLines := 10

	if ws.unconfFunds > 0 {
		b.WriteString(fmt.Sprintf("\nWaiting for %s to confirm...\n", ws.unconfFunds))
	} else {
		b.WriteString("\n\n")
	}
	nbLines += 2

	btnStyle := ws.as.styles.focused
	button := btnStyle.Render("[ Skip ]")
	fmt.Fprintf(&b, "\n\n%s\n\n", button)
	nbLines += 4

	b.WriteString(blankLines(ws.as.winH - nbLines - 1))
	b.WriteString(ws.footerView())

	return b.String()
}

func newLNFundWalletWindow(as *appState) (lnFundWalletWindow, tea.Cmd) {
	cmd := func() tea.Msg {
		if as.lnRPC == nil {
			return fmt.Errorf("LN client not configured")
		}
		na, err := as.lnRPC.NewAddress(as.ctx,
			&lnrpc.NewAddressRequest{Type: lnrpc.AddressType_PUBKEY_HASH})
		if err != nil {
			return err
		}
		return na
	}
	return lnFundWalletWindow{as: as}, cmd
}
