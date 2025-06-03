package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/client/clientintf"
)

type startOnboardScreen struct {
	initless
	as   *appState
	form formHelper

	attemptingStart bool
	onboardErr      error
}

func (os startOnboardScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	}

	// Handle messages when inputing form data.
	switch msg := msg.(type) {
	case msgCancelForm:
		// Send to screen to receive funds.
		return newLNFundWalletWindow(os.as)

	case msgSubmitForm:
		// Start onboarding attempt.
		os.attemptingStart = true
		key := os.form.inputs[0].(*textInputHelper).Value()
		return os, cmdAttemptStartOnboard(os.as, key)

	case msgStartOnboardErr:
		os.onboardErr = msg
		os.attemptingStart = false
		return os, nil

	case msgOnboardStateChanged:
		// Initial notification that onboarding started.
		return newOnboardScreen(os.as)

	case tea.KeyMsg:
		os.form, cmd = os.form.Update(msg)
		return os, cmd
	}

	return os, cmd
}

func (os startOnboardScreen) headerView(styles *theme) string {
	msg := "Onboarding Bison Relay Client"
	headerMsg := styles.header.Render(msg)
	spaces := styles.header.Render(strings.Repeat(" ",
		max(0, os.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (os startOnboardScreen) footerView(styles *theme) string {
	footerMsg := fmt.Sprintf(
		" [%s] ",
		time.Now().Format("15:04"),
	)
	fs := styles.footer
	spaces := fs.Render(strings.Repeat(" ",
		max(0, os.as.winW-lipgloss.Width(footerMsg))))
	return fs.Render(footerMsg + spaces)
}

func (os startOnboardScreen) View() string {
	styles := os.as.styles.Load()

	var b strings.Builder
	b.WriteString(os.headerView(styles))
	b.WriteString("\n\n")
	b.WriteString("Automatic onboarding is supported by having an existing\n")
	b.WriteString("BR user send you a Paid Invite Key with funds for setting\n")
	b.WriteString("up the required LN channels.\n")
	b.WriteString("\n")
	nbLines := 7

	if os.attemptingStart {
		b.WriteString("Attempting to start onboarding...\n")
		nbLines += 1
	} else {
		b.WriteString(os.form.View())
		nbLines += os.form.lineCount()
	}

	if os.onboardErr != nil {
		b.WriteString("\n")
		b.WriteString(styles.err.Render(os.onboardErr.Error()))
		b.WriteString("\n")
		nbLines += 2
	}

	b.WriteString(blankLines(os.as.winH - nbLines - 1))
	b.WriteString(os.footerView(styles))

	return b.String()
}

func cmdAttemptStartOnboard(as *appState, key string) func() tea.Msg {
	return func() tea.Msg {
		if key == "" {
			return msgStartOnboardErr(fmt.Errorf("key is empty"))
		}

		pik, err := clientintf.DecodePaidInviteKey(key)
		if err != nil {
			return msgStartOnboardErr(fmt.Errorf("invalid key: %v", err))
		}

		err = as.c.StartOnboarding(pik)
		if err != nil {
			return msgStartOnboardErr(fmt.Errorf("error when attempting to start onboard: %v", err))
		}

		// Force wallet to recheck server conn to enable it.
		go as.forceRecheckWalletConn()

		return nil
	}
}

func newStartOnboardScreen(as *appState) (startOnboardScreen, tea.Cmd) {
	styles := as.styles.Load()

	form := newFormHelper(styles,
		newTextInputHelper(styles,
			tihWithPrompt("Key: "),
		),
		newButtonHelper(styles,
			btnWithLabel(" [ Start Onboarding ]"),
			btnWithTrailing(" "),
			btnWithFixedMsgAction(msgSubmitForm{}),
		),
		newButtonHelper(styles,
			btnWithLabel(" [ Skip Onboarding ]"),
			btnWithTrailing("\n"),
			btnWithFixedMsgAction(msgCancelForm{}),
		),
	)

	os := startOnboardScreen{
		as:   as,
		form: form,
	}
	return os, nil
}
