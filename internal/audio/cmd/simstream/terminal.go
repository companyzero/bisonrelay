package main

import (
	"context"
	"os"

	"golang.org/x/sys/unix"
)

type userInputCtl struct {
	oldTermios *unix.Termios
	ss         *simstream
	psGain     float64
	csGain     float64
}

func (ctl *userInputCtl) processInput(_ context.Context, in []byte) {
	switch in[0] {
	case '+':
		ctl.psGain += 1
		ctl.ss.ps.SetVolumeGain(ctl.psGain)

	case '-':
		ctl.psGain -= 1
		ctl.ss.ps.SetVolumeGain(ctl.psGain)

	case '>':
		ctl.csGain += 1
		ctl.ss.cs.SetVolumeGain(ctl.csGain)

	case '<':
		ctl.csGain -= 1
		ctl.ss.cs.SetVolumeGain(ctl.csGain)
	}
}

func (ctl *userInputCtl) run(ctx context.Context) error {
	defer restoreTerminal(os.Stdin, ctl.oldTermios)
	b := make([]byte, 1)

	stdin := os.Stdin

	readChan := make(chan int, 10)
	errChan := make(chan error, 10)
	readNext := func() {
		n, err := stdin.Read(b)
		if err != nil {
			errChan <- err
		} else {
			readChan <- n
		}
	}

	for ctx.Err() == nil {
		go readNext()
		select {
		case n := <-readChan:
			if n == 0 {
				continue
			}

			ctl.processInput(ctx, b)

		case err := <-errChan:
			return err
		case <-ctx.Done():
			return nil
		}
	}

	return nil
}

func initUserInputCtl(ss *simstream) (*userInputCtl, error) {
	oldTermios, err := makeRaw(os.Stdin)
	if err != nil {
		return nil, err
	}
	return &userInputCtl{oldTermios: oldTermios, ss: ss}, nil
}

func makeRaw(f *os.File) (*unix.Termios, error) {
	termios, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	if err != nil {
		return nil, err

	}

	oldTermios := *termios

	// Turn off ICANON (canonical mode) and ECHO
	termios.Lflag &^= unix.ICANON | unix.ECHO

	// Set minimum number of bytes for non-canonical read
	termios.Cc[unix.VMIN] = 1
	// Set timeout to 0 deciseconds
	termios.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(int(f.Fd()), unix.TCSETS, termios); err != nil {
		return nil, err

	}

	return &oldTermios, nil
}

func restoreTerminal(f *os.File, termios *unix.Termios) error {
	return unix.IoctlSetTermios(int(f.Fd()), unix.TCSETS, termios)

}
