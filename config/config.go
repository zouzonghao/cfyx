package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

// HostInfo contains the group and DNS record ID for a host.
type HostInfo struct {
	Group string `yaml:"group"`
	ID    string `yaml:"id"`
}

// CloudflareConfig holds the Cloudflare API credentials.
type CloudflareConfig struct {
	APIToken string `yaml:"api_token"`
	ZoneID   string `yaml:"zone_id"`
}

// AppConfig holds the application's configuration.
type AppConfig struct {
	GroupRules map[string][][]string `yaml:"groupRules"`
	HostMap    map[string]HostInfo   `yaml:"hostMap"`
	Cloudflare CloudflareConfig      `yaml:"cloudflare"`
	FullMode   bool                  `yaml:"fullMode"`
	Timezone   string                `yaml:"timezone"`
	Vless      string                `yaml:"vless"`
	VerifyURL  string                `yaml:"verifyURL"`
}

var (
	Current    AppConfig
	manualMode bool
	manualIPs  = make(map[string]string)
	mu         sync.RWMutex
)

// LoadConfig loads configuration from config.yaml.
func LoadConfig(filepath string) {
	configFile, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatalf("Error reading %s: %v.", filepath, err)
	}

	var loaded AppConfig
	if err := yaml.Unmarshal(configFile, &loaded); err != nil {
		log.Fatalf("Error parsing %s: %v.", filepath, err)
	}

	if len(loaded.GroupRules) == 0 || len(loaded.HostMap) == 0 {
		log.Fatalf("Configuration is empty or invalid in %s.", filepath)
	}

	mu.Lock()
	Current = loaded
	manualMode = false
	manualIPs = make(map[string]string)
	mu.Unlock()

	log.Printf("Successfully loaded configuration from %s.", filepath)
}

func IsManualMode() bool {
	mu.RLock()
	defer mu.RUnlock()
	return manualMode
}

func UpdateManualSettings(enabled bool, ips map[string]string) error {
	mu.Lock()
	defer mu.Unlock()

	updatedManualIPs := make(map[string]string, len(ips))
	if enabled {
		for group, ip := range ips {
			if ip == "" {
				continue
			}
			if _, ok := Current.GroupRules[group]; !ok {
				return fmt.Errorf("unknown group: %s", group)
			}
			parsedIP := net.ParseIP(ip)
			if parsedIP == nil || parsedIP.To4() == nil {
				return fmt.Errorf("group %s requires a valid IPv4 address", group)
			}
			updatedManualIPs[group] = ip
		}
	}

	manualMode = enabled
	manualIPs = updatedManualIPs
	return nil
}

func ManualSettings() (bool, map[string]string) {
	mu.RLock()
	defer mu.RUnlock()
	copyIPs := make(map[string]string, len(manualIPs))
	for group, ip := range manualIPs {
		copyIPs[group] = ip
	}
	return manualMode, copyIPs
}

func GroupNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	groups := make([]string, 0, len(Current.GroupRules))
	for group := range Current.GroupRules {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}
