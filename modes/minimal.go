package modes

import (
	"cf-optimizer/cloudflare"
	"cf-optimizer/config"
	"cf-optimizer/database"
	"cf-optimizer/latency"
	"cf-optimizer/providers"
	"cf-optimizer/tracer"
	"cf-optimizer/verifier"
	"log"
	"sort"
	"sync"
	"time"
)

// RunMinimalMode starts the application in minimal mode.
func RunMinimalMode() {
	log.Println("Running in minimal mode.")
	go fetchAndProcessMinimal()
	ticker := time.NewTicker(2 * time.Hour)
	go func() {
		for range ticker.C {
			fetchAndProcessMinimal()
		}
	}()
}

func fetchAndProcessMinimal() {
	if config.IsManualMode() {
		log.Println("Minimal mode: Manual mode is enabled, skipping automatic optimization.")
		return
	}

	if !startMinimalRun() {
		log.Println("Minimal mode: previous run still in progress, skipping.")
		return
	}
	defer finishMinimalRun()

	provider := &providers.UouinProvider{}
	sourceIPs, err := provider.FetchIPs()
	if err != nil {
		log.Printf("Minimal mode: Error fetching IPs from UouinProvider: %v", err)
		return
	}
	if len(sourceIPs) == 0 {
		log.Println("Minimal mode: No IPs found from UouinProvider.")
		return
	}
	log.Printf("Minimal mode: Fetched %d IPs from UouinProvider", len(sourceIPs))

	seen := make(map[string]struct{}, len(sourceIPs))
	uniqueIPs := make([]string, 0, len(sourceIPs))
	for _, ip := range sourceIPs {
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		uniqueIPs = append(uniqueIPs, ip)
	}
	log.Printf("Minimal mode: Using %d unique source IPs.", len(uniqueIPs))
	processIPs(uniqueIPs, "Minimal mode")
}

func processIPs(ips []string, modeName string) {
	newIPs, err := database.FilterExistingIPs(ips)
	if err != nil {
		log.Printf("%s: Error filtering existing IPs: %v", modeName, err)
		return
	}
	log.Printf("%s: %d new IPs to trace, %d already in DB.", modeName,
		len(newIPs), len(ips)-len(newIPs))

	for _, ip := range newIPs {
		group := tracer.GetIPGroup(ip)
		log.Printf("%s: Traced IP: %s, Group: %s", modeName, ip, group)
		if err := database.InsertIP(ip, group); err != nil {
			log.Printf("%s: Error inserting IP %s into database: %v", modeName, ip, err)
		}
	}

	results := measureIPs(ips, modeName)
	if len(results) == 0 {
		log.Printf("%s: No IPs with successful ping tests. Retaining previous best IPs.", modeName)
		return
	}
	updateDNS(results, modeName)
}

func groupHasHosts(group string) bool {
	for _, hostInfo := range config.Current.HostMap {
		if hostInfo.Group == group {
			return true
		}
	}
	return false
}

func measureIPs(ips []string, modeName string) []ipWithLatencyAndGroup {
	pingCount := 3
	if modeName == "Minimal mode" {
		pingCount = 2
	}

	var results []ipWithLatencyAndGroup
	for _, ip := range ips {
		group, err := database.GetGroupByIP(ip)
		if err != nil {
			log.Printf("%s: No group found for IP %s: %v", modeName, ip, err)
			continue
		}
		if !groupHasHosts(group) {
			log.Printf("%s: IP %s belongs to group %s without configured hosts, skipping latency test.", modeName, ip, group)
			continue
		}

		var totalLatency time.Duration
		var successfulTests int
		for i := 0; i < pingCount; i++ {
			lat, err := latency.Measure(ip)
			if err != nil {
				log.Printf("%s: Ping %d/%d for IP %s failed: %v", modeName, i+1, pingCount, ip, err)
			} else {
				totalLatency += lat
				successfulTests++
				log.Printf("%s: Ping %d/%d for IP %s (group: %s): latency=%v",
					modeName, i+1, pingCount, ip, group, lat)
			}
			time.Sleep(1 * time.Second)
		}

		if successfulTests > 0 {
			avgLatency := totalLatency / time.Duration(successfulTests)
			log.Printf("%s: IP %s (group: %s) avg latency: %v", modeName, ip, group, avgLatency)
			results = append(results, ipWithLatencyAndGroup{IP: ip, Group: group, Latency: avgLatency})
		}
	}
	return results
}

var (
	minimalRunMu   sync.Mutex
	minimalRunning bool
	bestIPsMu      sync.RWMutex
	bestIPsByGroup = make(map[string]string)
)

func startMinimalRun() bool {
	minimalRunMu.Lock()
	defer minimalRunMu.Unlock()
	if minimalRunning {
		return false
	}
	minimalRunning = true
	return true
}

func finishMinimalRun() {
	minimalRunMu.Lock()
	minimalRunning = false
	minimalRunMu.Unlock()
}

// ipWithLatencyAndGroup pairs an IP with its routing group and measured
// average latency, used by the shared processing pipeline.
type ipWithLatencyAndGroup struct {
	IP      string
	Group   string
	Latency time.Duration
}

// updateDNS groups the ping results by routing group, sorts each group by latency, verifies the top 3 candidates per group with xray, and updates
// DNS for all hosts mapping to that group.
func updateDNS(results []ipWithLatencyAndGroup, modeName string) {
	log.Printf("%s: Starting DNS updates...", modeName)

	// Group by group name
	byGroup := make(map[string][]ipWithLatencyAndGroup)
	for _, r := range results {
		byGroup[r.Group] = append(byGroup[r.Group], r)
	}

	// Sort each group by latency (ascending)
	for group := range byGroup {
		sort.Slice(byGroup[group], func(i, j int) bool {
			return byGroup[group][i].Latency < byGroup[group][j].Latency
		})
	}

	// Collect unique groups from HostMap
	uniqueGroups := make(map[string]struct{})
	for _, hostInfo := range config.Current.HostMap {
		uniqueGroups[hostInfo.Group] = struct{}{}
	}

	// Verify top 3 per group with xray, collect best IP.
	// Only groups with a verified IP this round are updated; groups without
	// results retain their previous cached best IP (consistent with DNS
	// update behavior, which skips groups without a verified IP).
	updatedIPs := make(map[string]string)
	for group := range uniqueGroups {
		groupResults, ok := byGroup[group]
		if !ok || len(groupResults) == 0 {
			log.Printf("%s: No IPs for group %s, skipping.", modeName, group)
			continue
		}

		candidates := groupResults
		if len(candidates) > 3 {
			candidates = candidates[:3]
		}

		bestIP := verifyCandidates(modeName, group, candidates)
		if bestIP != "" {
			updatedIPs[group] = bestIP
		}
	}

	bestIPsMu.Lock()
	for group, ip := range updatedIPs {
		bestIPsByGroup[group] = ip
	}
	bestIPsMu.Unlock()

	// Update DNS only for hosts whose group got a new verified IP this round
	for host, hostInfo := range config.Current.HostMap {
		if bestIP, ok := updatedIPs[hostInfo.Group]; ok {
			log.Printf("%s: Updating DNS for %s (group: %s) to IP %s", modeName,
				host, hostInfo.Group, bestIP)
			err := cloudflare.UpdateDNSRecord(
				config.Current.Cloudflare.ZoneID,
				hostInfo.ID,
				config.Current.Cloudflare.APIToken,
				host,
				bestIP,
			)
			if err != nil {
				log.Printf("%s: Error updating DNS for %s: %v", modeName, host, err)
			}
		} else {
			log.Printf("%s: No new verified IP for group %s (host: %s), skipping DNS update.", modeName,
				hostInfo.Group, host)
		}
	}
	log.Printf("%s: Finished DNS updates.", modeName)
}

// verifyCandidates tests candidates with xray in order, returning the first
// IP that passes. When verification is disabled, returns the lowest-latency
// candidate directly.
func verifyCandidates(modeName, groupName string, candidates []ipWithLatencyAndGroup) string {
	if !verifier.Enabled() {
		return candidates[0].IP
	}

	for i, c := range candidates {
		log.Printf("%s: Verifying IP %s (rank %d/%d, latency %v) for group %s",
			modeName, c.IP, i+1, len(candidates), c.Latency, groupName)
		if verifier.VerifyIP(c.IP) {
			log.Printf("%s: IP %s verified for group %s", modeName, c.IP, groupName)
			return c.IP
		}
		log.Printf("%s: IP %s failed verification, trying next candidate", modeName, c.IP)
	}

	log.Printf("%s: All %d candidates failed verification for group %s, skipping DNS update",
		modeName, len(candidates), groupName)
	return ""
}
