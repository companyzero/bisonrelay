package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/companyzero/bisonrelay/brclient/internal/sloglinesbuffer"
	"github.com/companyzero/bisonrelay/embeddeddcrlnd"
	"github.com/decred/dcrlnd/lnrpc/initchainsyncrpc"
	"github.com/muesli/reflow/wordwrap"
)

type unlockLNScreen struct {
	cfg         *config
	lndc        *embeddeddcrlnd.Dcrlnd
	needsUnlock bool
	err         error
	styles      *theme
	sendMsg     func(tea.Msg)
	updt        *initchainsyncrpc.ChainSyncUpdate
	lndLogLines *sloglinesbuffer.Buffer

	connCtx    context.Context
	connCancel func()

	viewHeight, viewWidth int
	viewport              viewport.Model

	txtPass    textinput.Model
	unlocking  bool
	unlockErr  string
	crashStack []byte
}

func (ulns unlockLNScreen) Init() tea.Cmd {
	var cmds []tea.Cmd
	if ulns.lndc == nil {
		cfg := embeddeddcrlnd.Config{
			RootDir:      defaultLNWalletDir(ulns.cfg.Root),
			Network:      ulns.cfg.Network,
			DebugLevel:   ulns.cfg.LNDebugLevel,
			MaxLogFiles:  ulns.cfg.LNMaxLogFiles,
			RPCAddresses: ulns.cfg.LNRPCListen,
			DialFunc:     ulns.cfg.dialFunc,
		}

		cmd := func() tea.Msg {
			return cmdRunDcrlnd(ulns.connCtx, cfg)
		}
		cmds = appendCmd(cmds, cmd)
	} else if !ulns.needsUnlock {
		ulns.runNotifier()
	}

	return batchCmds(cmds)
}

func (ulns *unlockLNScreen) runNotifier() {
	// Run chain sync notifier to deliver messages to the unlock
	// screen.
	notifier := func(updt *initchainsyncrpc.ChainSyncUpdate, err error) {
		ulns.sendMsg(lnChainSyncUpdate{updt, err})
	}
	ctx := context.Background()
	go ulns.lndc.NotifyInitialChainSync(ctx, notifier)
}

func (ulns unlockLNScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if isCrashMsg(msg) {
		ulns.crashStack = allStack()
		ulns.err = fmt.Errorf("crashing app")
		ulns.connCancel()
		return ulns, tea.Quit
	}
	if err := isQuitMsg(msg); err != nil {
		ulns.err = fmt.Errorf("user canceled unlocking")
		ulns.connCancel()
		return ulns, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ulns.viewHeight = msg.Height - 15
		ulns.viewWidth = msg.Width - 1
		ulns.viewport.Width = msg.Width - 1
		ulns.viewport.YPosition = 15
		ulns.viewport.Height = msg.Height - ulns.viewport.YPosition
		return ulns, nil

	case runDcrlndErrMsg:
		ulns.err = msg.error
		ulns.connCancel()
		return ulns, tea.Quit

	case *embeddeddcrlnd.Dcrlnd:
		ulns.lndc = msg
		cmd := ulns.txtPass.Focus()
		return ulns, cmd

	case unlockDcrlndResult:
		ulns.unlocking = false
		if msg.err != nil {
			ulns.unlockErr = msg.err.Error()
			return ulns, nil
		} else {
			ulns.needsUnlock = false
			ulns.runNotifier()
			return ulns, nil
		}

	case lnChainSyncUpdate:
		if msg.err != nil {
			ulns.err = msg.err
			return ulns, nil
		}
		ulns.updt = msg.update
		if msg.update.Synced {
			ulns.connCancel()
			return ulns, tea.Quit
		}
		return ulns, nil

	case logUpdated:
		logLines := ulns.lndLogLines.LastLogLines(ulns.viewport.Height)
		logTxt := wordwrap.String(strings.Join(logLines, ""), ulns.viewWidth-1)
		ulns.viewport.SetContent(logTxt)
		ulns.viewport.GotoBottom()
		return ulns, nil
	}

	if ulns.lndc != nil && ulns.needsUnlock {
		var cmd tea.Cmd
		ulns.txtPass, cmd = ulns.txtPass.Update(msg)
		if !isEnterMsg(msg) {
			return ulns, cmd
		}

		pass := ulns.txtPass.Value()
		ulns.txtPass.SetValue("")
		cmd = func() tea.Msg { return cmdUnlockDcrlnd(ulns.lndc, pass) }
		ulns.unlocking = true
		return ulns, cmd
	}

	return ulns, nil
}

func (ulns unlockLNScreen) View() string {
	if ulns.lndc == nil {
		return "Initializing internal dcrlnd instance"
	}
	if ulns.unlocking {
		return "Unlocking internal wallet"
	}
	if ulns.needsUnlock {
		return "Unlock internal dcrlnd wallet\n\n" +
			ulns.txtPass.View() + "\n\n" +
			ulns.styles.err.Render(ulns.unlockErr)
	}

	if ulns.updt == nil {
		return "Initial sync"
	}

	var b strings.Builder
	ts := time.Unix(ulns.updt.BlockTimestamp, 0).Format("2006-01-02 15:04:05")
	pf := func(s string, args ...interface{}) {
		b.WriteString(fmt.Sprintf(s, args...))
	}
	pf("Initial Sync\n\n")
	pf("Block Hash: %x\n", ulns.updt.BlockHash)
	pf("Block Height: %d\n", ulns.updt.BlockHeight)
	pf("Block Timestamp: %s\n\n", ts)
	pf("Latest Log Lines:\n\n")
	pf(ulns.viewport.View())
	return b.String()
}

func newUnlockLNScreen(cfg *config, lndc *embeddeddcrlnd.Dcrlnd,
	msgSender func(tea.Msg), lndLogLines *sloglinesbuffer.Buffer) unlockLNScreen {

	theme, err := newTheme(nil)
	if err != nil {
		panic(err)
	}

	txtPass := textinput.New()
	txtPass.Placeholder = ""
	txtPass.Prompt = "Wallet Passphrase: "
	txtPass.EchoCharacter = '*'
	txtPass.EchoMode = textinput.EchoPassword
	txtPass.PromptStyle = theme.focused
	txtPass.TextStyle = theme.focused
	txtPass.Width = 100
	txtPass.SetCursorMode(textinput.CursorBlink)

	viewport := viewport.Model{
		YPosition: 15,
		Height:    10,
	}
	logLines := lndLogLines.LastLogLines(viewport.Height)
	logTxt := strings.Join(logLines, "")
	viewport.SetContent(logTxt)
	viewport.GotoBottom()

	connCtx, connCancel := context.WithCancel(context.Background())

	return unlockLNScreen{
		cfg:         cfg,
		lndc:        lndc,
		needsUnlock: lndc == nil,
		styles:      theme,
		txtPass:     txtPass,
		sendMsg:     msgSender,
		lndLogLines: lndLogLines,
		viewport:    viewport,
		connCtx:     connCtx,
		connCancel:  connCancel,
	}
}

func cmdUnlockDcrlnd(lndc *embeddeddcrlnd.Dcrlnd, pass string) tea.Msg {
	ctx := context.Background()
	err := lndc.TryUnlock(ctx, pass)
	return unlockDcrlndResult{err: err}
}
