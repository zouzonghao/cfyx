package modes

import (
	"cf-optimizer/config"
	"cf-optimizer/providers"
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
	if config.IsManualMode() {
		log.Println("Full mode: Manual mode is enabled, skipping automatic optimization.")
		return
	}

	if !startFullRun() {
		log.Println("Full mode: previous run still in progress, skipping.")
		return
	}
	defer finishFullRun()

	log.Println("Full mode: Starting to fetch and process IPs...")

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
				log.Printf("Full mode: Error fetching from %T: %v", provider, err)
				return
			}
			log.Printf("Full mode: Fetched %d IPs from %T", len(ips), provider)
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
		log.Println("Full mode: No IPs found from any provider.")
		return
	}

	log.Printf("Full mode: Found %d unique IPs from all providers.", len(uniqueIPs))
	processIPs(uniqueIPs, "Full mode")
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
