package config

import (
	"log"
	"os"

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
}

var (
	Current AppConfig
)

// LoadConfig loads configuration from config.yaml.
func LoadConfig(filepath string) {
	configFile, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatalf("Error reading %s: %v.", filepath, err)
	}

	if err := yaml.Unmarshal(configFile, &Current); err != nil {
		log.Fatalf("Error parsing %s: %v.", filepath, err)
	}

	if len(Current.GroupRules) == 0 || len(Current.HostMap) == 0 {
		log.Fatalf("Configuration is empty or invalid in %s.", filepath)
	}

	log.Printf("Successfully loaded configuration from %s.", filepath)
}
