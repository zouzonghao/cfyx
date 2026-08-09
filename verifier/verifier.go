package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const (
	defaultVerifyURL = "http://cp.cloudflare.com/generate_204"
	xrayBinaryPath   = "./xray"
)

// VlessConfig holds the parsed parameters from a VLESS URL.
type VlessConfig struct {
	UUID       string
	Address    string
	Port       int
	Encryption string
	Security   string
	Network    string
	Host       string
	Path       string
}

var (
	vlessCfg  *VlessConfig
	verifyURL string
)

// Init parses the VLESS URL and stores the verification target URL.
// If vlessURL is empty, verification is disabled and VerifyIP returns true.
func Init(vlessURL, targetURL string) error {
	if vlessURL == "" {
		vlessCfg = nil
		verifyURL = ""
		return nil
	}

	cfg, err := ParseVlessURL(vlessURL)
	if err != nil {
		return fmt.Errorf("failed to parse VLESS URL: %w", err)
	}
	vlessCfg = cfg

	if targetURL == "" {
		verifyURL = defaultVerifyURL
	} else {
		verifyURL = targetURL
	}

	log.Printf("Verifier initialized: address=%s, port=%d, host=%s, verifyURL=%s",
		cfg.Address, cfg.Port, cfg.Host, verifyURL)
	return nil
}

// Enabled returns true if VLESS verification is configured.
func Enabled() bool {
	return vlessCfg != nil
}

// ParseVlessURL parses a VLESS share URL into a VlessConfig.
func ParseVlessURL(vlessURL string) (*VlessConfig, error) {
	u, err := url.Parse(vlessURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if u.Scheme != "vless" {
		return nil, fmt.Errorf("not a vless URL: %s", vlessURL)
	}

	uuid := u.User.Username()
	if uuid == "" {
		return nil, fmt.Errorf("missing UUID in VLESS URL")
	}

	address := u.Hostname()
	if address == "" {
		return nil, fmt.Errorf("missing address in VLESS URL")
	}

	portStr := u.Port()
	if portStr == "" {
		portStr = "443"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port %q: %w", portStr, err)
	}

	q := u.Query()

	cfg := &VlessConfig{
		UUID:       uuid,
		Address:    address,
		Port:       port,
		Encryption: q.Get("encryption"),
		Security:   q.Get("security"),
		Network:    q.Get("type"),
		Host:       q.Get("host"),
		Path:       q.Get("path"),
	}

	if cfg.Encryption == "" {
		cfg.Encryption = "none"
	}
	if cfg.Network == "" {
		cfg.Network = "tcp"
	}
	if cfg.Path == "" {
		cfg.Path = "/"
	}

	return cfg, nil
}

// GenerateXrayConfig builds an xray JSON config that proxies through the
// given IP using the parsed VLESS parameters. The HTTP inbound listens on
// the specified local port.
func (c *VlessConfig) GenerateXrayConfig(ip string, localPort int) (string, error) {
	outbound := map[string]interface{}{
		"protocol": "vless",
		"settings": map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": ip,
					"port":    c.Port,
					"users": []interface{}{
						map[string]interface{}{
							"id":         c.UUID,
							"encryption": c.Encryption,
						},
					},
				},
			},
		},
	}

	streamSettings := map[string]interface{}{
		"network": c.Network,
	}

	if c.Security == "tls" {
		streamSettings["security"] = "tls"
		streamSettings["tlsSettings"] = map[string]interface{}{
			"serverName": c.Host,
		}
	}

	if c.Network == "ws" {
		wsSettings := map[string]interface{}{
			"path": c.Path,
		}
		if c.Host != "" {
			wsSettings["host"] = c.Host
		}
		streamSettings["wsSettings"] = wsSettings
	}

	outbound["streamSettings"] = streamSettings

	config := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"port":     localPort,
				"listen":   "127.0.0.1",
				"protocol": "http",
				"settings": map[string]interface{}{},
			},
		},
		"outbounds": []interface{}{outbound},
	}

	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal xray config: %w", err)
	}
	return string(data), nil
}

// CheckXrayBinary checks that the xray binary exists at ./xray and is
// executable. Returns an error if it is missing or not executable.
func CheckXrayBinary() error {
	info, err := os.Stat(xrayBinaryPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("xray binary not found at %s", xrayBinaryPath)
	}
	if err != nil {
		return fmt.Errorf("failed to stat xray binary: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a file", xrayBinaryPath)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("%s is not executable (run: chmod +x %s)", xrayBinaryPath, xrayBinaryPath)
	}
	return nil
}

// VerifyIP tests whether the given IP can establish a working VLESS proxy
// connection by starting a temporary xray instance and requesting the
// verification URL through it.
func VerifyIP(ip string) bool {
	if vlessCfg == nil {
		return true
	}

	port, err := getFreePort()
	if err != nil {
		log.Printf("VerifyIP: failed to obtain free port: %v", err)
		return false
	}

	configJSON, err := vlessCfg.GenerateXrayConfig(ip, port)
	if err != nil {
		log.Printf("VerifyIP: failed to generate config for %s: %v", ip, err)
		return false
	}

	tmpFile, err := os.CreateTemp("", "xray-config-*.json")
	if err != nil {
		log.Printf("VerifyIP: failed to create temp config: %v", err)
		return false
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(configJSON); err != nil {
		tmpFile.Close()
		log.Printf("VerifyIP: failed to write config: %v", err)
		return false
	}
	tmpFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, xrayBinaryPath, "run", "-c", tmpPath)
	if err := cmd.Start(); err != nil {
		log.Printf("VerifyIP: failed to start xray for %s: %v", ip, err)
		return false
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	if err := waitForPort(port, 5*time.Second); err != nil {
		log.Printf("VerifyIP: xray did not start for IP %s: %v", ip, err)
		return false
	}

	proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		log.Printf("VerifyIP: failed to parse proxy URL: %v", err)
		return false
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(verifyURL)
	if err != nil {
		log.Printf("VerifyIP: request through IP %s failed: %v", ip, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		log.Printf("VerifyIP: IP %s verified OK (status %d)", ip, resp.StatusCode)
		return true
	}

	log.Printf("VerifyIP: IP %s failed (status %d)", ip, resp.StatusCode)
	return false
}

func getFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}

func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}
