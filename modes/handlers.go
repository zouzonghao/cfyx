package modes

import (
	"cf-optimizer/config"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// GetHostsHandler handles requests to the /gethosts endpoint using the
// verified IPs selected by the most recent processing run.
func GetHostsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling /gethosts request.")

	bestIPsMu.RLock()
	selectedIPs := make(map[string]string, len(bestIPsByGroup))
	for group, ip := range bestIPsByGroup {
		selectedIPs[group] = ip
	}
	bestIPsMu.RUnlock()

	var responseBody strings.Builder
	for host, hostInfo := range config.Current.HostMap {
		if ip, ok := selectedIPs[hostInfo.Group]; ok {
			fmt.Fprintf(&responseBody, "%s  %s\n", ip, host)
		} else {
			log.Printf("No selected IP for group %s (host: %s)", hostInfo.Group, host)
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, responseBody.String())
}
