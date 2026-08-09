package config

import (
	"testing"
)

func TestUpdateManualSettings(t *testing.T) {
	Current = AppConfig{GroupRules: map[string][][]string{"JP": {{"Japan"}}, "SG": {{"Singapore"}}}}
	manualMode = false
	manualIPs = make(map[string]string)
	if err := UpdateManualSettings(true, map[string]string{"JP": "203.0.113.10", "SG": ""}); err != nil {
		t.Fatalf("UpdateManualSettings returned error: %v", err)
	}
	manualMode, manualIPs := ManualSettings()
	if !manualMode || manualIPs["JP"] != "203.0.113.10" {
		t.Fatalf("unexpected manual settings: mode=%t ips=%v", manualMode, manualIPs)
	}
	if _, ok := manualIPs["SG"]; ok {
		t.Fatalf("empty manual IP should not be stored: %v", manualIPs)
	}

	if err := UpdateManualSettings(true, map[string]string{"JP": "not-an-ip"}); err == nil {
		t.Fatal("UpdateManualSettings accepted an invalid IPv4 address")
	}
	if err := UpdateManualSettings(false, nil); err != nil {
		t.Fatalf("UpdateManualSettings could not disable manual mode: %v", err)
	}
	manualMode, manualIPs = ManualSettings()
	if manualMode || len(manualIPs) != 0 {
		t.Fatalf("manual settings were not cleared: mode=%t ips=%v", manualMode, manualIPs)
	}
}
