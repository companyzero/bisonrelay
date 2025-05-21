package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/term/ansi"
	"github.com/companyzero/bisonrelay/brclient/internal/sloglinesbuffer"
	"github.com/companyzero/bisonrelay/embeddeddcrlnd"
	"github.com/decred/dcrlnd/lnrpc/initchainsyncrpc"
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

	viewWidth int
	viewport  viewport.Model

	txtPass    textinput.Model
	unlocking  bool
	unlockErr  string
	crashStack []byte

	compactingDb   bool
	migratingDb    bool
	compactErrored bool
}

func (ulns unlockLNScreen) Init() tea.Cmd {
	var cmds []tea.Cmd
	if ulns.lndc == nil {
		cfg := embeddeddcrlnd.Config{
			RootDir:           defaultLNWalletDir(ulns.cfg.Root),
			Network:           ulns.cfg.Network,
			DebugLevel:        ulns.cfg.LNDebugLevel,
			MaxLogFiles:       ulns.cfg.LNMaxLogFiles,
			RPCAddresses:      ulns.cfg.LNRPCListen,
			DialFunc:          ulns.cfg.dialFunc,
			TorAddr:           ulns.cfg.ProxyAddr,
			TorIsolation:      ulns.cfg.TorIsolation,
			SyncFreeList:      ulns.cfg.SyncFreeList,
			AutoCompact:       ulns.cfg.AutoCompact,
			AutoCompactMinAge: ulns.cfg.AutoCompactMinAge,
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

// cmdCheckWalletUnlocked checks if the wallet is already unlocked
func cmdCheckWalletUnlocked(lndc *embeddeddcrlnd.Dcrlnd) tea.Msg {
	// Try to use a blank password to see if we get "wallet already unlocked" error
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err := lndc.TryUnlock(ctx, "")
	// If we get nil error or "wallet already unlocked" error, wallet is unlocked
	if err == nil || strings.Contains(err.Error(), "wallet already unlocked") {
		return checkWalletUnlockedResult{isUnlocked: true}
	}

	return checkWalletUnlockedResult{isUnlocked: false, err: err}
}

// cmdShowLoadingTick returns a tick message after a delay
func cmdShowLoadingTick() tea.Msg {
	time.Sleep(500 * time.Millisecond)
	return showLoadingTick{}
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
		ulns.viewWidth = msg.Width - 1
		ulns.viewport.Width = msg.Width - 1
		ulns.viewport.Height = msg.Height - ulns.viewport.YPosition - 3
		return ulns, nil

	case runDcrlndErrMsg:
		ulns.err = msg.error
		ulns.connCancel()
		return ulns, tea.Quit

	case *embeddeddcrlnd.Dcrlnd:
		ulns.lndc = msg
		// Check if wallet is already unlocked (probably via pass.txt file)
		return ulns, func() tea.Msg { return cmdCheckWalletUnlocked(msg) }

	case checkWalletUnlockedResult:
		if msg.isUnlocked {
			// Wallet is already unlocked, show a loading state while RPC starts up
			ulns.needsUnlock = false
			ulns.unlocking = true // Use unlocking flag to show loading state

			// Start a tick to show loading animation
			return ulns, cmdShowLoadingTick
		}
		// Wallet needs to be unlocked, focus the password input
		cmd := ulns.txtPass.Focus()
		return ulns, cmd

	case showLoadingTick:
		// Continue showing loading state and pulsing
		if !ulns.needsUnlock && ulns.unlocking {
			// Try to start the runNotifier
			ulns.runNotifier()
			// Keep ticking to show loading animation
			return ulns, cmdShowLoadingTick
		}
		return ulns, nil

	case unlockDcrlndResult:
		ulns.unlocking = false
		if msg.err != nil {
			// Check if the error is because wallet is already unlocked
			if strings.Contains(msg.err.Error(), "wallet already unlocked") {
				ulns.needsUnlock = false
				ulns.connCancel()
				return ulns, tea.Quit
			}
			ulns.unlockErr = msg.err.Error()
			return ulns, nil
		}
		ulns.needsUnlock = false
		ulns.runNotifier()
		return ulns, nil

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
		logLines := ulns.lndLogLines.LastLogLines(20)
		ulns.compactingDb = stringsContains(logLines, "Compacting database file at") && !stringsContains(logLines, "Database(s) now open")
		ulns.migratingDb = stringsContains(logLines, "Performing database schema migration") && !stringsContains(logLines, "Database(s) now open")
		ulns.compactErrored = stringsContains(logLines, "error during compact")
		logTxt := ansi.Wordwrap(strings.Join(logLines, ""), ulns.viewWidth-1, wordBreakpoints)
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
	var b strings.Builder
	pf := func(s string, args ...interface{}) {
		b.WriteString(fmt.Sprintf(s, args...))
	}

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	migrateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)

	var lines int
	switch {
	case ulns.lndc == nil:
		pf(titleStyle.Render("Initializing internal dcrlnd instance"))
		pf("\n\n")
		lines += 2

		// Additional info.
		extraLines := true
		switch {
		case ulns.compactErrored:
			pf(ulns.styles.err.Render("Compaction error. Look at the log to see actual error"))
		case ulns.migratingDb:
			pf(migrateStyle.Render("Performing DB upgrade. This might take a while."))
		case ulns.compactingDb:
			pf(migrateStyle.Render("Compacting DB. This might take a while."))
		default:
			extraLines = false
		}
		if extraLines {
			pf("\n\n")
			lines += 2
		}

	case ulns.unlocking:
		if ulns.needsUnlock {
			pf(titleStyle.Render("Unlocking internal wallet"))
		} else {
			// This is the pass.txt auto-unlock case
			pf(loadingStyle.Render("Waiting for RPC server to be ready"))
			pf("\n\n")
			pf("Wallet automatically unlocked by password file")
			lines += 3
		}
		pf("\n\n")
		lines += 2

	case ulns.needsUnlock:
		pf(titleStyle.Render("Unlock internal dcrlnd wallet"))
		pf("\n\n")
		pf(ulns.txtPass.View())
		pf("\n\n")
		errMsg := ansi.Wordwrap(ulns.unlockErr, ulns.viewWidth, wordBreakpoints)
		pf(ulns.styles.err.Render(errMsg))
		pf("\n")
		lines += 2 + 2 + 1 + countNewLines(errMsg)

	case ulns.updt == nil:
		pf(titleStyle.Render("Initial Sync"))
		pf("\n\n")
		lines += 2

	default:
		ts := time.Unix(ulns.updt.BlockTimestamp, 0).Format("2006-01-02 15:04:05")
		pf(titleStyle.Render("Initial Sync"))
		pf("\n\n")
		pf("Block Hash: %x\n", ulns.updt.BlockHash)
		pf("Block Height: %d\n", ulns.updt.BlockHeight)
		pf("Block Timestamp: %s\n\n", ts)
		lines += 6
	}

	for i := lines; i < ulns.viewport.YPosition; i++ {
		pf("\n")
	}
	// ulns.viewport.YPosition = lines + 1

	pf("Latest Log Lines:\n\n")
	pf(ulns.viewport.View())
	pf("\n")
	return b.String()
}

func newUnlockLNScreen(cfg *config, lndc *embeddeddcrlnd.Dcrlnd,
	msgSender func(tea.Msg), lndLogLines *sloglinesbuffer.Buffer) unlockLNScreen {

	theme, err := newTheme(nil)
	if err != nil {
		panic(err)
	}

	c := cursor.New()
	c.Style = theme.cursor
	c.SetMode(cursor.CursorBlink)

	txtPass := textinput.New()
	txtPass.Placeholder = ""
	txtPass.Prompt = "Wallet Passphrase: "
	txtPass.EchoCharacter = '*'
	txtPass.EchoMode = textinput.EchoPassword
	txtPass.PromptStyle = theme.focused
	txtPass.TextStyle = theme.focused
	txtPass.Width = 100
	txtPass.Cursor = c

	viewport := viewport.Model{
		YPosition: 9,
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
