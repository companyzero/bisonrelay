//go:build openbsd

package rtdtserver

import (
	"net"

	"golang.org/x/sys/unix"
)

func initKernelStatsTracker(inner *net.UDPConn) (kernelStatsTracker, error) {
	st := obsdKernelStatsTracker{inner: inner}

	// Try one gather to ensure it works.
	_, err := st.stats()
	return st, err
}

type obsdKernelStatsTracker struct {
	inner *net.UDPConn
}

func (st obsdKernelStatsTracker) stats() (UDPProcStats, error) {
	var stats UDPProcStats
	sc, err := st.inner.SyscallConn()
	if err != nil {
		return stats, err
	}

	var getOptErr error
	err = sc.Control(func(ufd uintptr) {
		var err error
		var optval int
		fd := int(ufd)

		// The following only returns the socket buffer size, not the
		// amount of outstanding data in the buffers.
		/*
			// Get receive buffer info
			if optval, getOptErr = unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF); err != nil {
				return

			}
			stats.RXQueue = int(optval)

			// Get send buffer info
			if optval, getOptErr = unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_SNDBUF); err != nil {
				return

			}
			stats.TXQueue = int(optval)
		*/

		// Get error count
		if optval, getOptErr = unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR); err != nil {
			return
		}
		stats.Drops = int(optval)
	})
	if err != nil {
		return stats, err
	}
	if getOptErr != nil {
		return stats, getOptErr
	}

	return stats, nil
}
