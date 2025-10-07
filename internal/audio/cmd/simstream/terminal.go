package main

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type userInputCtl struct {
	oldTermios *unix.Termios
	ss         *simstream
	psGain     float64
	csGain     float64
}

func printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
	fmt.Println("")
}

func (ctl *userInputCtl) processInput(_ context.Context, in []byte) {
	switch in[0] {
	case '+':
		ctl.psGain += 1
		ctl.ss.ps.SetVolumeGain(ctl.psGain)
		printf("Set playback gain to %.2f", ctl.psGain)

	case '-':
		ctl.psGain -= 1
		ctl.ss.ps.SetVolumeGain(ctl.psGain)
		printf("Set playback gain to %.2f", ctl.psGain)

	case '>':
		ctl.csGain += 1
		ctl.ss.cs.SetVolumeGain(ctl.csGain)
		printf("Set capture gain to %.2f", ctl.csGain)

	case '<':
		ctl.csGain -= 1
		ctl.ss.cs.SetVolumeGain(ctl.csGain)
		printf("Set capture gain to %.2f", ctl.csGain)

	case 'j':
		newPlMilli := ctl.ss.packetLossMilli.Add(-10)
		printf("Set packetLoss chance to %.2f%%", float64(newPlMilli)/1000*100)

	case 'k':
		newPlMilli := ctl.ss.packetLossMilli.Add(10)
		printf("Set packetLoss chance to %.2f%%", float64(newPlMilli)/1000*100)

	}
}

func (ctl *userInputCtl) run(ctx context.Context) error {
	defer restoreTerminal(os.Stdin, ctl.oldTermios)
	b := make([]byte, 1)

	printf("Packet Loss  : %.2f%%", float64(ctl.ss.packetLossMilli.Load())/1000*100)
	printf("Min Delay    : %dms", ctl.ss.minDelayMs.Load())
	printf("Avg Delay    : %dms", ctl.ss.meanDelayMs.Load())
	printf("Std Dev Delay: %dms", ctl.ss.stdDevDelayMs.Load())

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
