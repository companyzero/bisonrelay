//go:build linux

package rtdtserver

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type linuxKernelStatsTracker struct {
	inode int
	isV6  bool
}

func initKernelStatsTracker(inner *net.UDPConn) (kernelStatsTracker, error) {
	// Try multiple times because it may take a few milliseconds for the
	// inode to show up in the list.
	var inode int
	var err error
	isV6 := strings.Contains(inner.LocalAddr().String(), "[") // Enough to determine version?
	for i := 0; i < 1000; i++ {
		inode, err = getUDPConnInode(os.Getpid(), inner)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		_, err = getUDPProcStats(inode, isV6)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
	}
	return linuxKernelStatsTracker{inode: inode, isV6: isV6}, err
}

func (st linuxKernelStatsTracker) stats() (UDPProcStats, error) {
	return getUDPProcStats(st.inode, st.isV6)
}

// getUDPConnInode retrieves the inode of a UDPConn for a given process ID.
// This function assumes it's running on a Linux system.
func getUDPConnInode(pid int, conn *net.UDPConn) (int, error) {
	sysConn, err := conn.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("unable to extract SyscallConn: %v", err)
	}

	var inode int
	var inodeErr error
	err = sysConn.Control(func(fd uintptr) {
		// Construct the path to the process's fd directory
		fdPath := filepath.Join("/proc", strconv.Itoa(pid), "fd")

		// Read the contents of the fd directory
		entries, err := os.ReadDir(fdPath)
		if err != nil {
			inodeErr = fmt.Errorf("error reading /proc/%d/fd: %w", pid, err)
			return
		}

		// Iterate through the file descriptors
		for _, entry := range entries {
			// Read the symlink target
			linkPath := filepath.Join(fdPath, entry.Name())
			target, err := os.Readlink(linkPath)
			if err != nil {
				continue // Skip if we can't read the symlink
			}

			// Check if this is the file descriptor we're looking for
			if !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
				// Skip if not a socket fd.
				continue
			}

			fdNum, err := strconv.ParseInt(entry.Name(), 10, 64)
			if err != nil {
				// Skip if we can't parse the fd number
				continue
			}

			if fdNum != int64(fd) {
				// Skip if it's not the fd we're looking for.
				continue
			}

			// These are the droids we were looking for.
			// Extract the inode from the target
			inodeStr := strings.TrimPrefix(target, "socket:[")
			inodeStr = strings.TrimSuffix(inodeStr, "]")

			inodeInt, err := strconv.ParseInt(inodeStr, 10, 32)
			if err != nil {
				inodeErr = fmt.Errorf("unable to decode inode as a number: %v", err)
				return
			}
			inode = int(inodeInt)
			return
		}

		inodeErr = fmt.Errorf("inode not found for the given UDPConn")
	})
	if err == nil && inodeErr != nil {
		err = inodeErr
	}
	return inode, err
}

var spaceRe = regexp.MustCompile(`\ +`)

// getUDPProcStats returns kernel socket stats for the given inode.
func getUDPProcStats(inode int, isV6 bool) (UDPProcStats, error) {
	var stats UDPProcStats

	procNetFname := "/proc/net/udp"
	if isV6 {
		procNetFname = "/proc/net/udp6"
	}

	f, err := os.Open(procNetFname)
	if err != nil {
		return stats, fmt.Errorf("unable to open %s: %v", procNetFname, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(bufio.NewReader(f))
	for scanner.Scan() {
		txt := strings.TrimSpace(scanner.Text())
		cols := spaceRe.Split(txt, -1)
		if len(cols) != 13 {
			continue
		}

		in, err := strconv.Atoi(cols[9])
		if err != nil {
			// Skip if col is not an inode int.
			continue
		}

		if in != inode {
			// Skip if it's not the target inode.
			continue
		}

		txrx := strings.Split(cols[4], ":")
		if len(txrx) != 2 {
			return stats, fmt.Errorf("tx:rx col not correctly split")
		}

		if tx, err := strconv.ParseInt(txrx[0], 16, 32); err != nil {
			return stats, fmt.Errorf("tx not a number: %v", err)
		} else {
			stats.TXQueue = int(tx)
		}
		if rx, err := strconv.ParseInt(txrx[1], 16, 32); err != nil {
			return stats, fmt.Errorf("rx not a number: %v", err)
		} else {
			stats.RXQueue = int(rx)
		}
		if stats.Drops, err = strconv.Atoi(cols[12]); err != nil {
			return stats, fmt.Errorf("drops not a number: %v", err)
		}

		return stats, nil
	}

	return stats, fmt.Errorf("could not find stats for target inode "+
		"%d (v6=%v)", inode, isV6)
}
