package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/term/ansi"
	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
)

type onboardScreen struct {
	initless
	as       *appState
	ostate   *clientintf.OnboardState
	viewport viewport.Model
	oerr     error
	btns     formHelper
}

func (os *onboardScreen) updateLogLines() {
	vph := 10
	if os.as.winW > 0 && os.as.winH > 0 {
		os.viewport.Width = os.as.winW
		os.viewport.Height = vph
	}

	nbLines := os.as.winH - vph
	os.viewport.SetContent(os.as.lastLogLines(nbLines))
	os.viewport.GotoBottom()
}

func (os *onboardScreen) updateButtons() {
	styles := os.as.styles.Load()

	btns := newFormHelper(styles)
	if os.oerr != nil {
		btns.AddInputs(newButtonHelper(
			styles,
			btnWithLabel("[ Retry ]"),
			btnWithTrailing(" "),
			btnWithFixedMsgAction(msgRetryAction{}),
		))
	}
	if os.ostate.Stage == clientintf.StageOpeningInbound && os.oerr != nil {
		btns.AddInputs(newButtonHelper(
			styles,
			btnWithLabel("[ Skip Inbound Channel ]"),
			btnWithTrailing(" "),
			btnWithFixedMsgAction(msgSkipAction{}),
		))
	}
	btns.AddInputs(newButtonHelper(
		styles,
		btnWithLabel("[ Cancel Onboarding ]"),
		btnWithFixedMsgAction(msgCancelForm{}),
	))
	os.btns = btns
	btns.setFocus(0)
}

func (os onboardScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(os.as, msg); ss != nil {
		return ss, cmd
	}

	// Handle generic messages.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // resize window
		os.as.winW = msg.Width
		os.as.winH = msg.Height
		return os, nil

	case tea.KeyMsg:
		os.btns, cmd = os.btns.Update(msg)
		return os, cmd

	case msgCancelForm:
		err := os.as.c.CancelOnboarding()
		if err != nil {
			os.as.diagMsg("Error cancelling onboarding: %v", err)
		}
		return newMainWindowState(os.as)

	case msgRetryAction:
		os.oerr = nil
		return os, cmdRetryOnboarding(os.as.c)

	case msgSkipAction:
		os.oerr = nil
		return os, cmdSkipOnboardingStage(os.as.c)

	case msgOnboardStateChanged:
		ostate, oerr := os.as.onboardingState()
		os.oerr = oerr
		if ostate != nil {
			os.ostate = ostate
		}
		if ostate.Stage == clientintf.StageOnboardDone {
			return newMainWindowState(os.as)
		}
		os.updateButtons()

	case msgStartOnboardErr:
		os.oerr = msg
		os.updateButtons()

	case logUpdated:
		os.updateLogLines()

	}

	return os, nil
}

func (os onboardScreen) headerView(styles *theme) string {
	msg := "Onboarding Bison Relay Client"
	headerMsg := styles.header.Render(msg)
	spaces := styles.header.Render(strings.Repeat(" ",
		max(0, os.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (os onboardScreen) footerView(styles *theme) string {
	footerMsg := fmt.Sprintf(
		" [%s] ",
		time.Now().Format("15:04"),
	)
	fs := styles.footer
	spaces := fs.Render(strings.Repeat(" ",
		max(0, os.as.winW-lipgloss.Width(footerMsg))))
	return fs.Render(footerMsg + spaces)
}

func (os onboardScreen) View() string {
	styles := os.as.styles.Load()

	var b strings.Builder
	b.WriteString(os.headerView(styles))
	nbLines := 1

	ostate := os.ostate
	var niceStage, encKey string
	if ostate != nil {
		switch ostate.Stage {
		case clientintf.StageFetchingInvite:
			niceStage = "Fetching invite from server"
		case clientintf.StageInviteUnpaid:
			niceStage = "Invite was not paid on server"
		case clientintf.StageInviteFetchTimeout:
			niceStage = "Timeout waiting for server to send invite"
		case clientintf.StageInviteNoFunds:
			niceStage = "Invite has no funds (onboarding cannot proceed)"
		case clientintf.StageRedeemingFunds:
			niceStage = "Redeeming on-chain invite funds"
		case clientintf.StageWaitingFundsConfirm:
			niceStage = "Waiting on-chain funds to confirm"
		case clientintf.StageOpeningOutbound:
			niceStage = "Opening outbound LN channel"
		case clientintf.StageWaitingOutMined:
			niceStage = "Waiting outbound LN channel to be mined"
		case clientintf.StageWaitingOutConfirm:
			niceStage = "Waiting confirmation of outbound LN channel"
		case clientintf.StageOpeningInbound:
			niceStage = "Opening inbound LN channel"
		case clientintf.StageInitialKX:
			niceStage = "Performing initial KX with remote client"
		case clientintf.StageOnboardDone:
			niceStage = "Onboarding done"
		default:
			niceStage = fmt.Sprintf("[Unknown %q]", os.ostate.Stage)
		}
		encKey, _ = ostate.Key.Encode()
	}

	pf := func(f string, args ...interface{}) {
		b.WriteString(fmt.Sprintf(f, args...))
	}

	wallet, recv, send := os.as.channelBalance()

	pf("Performing onboarding procedure.\n")
	pf("Current stage: %s\n", styles.nick.Render(niceStage))
	pf("Balances - Wallet: %s, Send: %s, Recv: %s\n", wallet, send, recv)
	pf("Original Key: %s\n", encKey)
	nbLines += 4
	if ostate.Invite != nil {
		pf("Initial RV Point: %s\n", ostate.Invite.InitialRendezvous)
		nbLines += 1
		if ostate.Invite.Funds != nil {
			pf("Invite Funds UTXO: %s:%d\n", ostate.Invite.Funds.Tx,
				ostate.Invite.Funds.Index)
			nbLines += 1
		}
	}
	if ostate.RedeemTx != nil {
		pf("On-Chain redemption tx: %s\n", ostate.RedeemTx)
		pf("On-Chain redemption amount: %s\n", ostate.RedeemAmount)
	} else {
		pf("\n\n")
	}
	nbLines += 2
	if ostate.OutChannelID != "" {
		pf("Outbound Channel ID: %s", ostate.OutChannelID)
	}
	pf("\n")
	if ostate.Stage == clientintf.StageWaitingOutConfirm {
		pf("Confirmations left: %d\n", ostate.OutChannelConfsLeft)
		nbLines += 1
	}
	nbLines += 1
	if ostate.InChannelID != "" {
		pf("Inbound Channel ID: %s", ostate.InChannelID)
	}
	pf("\n")
	nbLines += 1

	pf("\n")
	nbLines += 1
	if os.oerr != nil {
		errMsg := ansi.Wordwrap(styles.err.Render(os.oerr.Error()), os.as.winW-1, wordBreakpoints)
		pf(errMsg)
		pf("\n")
		nbLines += countNewLines(errMsg) + 1
	}

	pf("\n\n")
	nbLines += 2
	pf(os.btns.View())
	pf("\n\n")
	nbLines += 2
	pf("Latest Log Lines:\n")
	pf(os.viewport.View())
	nbLines += 10

	b.WriteString(blankLines(os.as.winH - nbLines - 1))
	b.WriteString(os.footerView(styles))

	return b.String()
}

func cmdRetryOnboarding(c *client.Client) func() tea.Msg {
	return func() tea.Msg {
		err := c.RetryOnboarding()
		if err != nil {
			return msgStartOnboardErr(err)
		}
		return nil
	}
}

func cmdSkipOnboardingStage(c *client.Client) func() tea.Msg {
	return func() tea.Msg {
		err := c.SkipOnboardingStage()
		if err != nil {
			return msgStartOnboardErr(err)
		}
		return nil
	}
}

func newOnboardScreen(as *appState) (onboardScreen, tea.Cmd) {
	ostate, oerr := as.onboardingState()
	os := onboardScreen{
		as:     as,
		ostate: ostate,
		oerr:   oerr,
	}
	os.updateLogLines()
	os.updateButtons()
	return os, nil
}
