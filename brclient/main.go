package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/companyzero/bisonrelay/brclient/internal/sloglinesbuffer"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/embeddeddcrlnd"
	"github.com/companyzero/bisonrelay/lockfile"
	"github.com/decred/dcrlnd/build"
	"github.com/decred/slog"
)

func runSetupWizard(cfgFilePath string) (*config, *embeddeddcrlnd.Dcrlnd, bool, error) {
	m := newSetupWizardScreen(cfgFilePath)
	p := tea.NewProgram(m, tea.WithAltScreen())

	unlockDone := make(chan struct{})
	go listenToCrashSignals(p, unlockDone, slog.Disabled)

	resModel, err := p.StartReturningModel()
	close(unlockDone)
	if err != nil {
		return nil, nil, false, err
	}
	sws, ok := resModel.(setupWizardScreen)
	if !ok {
		return nil, nil, false, fmt.Errorf("resulting wizard model was not setupWizardScreen; got %T", sws)
	}

	if sws.err != nil {
		if sws.crashStack != nil {
			fmt.Fprintf(os.Stderr, string(sws.crashStack))
		}
		return nil, sws.lndc, false, fmt.Errorf("error during setup wizard: %v", sws.err)
	}
	if !sws.completed {
		return nil, sws.lndc, false, fmt.Errorf("setup wizard was canceled")
	}

	// Finally, return a fully loaded config
	cfg, err := loadConfig()
	return cfg, sws.lndc, sws.isRestore(), err
}

func runUnlockAndSyncDcrlnd(cfg *config, lndc *embeddeddcrlnd.Dcrlnd,
	lndLogLines *sloglinesbuffer.Buffer) (*embeddeddcrlnd.Dcrlnd, error) {

	var p *tea.Program
	msgSender := func(msg tea.Msg) {
		go func() { p.Send(msg) }()
	}
	m := newUnlockLNScreen(cfg, lndc, msgSender, lndLogLines)
	p = tea.NewProgram(m, tea.WithAltScreen())

	logEventLis := lndLogLines.Listen(func(s string) {
		msgSender(logUpdated{line: s})
	})

	unlockDone := make(chan struct{})
	go listenToCrashSignals(p, unlockDone, slog.Disabled)

	resModel, err := p.StartReturningModel()
	close(unlockDone)
	logEventLis.Close()
	if err != nil {
		return nil, err
	}
	ulns, ok := resModel.(unlockLNScreen)
	if !ok {
		return nil, fmt.Errorf("resulting wizard model was not unlockLNScreen; got %T", ulns)
	}

	if ulns.err != nil {
		if ulns.crashStack != nil {
			fmt.Fprintln(os.Stderr, string(ulns.crashStack))
		}
		return ulns.lndc, fmt.Errorf("error during setup wizard: %v", ulns.err)
	}

	// Override the config options related to connecting to dcrlnd.
	lndc = ulns.lndc
	cfg.LNRPCHost = lndc.RPCAddr()
	cfg.LNTLSCertPath = lndc.TLSCertPath()
	cfg.LNMacaroonPath = lndc.MacaroonPath()

	return lndc, nil
}

func realMain() error {
	var lndc *embeddeddcrlnd.Dcrlnd
	var isRestore bool
	defer func() {
		// Stop internal dcrlnd if needed.
		if lndc == nil {
			return
		}
		fmt.Println("Shutting down internal LN wallet...")
		lndc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		if err := lndc.Wait(ctx); err != nil {
			fmt.Println("Error running dcrlnd:", err)
		}
	}()

	// Redirect the embedded dcrlnd logs from stdout.
	lndLogLines := new(sloglinesbuffer.Buffer)
	build.Stdout = lndLogLines

	// Load config.
	args, err := loadConfig()

	// Go into new setup wizard if config does not exist.
	var errNewCfg errConfigDoesNotExist
	if errors.As(err, &errNewCfg) {
		args, lndc, isRestore, err = runSetupWizard(errNewCfg.configPath)
	}
	if err != nil {
		return err
	}

	// Start CPU profiling.
	if args.CPUProfile != "" {
		f, err := os.Create(args.CPUProfile)
		if err != nil {
			return err
		}
		if args.CPUProfileHz > 0 {
			runtime.SetCPUProfileRate(args.CPUProfileHz)
		}

		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	// At this point, we know the db root, so use an app-wide lockfile to
	// handle the case where dcrlnd is internal.
	lockFilePath := filepath.Join(args.DBRoot, clientintf.LockFileName)
	ctxLF, cancel := context.WithTimeout(context.Background(), time.Second)
	lf, err := lockfile.Create(ctxLF, lockFilePath)
	cancel()
	if err != nil {
		return fmt.Errorf("unable to create lockfile %q: %v", lockFilePath, err)
	}
	defer lf.Close()

	if args.WalletType == "internal" {
		lndc, err = runUnlockAndSyncDcrlnd(args, lndc, lndLogLines)
		if err != nil {
			return err
		}
	}

	// Run main app.
	var p *tea.Program
	msgSender := func(msg tea.Msg) {
		if p == nil {
			return
		}
		go func() { p.Send(msg) }()
	}
	as, err := newAppState(msgSender, lndLogLines, isRestore, args)
	if err != nil {
		return err
	}

	p = tea.NewProgram(
		newInitStepState(as, nil), // initial state
		tea.WithAltScreen(),       // fullscreen
		//tea.WithMouseCellMotion(), // mouse support for mouse wheel
	)

	progDoneChan := make(chan struct{})
	go listenToCrashSignals(p, progDoneChan, as.log)
	err = p.Start()
	close(progDoneChan)
	if err != nil {
		return err
	}

	crashStack, runErr := as.getExitState()
	if crashStack != "" {
		fmt.Fprintln(os.Stderr, crashStack)
	}
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return runErr
	}
	return nil
}

func main() {
	err := realMain()
	if err != nil && !errors.Is(err, errCmdDone) {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
