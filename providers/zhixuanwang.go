package providers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// ZhixuanwangProvider implements the Provider interface for the ip.164746.xyz API.
type ZhixuanwangProvider struct{}

// FetchIPs fetches IP data from the ip.164746.xyz API.
func (p *ZhixuanwangProvider) FetchIPs() ([]string, error) {
	url := "https://ip.164746.xyz/ipTop10.html"

	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching API URL %s: %w", url, err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("API status code error: %d %s", res.StatusCode, res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading API response body: %w", err)
	}

	// The response is plain text, with IPs separated by commas.
	ipList := strings.Split(string(body), ",")

	// Clean up the list: trim whitespace and remove empty strings.
	var cleanedIPs []string
	for _, ip := range ipList {
		trimmedIP := strings.TrimSpace(ip)
		if trimmedIP != "" {
			cleanedIPs = append(cleanedIPs, trimmedIP)
		}
	}

	return cleanedIPs, nil
}
