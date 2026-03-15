package modes

import (
	"cf-optimizer/config"
	"cf-optimizer/database"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

var (
	IsFullMode bool
)

// GetHostsHandler handles requests to the /gethosts endpoint.
// It behaves differently based on whether the application is in full or minimal mode.
func GetHostsHandler(w http.ResponseWriter, r *http.Request) {
	if IsFullMode {
		getHostsFullMode(w, r)
	} else {
		getHostsMinimalMode(w, r)
	}
}

// getHostsMinimalMode serves hosts by directly fetching the latest IP from the database without latency testing.
func getHostsMinimalMode(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling /gethosts request in minimal mode.")

	var responseBody strings.Builder
	for host, hostInfo := range config.Current.HostMap {
		// For each host, get the single latest IP for its group.
		ips, err := database.GetLatestIPsByGroup(hostInfo.Group, 1)
		if err != nil {
			log.Printf("Error getting latest IP for group %s: %v", hostInfo.Group, err)
			continue
		}

		if len(ips) > 0 {
			fmt.Fprintf(&responseBody, "%s  %s\n", ips[0], host)
		} else {
			log.Printf("No IP found in database for group %s (host: %s)", hostInfo.Group, host)
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, responseBody.String())
}

// getHostsFullMode serves hosts by performing latency tests to find the best IP.
func getHostsFullMode(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling /gethosts request in full mode.")

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

	var responseBody strings.Builder
	for host, hostInfo := range config.Current.HostMap {
		if ip, ok := bestIPsByGroup[hostInfo.Group]; ok {
			fmt.Fprintf(&responseBody, "%s  %s\n", ip, host)
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, responseBody.String())
}
