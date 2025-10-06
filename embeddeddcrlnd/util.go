package embeddeddcrlnd

import (
	"fmt"
	"net"
)

// findAvailablePort tries to find an open tcp port in the 127.0.0.1 interface.
// Note: the port could be reused between when this is called and when it's
// actually bound.
func findAvailablePort() (uint16, error) {
	for i := 32723; i < 65535; i++ {
		lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", i)) //nolint:noctx
		if err != nil {
			continue
		}

		if err := lis.Close(); err != nil {
			continue
		}

		return uint16(i), nil
	}
	return 0, fmt.Errorf("unnable to find available port for gRPC")
}
