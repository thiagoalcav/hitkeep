package blocking

import (
	"fmt"
	"net"
	"strings"
)

// NormalizeCIDR accepts a plain IP or CIDR string and returns canonical CIDR form.
func NormalizeCIDR(input string) (string, *net.IPNet, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", nil, fmt.Errorf("cidr value is required")
	}

	if strings.Contains(trimmed, "/") {
		_, ipNet, err := net.ParseCIDR(trimmed)
		if err != nil {
			return "", nil, fmt.Errorf("invalid cidr %q", trimmed)
		}
		return ipNet.String(), ipNet, nil
	}

	ip := net.ParseIP(trimmed)
	if ip == nil {
		return "", nil, fmt.Errorf("invalid ip %q", trimmed)
	}

	if ipv4 := ip.To4(); ipv4 != nil {
		ipNet := &net.IPNet{IP: ipv4, Mask: net.CIDRMask(32, 32)}
		return ipNet.String(), ipNet, nil
	}

	ipv6 := ip.To16()
	if ipv6 == nil {
		return "", nil, fmt.Errorf("invalid ip %q", trimmed)
	}

	ipNet := &net.IPNet{IP: ipv6, Mask: net.CIDRMask(128, 128)}
	return ipNet.String(), ipNet, nil
}
