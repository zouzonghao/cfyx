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
	ManualMode bool                  `yaml:"manualMode"`
	ManualIPs  map[string]string     `yaml:"manualIPs"`
}

var (
	Current    AppConfig
	configPath string
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
	configPath = filepath
	mu.Unlock()

	log.Printf("Successfully loaded configuration from %s.", filepath)
}

func IsManualMode() bool {
	mu.RLock()
	defer mu.RUnlock()
	return Current.ManualMode
}

func UpdateManualSettings(manualMode bool, manualIPs map[string]string) error {
	mu.Lock()
	defer mu.Unlock()

	if manualMode {
		for group, ip := range manualIPs {
			if _, ok := Current.GroupRules[group]; !ok {
				return fmt.Errorf("unknown group: %s", group)
			}
			parsedIP := net.ParseIP(ip)
			if parsedIP == nil || parsedIP.To4() == nil {
				return fmt.Errorf("group %s requires a valid IPv4 address", group)
			}
		}
	}

	if manualMode {
		for group := range Current.GroupRules {
			if manualIPs[group] == "" {
				return fmt.Errorf("manual IP is required for group %s", group)
			}
		}
	}

	updatedManualIPs := make(map[string]string, len(manualIPs))
	for group, ip := range manualIPs {
		updatedManualIPs[group] = ip
	}
	updated := Current
	updated.ManualMode = manualMode
	updated.ManualIPs = updatedManualIPs
	if err := saveLocked(updated); err != nil {
		return err
	}
	Current = updated
	return nil
}

func ManualSettings() (bool, map[string]string) {
	mu.RLock()
	defer mu.RUnlock()
	manualIPs := make(map[string]string, len(Current.ManualIPs))
	for group, ip := range Current.ManualIPs {
		manualIPs[group] = ip
	}
	return Current.ManualMode, manualIPs
}

func saveLocked(updated AppConfig) error {
	if configPath == "" {
		return fmt.Errorf("configuration path is not initialized")
	}
	data, err := yaml.Marshal(updated)
	if err != nil {
		return fmt.Errorf("marshal configuration: %w", err)
	}
	temporaryPath := configPath + ".tmp"
	if err := os.WriteFile(temporaryPath, data, 0600); err != nil {
		return fmt.Errorf("write temporary configuration: %w", err)
	}
	if err := os.Rename(temporaryPath, configPath); err != nil {
		return fmt.Errorf("replace configuration: %w", err)
	}
	return nil
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
