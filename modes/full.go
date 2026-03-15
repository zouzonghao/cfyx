package modes

import (
	"cf-optimizer/cloudflare"
	"cf-optimizer/config"
	"cf-optimizer/database"
	"cf-optimizer/latency"
	"cf-optimizer/providers"
	"cf-optimizer/tracer"
	"log"
	"sync"
	"time"
)

// RunFullMode starts the application in full mode.
func RunFullMode() {
	log.Println("Running in full mode.")
	go fetchAndProcess()
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			fetchAndProcess()
		}
	}()
}

func fetchAndProcess() {
	if !startFullRun() {
		log.Println("Full mode: previous run still in progress, skipping.")
		return
	}
	defer finishFullRun()

	log.Println("Starting to fetch and process IPs from providers...")

	providersList := []providers.Provider{
		&providers.UouinProvider{},
		&providers.IpdbProvider{},
		&providers.ZhixuanwangProvider{},
	}

	var wg sync.WaitGroup
	ipsChan := make(chan []string, len(providersList))

	for _, p := range providersList {
		wg.Add(1)
		go func(provider providers.Provider) {
			defer wg.Done()
			ips, err := provider.FetchIPs()
			if err != nil {
				log.Printf("Error fetching IPs from a provider: %v", err)
				return
			}
			log.Printf("Fetched %d IPs from %T", len(ips), provider)
			ipsChan <- ips
		}(p)
	}

	wg.Wait()
	close(ipsChan)

	uniqueIPsMap := make(map[string]struct{})
	for ips := range ipsChan {
		for _, ip := range ips {
			uniqueIPsMap[ip] = struct{}{}
		}
	}

	var uniqueIPs []string
	for ip := range uniqueIPsMap {
		uniqueIPs = append(uniqueIPs, ip)
	}

	if len(uniqueIPs) == 0 {
		log.Println("No IPs found from any provider.")
		return
	}
	log.Printf("Found a total of %d unique IPs from all providers.", len(uniqueIPs))

	newIPs, err := database.FilterExistingIPs(uniqueIPs)
	if err != nil {
		log.Printf("Error filtering existing IPs from database: %v", err)
		return
	}

	if len(newIPs) == 0 {
		log.Println("No new IPs to process after filtering against the database.")
		return
	}

	log.Printf("Found %d new IPs to process.", len(newIPs))

	for _, ip := range newIPs {
		group := tracer.GetIPGroup(ip)
		log.Printf("IP: %s, Group: %s", ip, group)
		if err := database.InsertIP(ip, group); err != nil {
			log.Printf("Error inserting IP %s into database: %v", ip, err)
		}
	}

	log.Println("Finished processing all IPs.")
	go runLatencyTestAndDNSUpdate()
}

var (
	fullRunMu   sync.Mutex
	fullRunning bool
)

func startFullRun() bool {
	fullRunMu.Lock()
	defer fullRunMu.Unlock()
	if fullRunning {
		return false
	}
	fullRunning = true
	return true
}

func finishFullRun() {
	fullRunMu.Lock()
	fullRunning = false
	fullRunMu.Unlock()
}

type groupTestResult struct {
	groupName string
	bestIP    string
}

func runLatencyTestAndDNSUpdate() {
	log.Println("Starting latency tests and DNS updates...")

	uniqueGroups := make(map[string]struct{})
	for _, hostInfo := range config.Current.HostMap {
		uniqueGroups[hostInfo.Group] = struct{}{}
	}

	var wg sync.WaitGroup
	resultsChan := make(chan groupTestResult, len(uniqueGroups))

	for group := range uniqueGroups {
		wg.Add(1)
		go func(groupName string) {
			defer wg.Done()
			bestIP, _ := findBestIPForGroup(groupName)
			if bestIP != "" {
				resultsChan <- groupTestResult{groupName: groupName, bestIP: bestIP}
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
			err := cloudflare.UpdateDNSRecord(config.Current.Cloudflare.ZoneID, hostInfo.ID, config.Current.Cloudflare.APIToken, host, bestIP)
			if err != nil {
				log.Printf("Error updating DNS for %s: %v", host, err)
			}
		} else {
			log.Printf("No best IP found for group %s (host: %s), skipping DNS update.", hostInfo.Group, host)
		}
	}
	log.Println("Finished latency tests and DNS updates.")
}

// findBestIPForGroup performs latency tests for a given group and returns the best IP.
func findBestIPForGroup(groupName string) (string, time.Duration) {
	ips, err := database.GetLatestIPsByGroup(groupName, 5)
	if err != nil {
		log.Printf("Error getting IPs for group %s: %v", groupName, err)
		return "", 0
	}
	if len(ips) == 0 {
		log.Printf("No IPs found for group %s", groupName)
		return "", 0
	}

	log.Printf("Testing %d IPs for group %s...", len(ips), groupName)
	var bestIP string
	var minAvgLatency time.Duration

	for _, ip := range ips {
		var totalLatency time.Duration
		var successfulTests int
		for i := 0; i < 5; i++ {
			lat, err := latency.Measure(ip)
			if err != nil {
				log.Printf("Test %d/%d for IP %s failed: %v", i+1, 5, ip, err)
			} else {
				totalLatency += lat
				successfulTests++
				log.Printf("Test %d/%d for IP %s (group: %s): latency=%v", i+1, 5, ip, groupName, lat)
			}
			time.Sleep(1 * time.Second)
		}

		if successfulTests > 0 {
			avgLatency := totalLatency / time.Duration(successfulTests)
			log.Printf("Average latency for IP %s: %v", ip, avgLatency)
			if minAvgLatency == 0 || avgLatency < minAvgLatency {
				minAvgLatency = avgLatency
				bestIP = ip
			}
		}
	}

	if bestIP != "" {
		log.Printf("Best IP for group %s is %s with latency %v", groupName, bestIP, minAvgLatency)
	}
	return bestIP, minAvgLatency
}
