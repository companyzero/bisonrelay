package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/companyzero/bisonrelay/embeddeddcrlnd"
	"github.com/erikgeiser/promptkit/selection"
)

type setupWizardScreenStage int

const (
	swsStageNetwork setupWizardScreenStage = iota
	swsStageWalletType
	swsStageNewOrRestore
	swsStageExternalDetails
	swsStageServer
	swsStageWaitingRunForWalletPass
	swsStageWalletPass
	swsStageWaitingRunForWalletRestore
	swsStageWalletRestore
	swsStageCreatingWallet
	swsStageSeed
)

type setupWizardScreen struct {
	lndc        *embeddeddcrlnd.Dcrlnd
	cfgFilePath string

	winW, winH int
	completed  bool
	stage      setupWizardScreenStage
	err        error
	crashStack []byte
	styles     *theme

	selNetwork      *selection.Model
	selWalletType   *selection.Model
	selNewOrRestore *selection.Model

	connCtx    context.Context
	connCancel func()

	focusIndex    int
	inputs        []textinput.Model
	validationErr string

	net            string
	walletType     string
	newOrRestore   string
	lnHost         string
	lnTLSPath      string
	lnMacaroonPath string
	serverAddr     string
	walletPass     string
	seedWords      []string
	seed           []byte
	mcbBytes       []byte
}

func (sws setupWizardScreen) Init() tea.Cmd {
	var cmds []tea.Cmd

	cmds = appendCmd(cmds, sws.selNetwork.Init())
	cmds = appendCmd(cmds, sws.selWalletType.Init())
	cmds = appendCmd(cmds, sws.selNewOrRestore.Init())

	return batchCmds(cmds)
}

func (sws *setupWizardScreen) isRestore() bool {
	return len(sws.seedWords) > 0
}

func (sws *setupWizardScreen) setFocus(i int) []tea.Cmd {
	var cmds []tea.Cmd
	if i >= len(sws.inputs) {
		return nil
	}

	sws.focusIndex = i
	for i := 0; i <= len(sws.inputs)-1; i++ {
		if i == sws.focusIndex {
			// Set focused state
			cmd := sws.inputs[i].Focus()
			cmds = appendCmd(cmds, cmd)
			sws.inputs[i].PromptStyle = sws.styles.focused
			sws.inputs[i].TextStyle = sws.styles.focused
			continue
		}
		// Remove focused state
		sws.inputs[i].Blur()
		sws.inputs[i].PromptStyle = sws.styles.noStyle
		sws.inputs[i].TextStyle = sws.styles.noStyle
	}

	return cmds
}

func (sws *setupWizardScreen) initInputsExternalDetails() tea.Cmd {
	txtLNHost := textinput.New()
	txtLNHost.Placeholder = ""
	txtLNHost.Prompt = "LN Wallet Host: "
	txtLNHost.Width = sws.winW
	txtLNHost.SetValue("127.0.0.1:10009")
	txtLNHost.SetCursorMode(textinput.CursorBlink)

	txtLNTls := textinput.New()
	txtLNTls.Placeholder = ""
	txtLNTls.Prompt = "TLS Cert Path: "
	txtLNTls.Width = sws.winW
	txtLNTls.SetValue("~/.dcrlnd/tls.cert")
	txtLNTls.SetCursorMode(textinput.CursorBlink)

	txtLNMacaroon := textinput.New()
	txtLNMacaroon.Placeholder = ""
	txtLNMacaroon.Prompt = "Macaroon Path: "
	txtLNMacaroon.Width = sws.winW
	txtLNMacaroon.SetCursorMode(textinput.CursorBlink)
	defMacaPath := fmt.Sprintf("~/.dcrlnd/data/chain/decred/%s/admin.macaroon",
		sws.net)
	txtLNMacaroon.SetValue(defMacaPath)

	sws.inputs = []textinput.Model{
		txtLNHost,
		txtLNTls,
		txtLNMacaroon,
	}
	return batchCmds(sws.setFocus(0))
}

func (sws *setupWizardScreen) initInputsServer() tea.Cmd {
	txtServer := textinput.New()
	txtServer.Placeholder = ""
	txtServer.Prompt = "Relay Server Address: "
	txtServer.Width = sws.winW
	txtServer.SetCursorMode(textinput.CursorBlink)

	// Set default server address based on the newtork.
	switch sws.net {
	case "mainnet":
		txtServer.SetValue("br00.bisonrelay.org:12345")
	case "testnet":
		txtServer.SetValue("216.128.136.239:65432")
	case "simnet":
		txtServer.SetValue("127.0.0.1:12345")
	}

	sws.inputs = []textinput.Model{txtServer}

	return batchCmds(sws.setFocus(0))
}

func (sws *setupWizardScreen) initRestoreWallet() tea.Cmd {
	txtRestore := textinput.New()
	txtRestore.Placeholder = ""
	txtRestore.Prompt = "Wallet Seed: "
	txtRestore.Width = sws.winW
	txtRestore.SetCursorMode(textinput.CursorBlink)

	txtRestoreSCB := textinput.New()
	txtRestoreSCB.Placeholder = ""
	txtRestoreSCB.Prompt = "Path to Channel MCB (recommended): "
	txtRestoreSCB.Width = sws.winW
	txtRestoreSCB.SetCursorMode(textinput.CursorBlink)

	sws.inputs = []textinput.Model{txtRestore, txtRestoreSCB}
	return batchCmds(sws.setFocus(0))
}

func (sws *setupWizardScreen) initInputsWalletPass() tea.Cmd {
	txtPass := textinput.New()
	txtPass.Placeholder = ""
	txtPass.Prompt = "Wallet Passphrase: "
	txtPass.Width = sws.winW
	txtPass.EchoCharacter = '*'
	txtPass.EchoMode = textinput.EchoPassword
	txtPass.SetCursorMode(textinput.CursorBlink)

	txtPassDup := textinput.New()
	txtPassDup.Placeholder = ""
	txtPassDup.Prompt = "Repeat Passphrase: "
	txtPassDup.Width = sws.winW
	txtPassDup.EchoCharacter = '*'
	txtPassDup.EchoMode = textinput.EchoPassword
	txtPassDup.SetCursorMode(textinput.CursorBlink)

	sws.inputs = []textinput.Model{txtPass, txtPassDup}
	return batchCmds(sws.setFocus(0))
}

func (sws *setupWizardScreen) initConfirmSeedInputs() tea.Cmd {
	txtOk := textinput.New()
	txtOk.Placeholder = ""
	txtOk.Prompt = "Type OK to proceed: "
	txtOk.Width = sws.winW
	txtOk.SetCursorMode(textinput.CursorBlink)

	sws.inputs = []textinput.Model{txtOk}
	return batchCmds(sws.setFocus(0))
}

func (sws *setupWizardScreen) updateFocused(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	i := sws.focusIndex
	if i < 0 || i >= len(sws.inputs) {
		return cmd
	}

	sws.inputs[i], cmd = sws.inputs[i].Update(msg)

	return cmd
}

func (sws *setupWizardScreen) generateConfig() error {
	cfg := &config{
		WalletType:     sws.walletType,
		Network:        sws.net,
		LNRPCHost:      sws.lnHost,
		LNTLSCertPath:  sws.lnTLSPath,
		LNMacaroonPath: sws.lnMacaroonPath,
		ServerAddr:     sws.serverAddr,
	}

	return saveNewConfig(sws.cfgFilePath, cfg)
}

func (sws setupWizardScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if isCrashMsg(msg) {
		sws.crashStack = allStack()
		sws.err = fmt.Errorf("crashing app")
		sws.connCancel()
		return sws, tea.Quit
	}
	if err := isQuitMsg(msg); err != nil {
		sws.connCancel()
		return sws, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // resize window
		sws.winW = msg.Width
		sws.winH = msg.Height

		return sws, nil

	case runDcrlndErrMsg:
		sws.err = msg.error
		sws.connCancel()
		return sws, tea.Quit

	case *embeddeddcrlnd.Dcrlnd:
		sws.lndc = msg
		switch sws.stage {
		case swsStageWaitingRunForWalletPass:
			sws.stage = swsStageWalletPass
		case swsStageWaitingRunForWalletRestore:
			sws.stage = swsStageWalletRestore
		}
		return sws, nil

	case createWalletResult:
		sws.seed, sws.err = msg.seed, msg.err
		if sws.err != nil {
			sws.connCancel()
			return sws, tea.Quit
		}

		// Skip seed reviewing and go to server stage directly.
		if !sws.isRestore() {
			sws.initConfirmSeedInputs()
			sws.stage = swsStageSeed
		} else {
			sws.initInputsServer()
			sws.stage = swsStageServer
		}
		return sws, nil
	}

	switch sws.stage {
	case swsStageNetwork:
		sws.selNetwork.Update(msg)
		if !isEnterMsg(msg) && sws.selNetwork.Err == nil {
			return sws, nil
		}

		v, err := sws.selNetwork.Value()
		if err != nil {
			sws.err = sws.selNetwork.Err
			sws.connCancel()
			return sws, tea.Quit
		}

		if v == nil {
			sws.err = fmt.Errorf("nil value")
			sws.connCancel()
			return sws, tea.Quit
		}

		sws.net = v.Value.(string)
		sws.stage = swsStageWalletType

	case swsStageWalletType:
		sws.selWalletType.Update(msg)
		if !isEnterMsg(msg) && sws.selWalletType.Err == nil {
			return sws, nil
		}

		v, err := sws.selWalletType.Value()
		if err != nil {
			sws.err = sws.selWalletType.Err
			sws.connCancel()
			return sws, tea.Quit
		}

		if v == nil {
			sws.err = fmt.Errorf("nil value")
			sws.connCancel()
			return sws, tea.Quit
		}

		sws.walletType = v.Value.(string)
		switch sws.walletType {
		case "internal":
			sws.stage = swsStageNewOrRestore
			return sws, nil
		case "external":
			sws.stage = swsStageExternalDetails
			cmd := sws.initInputsExternalDetails()
			return sws, cmd
		default:
			sws.err = fmt.Errorf("unknown wallet type %s", sws.walletType)
			sws.connCancel()
			return sws, tea.Quit
		}

	case swsStageNewOrRestore:
		sws.selNewOrRestore.Update(msg)
		if !isEnterMsg(msg) && sws.selNewOrRestore.Err == nil {
			return sws, nil
		}

		v, err := sws.selNewOrRestore.Value()
		if err != nil {
			sws.err = sws.selNewOrRestore.Err
			sws.connCancel()
			return sws, tea.Quit
		}

		if v == nil {
			sws.err = fmt.Errorf("nil value")
			sws.connCancel()
			return sws, tea.Quit
		}

		sws.newOrRestore = v.Value.(string)
		switch sws.newOrRestore {
		case "new":
			sws.stage = swsStageWaitingRunForWalletPass
			sws.initInputsWalletPass()
		case "restore":
			sws.stage = swsStageWaitingRunForWalletRestore
			sws.initRestoreWallet()
		default:
			sws.err = fmt.Errorf("unknown new or restore selection %s", sws.newOrRestore)
			sws.connCancel()
			return sws, tea.Quit
		}
		rootDir := defaultLNWalletDir(defaultRootDir(sws.cfgFilePath))
		return sws, func() tea.Msg {
			cfg := embeddeddcrlnd.Config{
				RootDir:     rootDir,
				Network:     sws.net,
				DebugLevel:  "info",
				MaxLogFiles: 3,
			}
			return cmdRunDcrlnd(sws.connCtx, cfg)
		}

	case swsStageExternalDetails:
		var cmd tea.Cmd
		cmd = sws.updateFocused(msg)
		if !isEnterMsg(msg) {
			return sws, cmd
		}

		// Validate.
		var cmds []tea.Cmd
		sws.validationErr = ""
		switch sws.focusIndex {
		case 0:
			val := strings.TrimSpace(sws.inputs[0].Value())
			if val == "" {
				sws.validationErr = "Host cannot be empty"
			} else {
				sws.lnHost = val
				cmds = sws.setFocus(sws.focusIndex + 1)
			}
		case 1:
			val := strings.TrimSpace(sws.inputs[1].Value())
			if val == "" {
				sws.validationErr = "TLS cert path cannot be empty"
			} else {
				// TODO: Check if it's a valid TLS file
				sws.lnTLSPath = val
				cmds = sws.setFocus(sws.focusIndex + 1)
			}
		case 2:
			val := strings.TrimSpace(sws.inputs[2].Value())
			if val == "" {
				sws.validationErr = "Macaroon path cannot be empty"
			} else {
				// TODO: Check if it's a valid macaroon file
				sws.lnMacaroonPath = val
				sws.stage = swsStageServer
				cmd = sws.initInputsServer()
				return sws, cmd
			}
		}
		return sws, batchCmds(cmds)

	case swsStageServer:
		cmd := sws.updateFocused(msg)
		if !isEnterMsg(msg) {
			return sws, cmd
		}

		val := strings.TrimSpace(sws.inputs[0].Value())
		if val == "" {
			sws.validationErr = "Server address cannot be empty"
		} else {
			// TODO: verify if it's a valid server address before
			// accepting.
			sws.serverAddr = val
			err := sws.generateConfig()
			if err != nil {
				sws.validationErr = fmt.Sprintf("Unable to generate config: %v", err)
			} else {
				// Success! Keep initializing the app.
				sws.completed = true
				sws.connCancel()
				return sws, tea.Quit
			}
		}

	case swsStageWalletPass:
		cmd := sws.updateFocused(msg)
		if !isEnterMsg(msg) {
			return sws, cmd
		}
		if sws.focusIndex == 0 {
			sws.setFocus(1)
			return sws, nil
		}

		if sws.inputs[0].Value() != sws.inputs[1].Value() {
			sws.validationErr = "Passphrases are not equal"
			sws.inputs[0].SetValue("")
			sws.inputs[1].SetValue("")
			sws.setFocus(0)
			return sws, nil
		}
		if len(sws.inputs[0].Value()) < 8 {
			sws.validationErr = "Passphrases cannot be less than 8 characters long"
			sws.inputs[0].SetValue("")
			sws.inputs[1].SetValue("")
			sws.setFocus(0)
			return sws, nil
		}

		sws.validationErr = ""
		sws.walletPass = sws.inputs[0].Value()
		sws.stage = swsStageCreatingWallet
		return sws, func() tea.Msg {
			return cmdCreateWallet(sws.connCtx, sws.lndc, sws.walletPass, sws.seedWords, sws.mcbBytes)
		}

	case swsStageWalletRestore:
		cmd := sws.updateFocused(msg)
		if !isEnterMsg(msg) {
			return sws, cmd
		}
		if sws.focusIndex == 0 {
			sws.setFocus(1)
			return sws, nil
		}

		seedWords := strings.Split(strings.TrimSpace(sws.inputs[0].Value()), " ")
		if len(seedWords) != 24 {
			sws.validationErr = "Seed does not contain 24 words"
			sws.inputs[0].Reset()
			sws.setFocus(0)
			return sws, nil
		}

		// Providing the channel backup is optional
		mcbPath := strings.TrimSpace(sws.inputs[1].Value())
		if len(mcbPath) > 0 {
			mcbPath = filepath.Clean(mcbPath)
			var mcbBytes []byte

			mcbBytes, err := os.ReadFile(mcbPath)
			if err != nil {
				sws.validationErr = fmt.Sprintf("failed to read %v: %v", mcbPath, err)
				sws.setFocus(1)
				return sws, nil
			}
			sws.mcbBytes = mcbBytes
		}
		sws.seedWords = seedWords
		sws.validationErr = ""
		sws.stage = swsStageWalletPass
		cmd = sws.initInputsWalletPass()
		return sws, cmd

	case swsStageSeed:
		cmd := sws.updateFocused(msg)
		if !isEnterMsg(msg) {
			return sws, cmd
		}

		if strings.ToLower(sws.inputs[0].Value()) != "ok" {
			return sws, nil
		}

		sws.stage = swsStageServer
		cmd = sws.initInputsServer()
		return sws, cmd
	}

	return sws, nil
}

func (sws setupWizardScreen) innerView() string {
	switch sws.stage {
	case swsStageWaitingRunForWalletPass, swsStageWaitingRunForWalletRestore:
		return "Waiting for the embedded dcrlnd instance to initialize..."
	case swsStageNetwork:
		return sws.selNetwork.View()
	case swsStageWalletType:
		return sws.selWalletType.View()
	case swsStageNewOrRestore:
		return sws.selNewOrRestore.View()
	case swsStageExternalDetails, swsStageServer, swsStageWalletRestore, swsStageWalletPass:
		var b strings.Builder

		for i := range sws.inputs {
			b.WriteString(sws.inputs[i].View())
			b.WriteString("\n\n")
		}
		b.WriteString(sws.styles.err.Render(sws.validationErr))

		switch sws.stage {
		case swsStageWalletRestore:
			b.WriteString("\n\n")
			b.WriteString("The channel backup file is optional;  This will have\n")
			b.WriteString("your channel's counterparties force-close them.\n")
			b.WriteString("Execute '/ln restoremultiscb <scb-file>' if you wish\n")
			b.WriteString("to do it at a later time.\n")
		}

		return b.String()

	case swsStageCreatingWallet:
		return "Creating wallet..."

	case swsStageSeed:
		var b strings.Builder
		b.WriteString("Please copy the wallet seed to keep it safe\n\n")
		for i, word := range bytes.Split(sws.seed, []byte(" ")) {
			b.Write(word)
			if i%5 == 4 {
				b.WriteString("\n")
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString("\n\n")

		b.WriteString("ATTENTION: the seed is *ESSENTIAL* to recover the\n")
		b.WriteString("funds of the wallet. Keep it in a physical, secure\n")
		b.WriteString("location. LOSING ACCESS TO THE SEED MAY RESULT IN\n")
		b.WriteString("LOSS OF FUNDS.\n\n")

		for i := range sws.inputs {
			b.WriteString(sws.inputs[i].View())
			b.WriteString("\n\n")
		}
		b.WriteString(sws.styles.err.Render(sws.validationErr))

		return b.String()

	default:
		return fmt.Sprintf("unknown stage %d", sws.stage)
	}
}

func (sws setupWizardScreen) View() string {
	inner := sws.innerView()
	return fmt.Sprintf("Initial Client Setup\n\n%s", inner)
}

func newSetupWizardScreen(cfgFilePath string) setupWizardScreen {
	theme, err := newTheme(nil)
	if err != nil {
		panic(err)
	}

	networks := []string{"mainnet", "testnet", "simnet"}
	walletTypes := []string{"internal", "external"}
	newOrRestore := []string{"new", "restore"}

	selNetwork := selection.New("Network", selection.Choices(networks))
	selNetwork.Filter = nil

	selWalletType := selection.New("Wallet Type", selection.Choices(walletTypes))
	selWalletType.Filter = nil

	selNewOrRestore := selection.New("New or Restore", selection.Choices(newOrRestore))
	selNewOrRestore.Filter = nil

	connCtx, connCancel := context.WithCancel(context.Background())

	return setupWizardScreen{
		cfgFilePath: cfgFilePath,
		stage:       swsStageNetwork,
		styles:      theme,

		selNetwork:      selection.NewModel(selNetwork),
		selWalletType:   selection.NewModel(selWalletType),
		selNewOrRestore: selection.NewModel(selNewOrRestore),

		connCtx:    connCtx,
		connCancel: connCancel,
	}
}

func cmdRunDcrlnd(ctx context.Context, cfg embeddeddcrlnd.Config) tea.Msg {
	lndc, err := embeddeddcrlnd.RunDcrlnd(ctx, cfg)
	if err != nil {
		return runDcrlndErrMsg{err}
	}
	return lndc
}

func cmdCreateWallet(ctx context.Context, lndc *embeddeddcrlnd.Dcrlnd, pass string, existingSeed []string, mcbBytes []byte) tea.Msg {
	seed, err := lndc.Create(ctx, pass, existingSeed, mcbBytes)
	if err != nil {
		err = fmt.Errorf("unable to create wallet: %v", err)
	}
	return createWalletResult{seed, err}
}
