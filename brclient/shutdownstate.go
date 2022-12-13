package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/decred/slog"
)

// shutdownState tracks the shutdown progress once the app has been commanded
// to shutdown.
type shutdownState struct {
	initless
	cancel func()
	err    error
	wg     *sync.WaitGroup
}

type shutdownDone struct{}

func (ss shutdownState) waitShutdown() tea.Msg {
	ss.cancel()
	ss.wg.Wait()
	return shutdownDone{}
}

func (ss shutdownState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case shutdownDone:
		// All cleanup done. Time to go.
		return ss, tea.Quit
	}
	return ss, nil
}

func (ss shutdownState) View() string {
	if ss.err == nil || errors.Is(ss.err, errQuitRequested) {
		return "Shutting down..."
	}
	return fmt.Sprintf("Shutting down due to err: %v", ss.err)
}

func maybeShutdown(as *appState, msg tea.Msg) (tea.Model, tea.Cmd) {
	crash := isCrashMsg(msg)
	if err := isQuitMsg(msg); err != nil || crash {
		if crash {
			as.storeCrash()
		}

		ss := shutdownState{
			wg:     &as.wg,
			cancel: as.cancel,
			err:    err,
		}
		return ss, ss.waitShutdown
	}

	return nil, nil
}

// listenToCrashSignals blocks until an abort signal is received or programDone
// is closed. If an abort signal is received, a crashApp{} message is sent to
// the program and after a few seconds, the program is forcefully terminated.
func listenToCrashSignals(p *tea.Program, programDone chan struct{}, log slog.Logger) {
	// Listen to one of the abort signals.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGABRT, syscall.SIGQUIT)
	select {
	case s := <-c:
		log.Warnf("Received signal %s (%d)", s, s)
	case <-programDone:
		signal.Reset()
		return
	}
	signal.Reset()
	go p.Send(crashApp{})

	// Wait for crash to be processed and program to terminate.
	select {
	case <-time.After(3 * time.Second):
	case <-programDone:
		return
	}

	// If we're still here a second after the SIGABRT, then crashApp{} was
	// not processed, so capture and log the current stack frame and hard
	// kill the program.
	log.Warnf("Hard terminating program")
	stack := string(allStack())
	log.Info(stack)
	p.Kill()
	panic(stack)
}
