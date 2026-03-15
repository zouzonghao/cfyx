package cloudflare

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const (
	apiURLTemplate = "https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s"
)

// UpdateDNSRecord updates a specific DNS record on Cloudflare.
func UpdateDNSRecord(zoneID, recordID, apiToken, hostName, ipAddress string) error {
	url := fmt.Sprintf(apiURLTemplate, zoneID, recordID)

	updateTime := time.Now().Format("2006年01月02日15时04分")
	jsonBody := fmt.Sprintf(`{
		"type": "A",
		"name": "%s",
		"content": "%s",
		"ttl": 1,
		"proxied": false,
		"comment": "更新时间：%s"
	}`, hostName, ipAddress, updateTime)

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer([]byte(jsonBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to update DNS record, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully updated DNS record for %s to %s", hostName, ipAddress)
	return nil
}
