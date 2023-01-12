package netutils

import (
	"fmt"
	"net"
	"runtime"
	"strings"
)

// Listen binds to the specified address, both on tcp4 and tcp6 when an empty
// host is specified.
func Listen(addr string) ([]net.Listener, error) {
	var hasIPv4, hasIPv6 bool

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("`%s` is not a normalized "+
			"listener address", addr)
	}

	// Empty host or host of * on plan9 is both IPv4 and IPv6.
	if host == "" || (host == "*" && runtime.GOOS == "plan9") {
		hasIPv4 = true
		hasIPv6 = true
	} else {

		// Remove the IPv6 zone from the host, if present.  The zone
		// prevents ParseIP from correctly parsing the IP address.
		// ResolveIPAddr is intentionally not used here due to the
		// possibility of leaking a DNS query over Tor if the host is a
		// hostname and not an IP address.
		zoneIndex := strings.Index(host, "%")
		if zoneIndex != -1 {
			host = host[:zoneIndex]
		}

		ip := net.ParseIP(host)
		switch {
		case ip == nil:
			return nil, fmt.Errorf("`%s` is not a valid IP address", host)
		case ip.To4() == nil:
			hasIPv6 = true
		default:
			hasIPv4 = true
		}
	}
	listeners := make([]net.Listener, 0, 2)
	if hasIPv4 {
		listener, err := net.Listen("tcp4", addr)
		if err != nil {
			return nil, fmt.Errorf("unable to listen on tcp4:%s: %v", addr, err)
		}
		listeners = append(listeners, listener)
	}
	if hasIPv6 {
		listener, err := net.Listen("tcp6", addr)
		if err != nil {
			return nil, fmt.Errorf("unable to listen on tcp6:%s: %v", addr, err)
		}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}
