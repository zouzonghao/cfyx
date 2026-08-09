package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateManualSettings(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(path, []byte("groupRules:\n  JP: [[\"Japan\"]]\nhostMap:\n  example.com:\n    group: JP\n    id: record\n"), 0600); err != nil {
		t.Fatal(err)
	}

	LoadConfig(path)
	if err := UpdateManualSettings(true, map[string]string{"JP": "203.0.113.10"}); err != nil {
		t.Fatalf("UpdateManualSettings returned error: %v", err)
	}
	manualMode, manualIPs := ManualSettings()
	if !manualMode || manualIPs["JP"] != "203.0.113.10" {
		t.Fatalf("unexpected manual settings: mode=%t ips=%v", manualMode, manualIPs)
	}

	if err := UpdateManualSettings(true, map[string]string{"JP": "not-an-ip"}); err == nil {
		t.Fatal("UpdateManualSettings accepted an invalid IPv4 address")
	}
	if err := UpdateManualSettings(true, map[string]string{}); err == nil {
		t.Fatal("UpdateManualSettings accepted missing manual IPs")
	}

	LoadConfig(path)
	if !Current.ManualMode || Current.ManualIPs["JP"] != "203.0.113.10" {
		t.Fatalf("settings were not persisted: %+v", Current)
	}
}
