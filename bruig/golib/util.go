package golib

import (
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pbnjay/memory"
	"github.com/prometheus/procfs"
)

func fingerprintDER(c *x509.Certificate) string {
	d := sha256.New()
	d.Write(c.Raw)
	digest := d.Sum(nil)
	return hex.EncodeToString(digest)
}

// copyFileToWriter copies the contents of fname to w.
func copyFileToWriter(fname string, w io.Writer) error {
	f, err := os.Open(fname)
	if err != nil {
		return err
	}

	// If filename ends in .gz, copy from it decompressing so that it ends
	// up compressed only once in the zip archive.
	var r io.Reader = f
	if strings.HasSuffix(fname, ".gz") {
		gf, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return err
		}
		r = gf
	}

	_, err = io.Copy(w, r)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// createZipFileFromFile creates a new file in the zip archive, with metadata
// based on the passed fname. The file is stored in baseZipDir.
func createZipFileFromFile(zipFile *zip.Writer, fname, baseZipDir string) (io.Writer, error) {
	stat, err := os.Stat(fname)
	if err != nil {
		return nil, err
	}
	fheader, err := zip.FileInfoHeader(stat)
	if err != nil {
		return nil, err
	}

	// Strip .gz because archiveOldLogs() decompresses.
	fheader.Name = strings.TrimSuffix(fheader.Name, ".gz")
	if baseZipDir != "" {
		fheader.Name = baseZipDir + "/" + fheader.Name
	}

	fheader.Method = zip.Deflate

	return zipFile.CreateHeader(fheader)
}

// archiveGzippedLogs goes through all old logs in Dir(logFname) and archives
// them in the passed zip file.
func archiveOldLogs(zipFile *zip.Writer, logFname, baseZipDir string) (int, error) {
	dir := filepath.Dir(logFname)
	glob := filepath.Join(dir, filepath.Base(logFname)+".*")
	existing, err := filepath.Glob(glob)
	if err != nil {
		return 0, err
	}

	for _, fname := range existing {
		w, err := createZipFileFromFile(zipFile, fname, baseZipDir)
		if err != nil {
			return 0, err
		}

		err = copyFileToWriter(fname, w)
		if err != nil {
			return 0, err
		}
	}
	return len(existing), nil
}

func readOomAdjInt(filename string) int {
	fullName := fmt.Sprintf("/proc/%d/%s", os.Getpid(), filename)
	content, err := os.ReadFile(fullName)
	if err != nil {
		return -1
	}

	res, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return -2
	}
	return res
}

type cpuTime struct {
	cpuTimeMs int64
	when      time.Time
}

func reportCmdResultLoop(startTime, lastTime time.Time, id int32, lastCPUTimes []cpuTime) {
	wallStartTime := startTime.Round(0)

	proc, procErr := procfs.NewProc(os.Getpid())
	var procStat procfs.ProcStat
	if procErr == nil {
		procStat, procErr = proc.Stat()
	}
	if procErr == nil {
		copy(lastCPUTimes[1:], lastCPUTimes[0:])
		lastCPUTimes[0].cpuTimeMs = int64(procStat.CPUTime() * 1000)
		lastCPUTimes[0].when = time.Now()
	}
	totalMem := memory.TotalMemory()
	freeMem := memory.FreeMemory()

	oomAdj := readOomAdjInt("oom_adj")
	oomScore := readOomAdjInt("oom_score")
	oomScoreAdj := readOomAdjInt("oom_score_adj")

	elapsed := time.Since(startTime).Truncate(time.Millisecond)
	elapsedWall := time.Now().Round(0).Sub(wallStartTime).Truncate(time.Millisecond)
	sinceLast := time.Since(lastTime).Truncate(time.Millisecond)
	sinceLastWall := time.Now().Round(0).Sub(lastTime.Round(0)).Truncate(time.Millisecond)

	log01 := fmt.Sprintf("CmdResultLoop: running "+
		"for %s (wall %s, since last %s/%s) at pid %d id %d with %d/%d (%.2f%%) free mem "+
		"%d SIGURG connected %v",
		elapsed, elapsedWall, sinceLast, sinceLastWall,
		os.Getpid(), id,
		freeMem, totalMem,
		float64(freeMem)/float64(totalMem)*100,
		sigUrgCount.Load(),
		isServerConnected.Load())

	var log02, log03 string
	if procErr != nil {
		log02 = fmt.Sprintf("CmdResultLoop: procStat error: %v", procErr)
	} else {
		log02 = fmt.Sprintf("CmdResultLoop: cpuTime %.6f (utime %d stime %d) "+
			"resMem %d virtMem %d, oom_adj %d oom_score %d oom_score_adj %d",
			procStat.CPUTime(), procStat.UTime, procStat.STime,
			procStat.ResidentMemory(), procStat.VirtualMemory(),
			oomAdj, oomScore, oomScoreAdj)
		log03 = "CmdResultLoop: recent cpu times "
		var sum int64
		for i := 1; i < len(lastCPUTimes); i++ {
			d := lastCPUTimes[i-1].cpuTimeMs - lastCPUTimes[i].cpuTimeMs
			sum += d
			log03 += fmt.Sprintf("%d ", d)
		}
		l := len(lastCPUTimes) - 1
		totalTime := lastCPUTimes[0].when.Sub(lastCPUTimes[l].when).Truncate(time.Millisecond)
		totalTimeWall := lastCPUTimes[0].when.Round(0).Sub(lastCPUTimes[l].when.Round(0)).Truncate(time.Millisecond)
		log03 += fmt.Sprintf("(sum: %d, time %s, wall time %s)", sum,
			totalTime, totalTimeWall)
	}

	cmtx.Lock()
	if cs == nil || cs[0x12131400] == nil {
		cmtx.Unlock()
		fmt.Println(log01)
		fmt.Println(log02)
		fmt.Println(log03)
		return
	}
	cctx := cs[0x12131400]
	cctx.log.Info(log01)
	cctx.log.Info(log02)
	cctx.log.Info(log03)
	cmtx.Unlock()
}
