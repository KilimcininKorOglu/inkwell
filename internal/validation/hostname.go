package validation

import (
	"net"
)

// IsPrivateHost resolves the given hostname and checks if any of its
// IP addresses fall within private/internal ranges. Returns true if
// the hostname resolves to a blocked (private) address.
func IsPrivateHost(hostname string) bool {
	ips, err := net.LookupHost(hostname)
	if err != nil || len(ips) == 0 {
		return true // DNS failure or no IPs = block (fail-safe)
	}
	for _, ip := range ips {
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			return true // unparseable IP = block
		}
		if isPrivateIP(parsedIP) {
			return true
		}
	}
	return false
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() || // RFC 1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
		ip.Equal(net.ParseIP("169.254.169.254")) // Cloud metadata endpoint
}
