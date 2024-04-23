package golib

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sync"
	"time"
)

type profileRotation struct {
	dir string
	ts  time.Time
}

type profiler struct {
	runOnce    sync.Once
	rotateChan chan chan profileRotation
}

var globalProfiler = &profiler{
	rotateChan: make(chan chan profileRotation),
}

// rotate the profiler file and return the unix timestamp of the last finished
// profile.
func (p *profiler) rotate() profileRotation {
	ch := make(chan profileRotation, 1)
	p.rotateChan <- ch
	return <-ch
}

// zipLogs zips the timed profiling logs to the destination path.
func (p *profiler) zipLogs(destPath string) error {
	lastProf := p.rotate()

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer destFile.Close()

	zipFile := zip.NewWriter(destFile)

	files, err := os.ReadDir(lastProf.dir)
	if err != nil {
		return err
	}
	var n int
	for _, f := range files {
		name := f.Name()
		if len(name) != 33 {
			continue
		}
		fts, err := time.Parse("2006-01-02T15-04-05", name[8:27])
		if err != nil {
			continue
		}
		if fts.After(lastProf.ts) {
			continue
		}

		fpath := filepath.Join(lastProf.dir, name)
		w, err := createZipFileFromFile(zipFile, fpath, "")
		if err != nil {
			return err
		}

		err = copyFileToWriter(fpath, w)
		if err != nil {
			return err
		}
		n++
	}

	fmt.Println("Zipped", n, "perf profile files")
	return zipFile.Close()
}

func (p *profiler) run(dir string) error {
	const profileInterval = time.Hour
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}
	for {
		start := time.Now().UTC()
		fname := filepath.Join(dir, start.Format("profile-2006-01-02T15-04-05.pprof"))
		f, err := os.Create(fname)
		if err != nil {
			return fmt.Errorf("unable to create cpu profile: %v", err)
		}
		pprof.StartCPUProfile(f)

		var ch chan profileRotation
		select {
		case <-time.After(profileInterval):
		case ch = <-p.rotateChan:
		}
		pprof.StopCPUProfile()
		f.Close()

		if ch != nil {
			ch <- profileRotation{dir: dir, ts: start}
		}
	}

}

func (p *profiler) Run(dir string) {
	p.runOnce.Do(func() {
		err := p.run(dir)
		if err != nil {
			fmt.Println("CPU Profiler error:", err.Error())
		}
	})
}
