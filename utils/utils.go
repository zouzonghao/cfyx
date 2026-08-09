package utils

import (
	"fmt"
	"math/rand"
	"net"
)

// ExpandIP takes an IP string and returns 3 variant IPs derived from it:
//   - 末位+1 (decrements instead if 末位 >= 254)
//   - 2 random末位 in range 1-254
//
// Variants are deduplicated. The original IP is not included.
func ExpandIP(ipStr string) ([]string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address format: %s", ipStr)
	}

	ip = ip.To4()
	if ip == nil {
		return nil, fmt.Errorf("not an IPv4 address: %s", ipStr)
	}

	base := fmt.Sprintf("%d.%d.%d", ip[0], ip[1], ip[2])
	seen := make(map[string]bool)
	var result []string

	addVariant := func(lastOctet int) {
		if lastOctet < 1 || lastOctet > 254 {
			return
		}
		s := fmt.Sprintf("%s.%d", base, lastOctet)
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	// 末位+1（>= 254 时改为末位-1）
	if ip[3] >= 254 {
		addVariant(int(ip[3] - 1))
	} else {
		addVariant(int(ip[3]) + 1)
	}

	// 补充随机末位直到得到 3 个变体
	for len(result) < 3 {
		addVariant(rand.Intn(254) + 1)
	}

	return result, nil
}
