package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/decred/dcrd/dcrutil/v4"
)

type lnRequestRecvWindow struct {
	initless
	as *appState

	form formHelper

	requestErr error
	confirming bool
	confirmIdx int
	requesting bool
	reqPolicy  lnReqRecvConfirmPayment
}

func (pw *lnRequestRecvWindow) request() {
	server := pw.form.inputs[0].(*textInputHelper).Value()
	cert := []byte(pw.form.inputs[1].(*textInputHelper).Value())
	amount := pw.form.inputs[2].(*textInputHelper).Value()
	key := ""
	as := pw.as
	pw.requestErr = nil
	pw.requesting = true

	// reqestRecv() blocks until the inbound channel is confirmed, so
	// run as a goroutine.
	go func() {
		err := as.requestRecv(amount, server, key, cert)
		if err != nil {
			as.cwHelpMsg("Failed to add receive capacity: %v", err)
		}
		as.sendMsg(lnReqRecvResult{err: err})
	}()
}

func (pw *lnRequestRecvWindow) confirmResponse(accept bool) {
	pw.confirming = false
	go func() { pw.reqPolicy.replyChan <- accept }()
}

func (pw lnRequestRecvWindow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

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

	case lnReqRecvConfirmPayment:
		pw.reqPolicy = msg
		pw.confirmIdx = 0
		pw.confirming = true

	case lnReqRecvResult:
		pw.requestErr = msg.err
		pw.requesting = false
		pw.confirming = false
		if msg.err == nil {
			return newMainWindowState(pw.as)
		}
	}

	// Handle messages when in confirming state.
	if pw.confirming {
		msg, ok := msg.(tea.KeyMsg)
		if !ok {
			return pw, nil
		}

		switch msg.String() {
		case "tab", "right":
			pw.confirmIdx = (pw.confirmIdx + 1) % 2
		case "shift+tab", "left":
			pw.confirmIdx = ((pw.confirmIdx - 1) % 2) & 1
		case "enter":
			pw.confirmResponse(pw.confirmIdx == 0)
		}

		return pw, nil
	}

	// Handle messages when inputing form data.
	switch msg := msg.(type) {
	case msgSubmitForm:
		pw.request()

	case tea.KeyMsg:
		oldServer := pw.form.inputs[0].(*textInputHelper).Value()
		pw.form, cmd = pw.form.Update(msg)
		newServer := pw.form.inputs[0].(*textInputHelper).Value()
		if oldServer != newServer {
			pw.form.inputs[1].(*textInputHelper).SetValue("")
		}

		return pw, cmd
	}

	return pw, nil
}

func (pw lnRequestRecvWindow) headerView() string {
	msg := " Request Lightning Wallet Receive Capacity"
	headerMsg := pw.as.styles.header.Render(msg)
	spaces := pw.as.styles.header.Render(strings.Repeat(" ",
		max(0, pw.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (pw lnRequestRecvWindow) footerView() string {
	footerMsg := fmt.Sprintf(
		" [%s] ",
		time.Now().Format("15:04"),
	)
	fs := pw.as.styles.footer
	spaces := fs.Render(strings.Repeat(" ",
		max(0, pw.as.winW-lipgloss.Width(footerMsg))))
	return fs.Render(footerMsg + spaces)
}

func (pw lnRequestRecvWindow) View() string {
	var b strings.Builder

	pf := func(f string, args ...interface{}) {
		b.WriteString(fmt.Sprintf(f, args...))
	}
	pf(pw.headerView())
	pf("\n\n")

	nbLines := 2 + 2
	if pw.confirming {
		pf("Confirm payment of %s to receive inbound capacity?\n\n",
			dcrutil.Amount(pw.reqPolicy.estimatedAmount))

		pf("Channel Size: %s\n", dcrutil.Amount(pw.reqPolicy.chanSize))
		pf("Minimum channel lifetime: %s\n", pw.reqPolicy.policy.MinChanLifetime)
		pf("\n")
		pf("Note that the channel may be closed by the liquidity provider\n")
		pf("after the minimum lifetime if not enough payments flow through it.\n")
		pf("\n")

		yesStyle, noStyle := pw.as.styles.focused, pw.as.styles.noStyle
		if pw.confirmIdx == 1 {
			yesStyle, noStyle = noStyle, yesStyle
		}
		pf(yesStyle.Render("[ Yes ]"))
		pf(noStyle.Render(" [ No ]"))
		pf("\n")

		nbLines += 9
	} else if pw.requesting {
		b.WriteString("Requesting liquidity...")
		nbLines += 1
	} else {
		pf("Enter the following information to request recv capacity.\n\n")
		nbLines += 2

		pf(pw.form.View())
		nbLines += pw.form.lineCount()

		pf("\n")
		if pw.requestErr != nil {
			b.WriteString(pw.as.styles.err.Render(pw.requestErr.Error()))
		}
		pf("\n")
		nbLines += 2
	}

	for i := 0; i < pw.as.winH-nbLines; i++ {
		pf("\n")
	}

	pf(pw.footerView())

	return b.String()
}

func newLNRequestRecvWindow(as *appState) (lnRequestRecvWindow, tea.Cmd) {
	var server, cert string
	switch as.network {
	case "mainnet":
		server = "https://lp0.bisonrelay.org:9130"
		cert = `-----BEGIN CERTIFICATE-----
MIIBwjCCAWmgAwIBAgIQA78YKmDt+ffFJmAN5EZmejAKBggqhkjOPQQDAjAyMRMw
EQYDVQQKEwpiaXNvbnJlbGF5MRswGQYDVQQDExJscDAuYmlzb25yZWxheS5vcmcw
HhcNMjIwOTE4MTMzNjA4WhcNMzIwOTE2MTMzNjA4WjAyMRMwEQYDVQQKEwpiaXNv
bnJlbGF5MRswGQYDVQQDExJscDAuYmlzb25yZWxheS5vcmcwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAASF1StlsfdDUaCXMiZvDBhhMZMdvAUoD6wBdS0tMBN+9y91
UwCBu4klh+VmpN1kCzcR6HJHSx5Cctxn7Smw/w+6o2EwXzAOBgNVHQ8BAf8EBAMC
AoQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUqqlcDx8e+XgXXU9cXAGQEhS8
59kwHQYDVR0RBBYwFIISbHAwLmJpc29ucmVsYXkub3JnMAoGCCqGSM49BAMCA0cA
MEQCIGtLFLIVMnU2EloN+gI+uuGqqqeBIDSNhP9+bznnZL/JAiABsLKKtaTllCSM
cNPr8Y+sSs2MHf6xMNBQzV4KuIlPIg==
-----END CERTIFICATE-----`
	case "simnet":
		server = "https://127.0.0.1:29130"
	}

	certModel := newTextAreaModel(as.styles)
	certModel.SetHeight(5)

	form := newFormHelper(as.styles,
		newTextInputHelper(as.styles,
			tihWithPrompt("Server:Port: "),
			tihWithValue(server),
		),
		newTextInputHelper(as.styles,
			tihWithPrompt("Certificate: "),
			tihWithValue(cert),
		),
		newTextInputHelper(as.styles,
			tihWithPrompt("Amount: "),
		),
		newButtonHelper(as.styles,
			btnWithLabel(" [ Request Inbound Capacity ]"),
			btnWithTrailing("\n"),
			btnWithFixedMsgAction(msgSubmitForm{}),
		),
	)

	cmds := form.setFocus(0)
	return lnRequestRecvWindow{
		as:   as,
		form: form,
	}, batchCmds(cmds)
}
