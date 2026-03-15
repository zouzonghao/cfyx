package providers

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const (
	apiURLTemplate = "https://api.uouin.com/index.php/index/Cloudflare?key=%s&time=%s"
)

// UouinProvider implements the Provider interface for the uouin.com API.
type UouinProvider struct{}

// ApiResponse represents the structure of the JSON response from the API.
type ApiResponse struct {
	Data struct {
		CTCC ProviderData `json:"ctcc"`
	} `json:"data"`
}

// ProviderData holds the IP information for a provider.
type ProviderData struct {
	Info []IPDetail `json:"info"`
}

// IPDetail holds the detailed information for a single IP address.
type IPDetail struct {
	IP string `json:"ip"`
}

// generateCloudflareKey creates the authentication key and timestamp for the API.
func (p *UouinProvider) generateCloudflareKey() (key string, timestamp string) {
	timeMillis := time.Now().UnixNano() / int64(time.Millisecond)
	timestamp = strconv.FormatInt(timeMillis, 10)

	secret1 := "DdlTxtN0sUOu"
	secret2 := "70cloudflareapikey"

	hasher1 := md5.New()
	hasher1.Write([]byte(secret1))
	h1 := hex.EncodeToString(hasher1.Sum(nil))

	finalInput := h1 + secret2 + timestamp

	hasher2 := md5.New()
	hasher2.Write([]byte(finalInput))
	key = hex.EncodeToString(hasher2.Sum(nil))

	return key, timestamp
}

// FetchIPs fetches IP data from the uouin.com API.
func (p *UouinProvider) FetchIPs() ([]string, error) {
	key, timestamp := p.generateCloudflareKey()
	url := fmt.Sprintf(apiURLTemplate, key, timestamp)

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

	var apiResponse ApiResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	var ips []string
	for _, detail := range apiResponse.Data.CTCC.Info {
		ips = append(ips, detail.IP)
	}

	return ips, nil
}
