package utils

import (
	"fmt"
	"net"
)

// ModifyIP takes an IP string, adds 1 to the last octet (or subtracts 1 if it's 254).
func ModifyIP(ipStr string) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address format: %s", ipStr)
	}

	ip = ip.To4()
	if ip == nil {
		return "", fmt.Errorf("not an IPv4 address: %s", ipStr)
	}

	lastOctet := ip[3]
	if lastOctet == 254 {
		ip[3]--
	} else if lastOctet == 255 {
		ip[3] = 254
	} else {
		ip[3]++
	}

	return ip.String(), nil
}
