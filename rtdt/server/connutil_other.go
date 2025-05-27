//go:build !linux && !openbsd

package rtdtserver

import "net"

func initKernelStatsTracker(inner *net.UDPConn) (kernelStatsTracker, error) {
	return nullKernelStatsTracker{}, nil
}
