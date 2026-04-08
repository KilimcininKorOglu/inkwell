package validation

import (
	"fmt"
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

// ResolvePublicIP resolves a hostname and returns the first public IP.
// This prevents DNS rebinding by using the resolved IP directly for
// connections instead of re-resolving the hostname.
func ResolvePublicIP(hostname string) (string, error) {
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return "", fmt.Errorf("DNS lookup failed for %s: %w", hostname, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses resolved for %s", hostname)
	}
	for _, ip := range ips {
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			continue
		}
		if isPrivateIP(parsedIP) {
			continue
		}
		return ip, nil
	}
	return "", fmt.Errorf("all resolved IPs are in private/internal ranges for %s", hostname)
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() || // RFC 1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
		ip.Equal(net.ParseIP("169.254.169.254")) // Cloud metadata endpoint
}
