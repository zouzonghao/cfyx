package providers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// IpdbProvider implements the Provider interface for the ipdb API.
type IpdbProvider struct{}

// FetchIPs fetches IP data from the ipdb API.
func (p *IpdbProvider) FetchIPs() ([]string, error) {
	url := "https://ipdb.api.030101.xyz/?type=bestcf"

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

	// The response is plain text, with each IP on a new line.
	ipList := strings.Split(string(body), "\n")

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
