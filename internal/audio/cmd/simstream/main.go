// Simulate playback with variable network conditions.
//
// Captures from a capture device, applies some variable network simulation,
// then playback on the device.
//
// Use it with a virtual audio input device for better ergonomics (e.g.
// module-virtual-sink on Linux or Blackhole in MacOS).

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"

	"github.com/companyzero/bisonrelay/internal/audio"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

// printDevices prints info about audio devices.
func printDevices(devices *audio.Devices) error {
	pf := func(format string, args ...interface{}) {
		fmt.Println(fmt.Sprintf(format, args...))
	}

	printDevice := func(i int, dev *audio.Device) {
		defaultStr := ""
		if dev.IsDefault {
			defaultStr = "(default) "
		}
		pf("  Device %d %s%s", i, defaultStr, dev.Name)
		pf("  ID: %s", strescape.Nick(string(dev.ID)))
		pf("")
	}

	if len(devices.Capture) == 0 {
		pf("No audio capture devices found")
	} else {
		pf("Audio capture devices")
		pf("")
		for i := range devices.Capture {
			printDevice(i, &devices.Capture[i])
		}
	}

	if len(devices.Playback) == 0 {
		pf("No audio playback devices found")
	} else {
		pf("Audio playback devices")
		pf("")
		for i := range devices.Playback {
			printDevice(i, &devices.Playback[i])
		}
	}

	return nil
}

func realMain() error {
	flagListDevices := flag.Bool("lsdev", false, "List audio devices and quit")
	flagCapDevice := flag.Int("capdev", -1, "Capture device. Use -1 for system default")
	flagPlayDevice := flag.Int("playdev", -1, "Playback device. Use -1 for system default")
	flagCPUProfile := flag.String("cpuprofile", "", "Generate CPU profile")
	flagMemProfile := flag.String("memprofile", "", "Generate Mem profile")
	flagDebugLevel := flag.String("debuglevel", "info", "Log level to use")
	flag.Parse()

	logLevel := slog.LevelInfo
	switch *flagDebugLevel {
	case "info":
	case "debug":
		logLevel = slog.LevelDebug
	case "trace":
		logLevel = slog.LevelTrace
	default:
		return fmt.Errorf("unknown log level %q", *flagDebugLevel)
	}

	logBknd := slog.NewBackend(os.Stdout)
	log := logBknd.Logger("MAIN")
	log.SetLevel(logLevel)

	// Start CPU profiling.
	if *flagCPUProfile != "" {
		f, err := os.Create(*flagCPUProfile)
		if err != nil {
			return err
		}

		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}
	if *flagMemProfile != "" {
		defer func() {
			f, err := os.Create(*flagMemProfile)
			if err == nil {
				runtime.GC()
				err = pprof.WriteHeapProfile(f)
			}
			if err == nil {
				f.Close()
			}
		}()
	}

	devices, err := audio.ListAudioDevices(log)
	if err != nil {
		return err
	}

	if *flagListDevices {
		return printDevices(&devices)
	}

	logNoteRec := logBknd.Logger("AREC")
	logNoteRec.SetLevel(logLevel)
	noterec, err := audio.NewRecorder(logNoteRec)
	if err != nil {
		return err
	}

	// Set devices.
	if *flagCapDevice > -1 {
		if *flagCapDevice >= len(devices.Capture) {
			return fmt.Errorf("capture device %d not found (%d devices)",
				*flagCapDevice, len(devices.Capture))
		}
		dev := devices.Capture[*flagCapDevice]
		noterec.SetCaptureDevice(dev.ID)
	}
	if *flagPlayDevice > -1 {
		if *flagPlayDevice >= len(devices.Playback) {
			return fmt.Errorf("playback device %d not found (%d devices)",
				*flagPlayDevice, len(devices.Playback))
		}
		dev := devices.Playback[*flagPlayDevice]
		noterec.SetPlaybackDevice(dev.ID)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)

	ss, err := newSimStream(ctx, noterec, log)
	if err != nil {
		return err
	}
	g.Go(func() error { return ss.run(gctx) })

	userCtl, err := initUserInputCtl(ss)
	if err != nil {
		return err
	}
	g.Go(func() error { return userCtl.run(gctx) })

	err = g.Wait()
	if errors.Is(err, context.Canceled) {
		err = nil
	}

	noterec.Stop()
	<-ss.cs.CaptureDone()
	<-ss.ps.PlaybackDone()

	return err
}

func main() {
	err := realMain()
	if err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}
}
