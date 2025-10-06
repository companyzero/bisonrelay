package main

import (
	"fmt"
	"net"
	"runtime"
	"syscall"

	"github.com/decred/slog"
)

// checkKernelUDPBufferSize checks the size of OS/kernel buffers to ensure
// they are not too small and could cause dropped packets.
func checkKernelUDPBufferSize(socket *net.UDPConn, ignoreSmallErr bool, log slog.Logger) error {
	// Double check buffer size.
	size, err := getKernelBufferSize(socket)
	if err != nil {
		return fmt.Errorf("unable to query kernel for UDP "+
			"buffer size of listen addr %s: %v", socket.LocalAddr(), err)
	}

	if size < minKernelBufferSize {
		log.Warnf("Kernel UDP buffer size for listen address %s "+
			"is too small (%d bytes)", socket.LocalAddr(), size)
		log.Warnf("Make sure the kernel buffer sizes are large enough " +
			"to avoid dropped packets in the kernel buffers")
		switch runtime.GOOS {
		case "linux":
			log.Warnf("Use `sysctl -w net.core.{rmem_max,rmem_default,wmem_max,wmem_default}=size_in_bytes` " +
				"to set the UDP kernel buffer sizes on Linux")
		case "openbsd":
			log.Warnf("Use `sysctl net.inet.udp.{recvspace,sendspace}=size_in_bytes` " +
				"to set the UDP kernel buffer sizes on OpenBSD")
		}

		if !ignoreSmallErr {
			log.Warnf("Set `ignoresmallkernelbuffers = 1` in the " +
				"config file to ignore the small kernel buffer " +
				"size error and start the server anyway")
			return fmt.Errorf("kernel UDP buffer for address %s "+
				"too small (%d bytes)", socket.LocalAddr(), size)
		}
	}

	return nil
}

// getKernelBufferSize returns the size of the kernel buffers used for the
// given socket.
func getKernelBufferSize(socket *net.UDPConn) (int, error) {
	var size int
	sysConn, err := socket.SyscallConn()
	if err != nil {
		return 0, err
	}

	var getErr error
	err = sysConn.Control(func(fd uintptr) {
		size, getErr = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
	})
	if err != nil {
		return 0, err
	}
	if getErr != nil {
		return 0, getErr
	}
	return size, nil
}
