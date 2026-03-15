package modes

import (
	"cf-optimizer/cloudflare"
	"cf-optimizer/config"
	"cf-optimizer/database"
	"cf-optimizer/latency"
	"cf-optimizer/providers"
	"cf-optimizer/tracer"
	"cf-optimizer/utils"
	"log"
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
	if !startMinimalRun() {
		log.Println("Minimal mode: previous run still in progress, skipping.")
		return
	}
	defer finishMinimalRun()

	log.Println("Starting to fetch and process IPs in minimal mode...")

	provider := &providers.UouinProvider{}
	ips, err := provider.FetchIPs()
	if err != nil {
		log.Printf("Minimal mode: Error fetching IPs from UouinProvider: %v", err)
		return
	}
	if len(ips) == 0 {
		log.Println("Minimal mode: No IPs found from UouinProvider.")
		return
	}
	log.Printf("Minimal mode: Fetched %d IPs from UouinProvider", len(ips))

	stopEarly := false

	for _, originalIP := range ips {
		if stopEarly {
			break
		}

		modifiedIP, err := utils.ModifyIP(originalIP)
		if err != nil {
			log.Printf("Minimal mode: Error modifying IP %s: %v", originalIP, err)
			continue
		}

		group := tracer.GetIPGroup(modifiedIP)
		log.Printf("Minimal mode: Original IP: %s, Modified IP: %s, Group: %s", originalIP, modifiedIP, group)

		// Store every processed IP to the database
		if err := database.InsertIP(modifiedIP, group); err != nil {
			log.Printf("Minimal mode: Error inserting IP %s into database: %v", modifiedIP, err)
		}

		if group == "SG_GD" {
			log.Println("Minimal mode: Found SG_GD group. Will stop processing further IPs after this batch.")
			stopEarly = true
		}
	}

	log.Println("Minimal mode: Finished processing IPs. Now updating DNS.")
	runLatencyTestAndDNSUpdateMinimal()
}

var (
	minimalRunMu   sync.Mutex
	minimalRunning bool
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

type minimalGroupTestResult struct {
	groupName string
	bestIP    string
}

func runLatencyTestAndDNSUpdateMinimal() {
	log.Println("Minimal mode: Starting DNS updates...")

	uniqueGroups := make(map[string]struct{})
	for _, hostInfo := range config.Current.HostMap {
		uniqueGroups[hostInfo.Group] = struct{}{}
	}

	var wg sync.WaitGroup
	resultsChan := make(chan minimalGroupTestResult, len(uniqueGroups))

	for group := range uniqueGroups {
		wg.Add(1)
		go func(groupName string) {
			defer wg.Done()
			bestIP, _ := findBestIPForGroupMinimal(groupName, 5, 1)
			if bestIP != "" {
				resultsChan <- minimalGroupTestResult{groupName: groupName, bestIP: bestIP}
			}
		}(group)
	}

	wg.Wait()
	close(resultsChan)

	bestIPsByGroup := make(map[string]string)
	for result := range resultsChan {
		bestIPsByGroup[result.groupName] = result.bestIP
	}

	for host, hostInfo := range config.Current.HostMap {
		if bestIP, ok := bestIPsByGroup[hostInfo.Group]; ok {
			log.Printf("Minimal mode: Updating DNS for %s (group: %s) to IP %s", host, hostInfo.Group, bestIP)
			err := cloudflare.UpdateDNSRecord(config.Current.Cloudflare.ZoneID, hostInfo.ID, config.Current.Cloudflare.APIToken, host, bestIP)
			if err != nil {
				log.Printf("Minimal mode: Error updating DNS for %s: %v", host, err)
			}
		} else {
			log.Printf("Minimal mode: No best IP found for group %s (host: %s), skipping DNS update.", hostInfo.Group, host)
		}
	}
	log.Println("Minimal mode: Finished DNS updates.")
}

func findBestIPForGroupMinimal(groupName string, latestLimit int, testsPerIP int) (string, time.Duration) {
	ips, err := database.GetLatestIPsByGroup(groupName, latestLimit)
	if err != nil {
		log.Printf("Minimal mode: Error getting IPs for group %s: %v", groupName, err)
		return "", 0
	}
	if len(ips) == 0 {
		log.Printf("Minimal mode: No IPs found for group %s", groupName)
		return "", 0
	}

	log.Printf("Minimal mode: Testing %d IPs for group %s...", len(ips), groupName)
	var bestIP string
	var minAvgLatency time.Duration

	for _, ip := range ips {
		var totalLatency time.Duration
		var successfulTests int
		for i := 0; i < testsPerIP; i++ {
			lat, err := latency.Measure(ip)
			if err != nil {
				log.Printf("Minimal mode: Test %d/%d for IP %s failed: %v", i+1, testsPerIP, ip, err)
			} else {
				totalLatency += lat
				successfulTests++
				log.Printf("Minimal mode: Test %d/%d for IP %s (group: %s): latency=%v", i+1, testsPerIP, ip, groupName, lat)
			}
			time.Sleep(1 * time.Second)
		}

		if successfulTests > 0 {
			avgLatency := totalLatency / time.Duration(successfulTests)
			log.Printf("Minimal mode: Average latency for IP %s: %v", ip, avgLatency)
			if minAvgLatency == 0 || avgLatency < minAvgLatency {
				minAvgLatency = avgLatency
				bestIP = ip
			}
		}
	}

	if bestIP != "" {
		log.Printf("Minimal mode: Best IP for group %s is %s with latency %v", groupName, bestIP, minAvgLatency)
	}
	return bestIP, minAvgLatency
}
