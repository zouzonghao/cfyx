package tracer

import (
	"bytes"
	"cf-optimizer/config"
	"context"
	"encoding/json"
	"log"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// NextTraceOutput defines the structure for the JSON output from nexttrace.
type NextTraceOutput struct {
	Hops [][]Hop `json:"Hops"`
}

// Hop represents a single hop in the traceroute.
type Hop struct {
	Success bool    `json:"Success"`
	Geo     GeoInfo `json:"Geo"`
}

// GeoInfo contains the geographical information for a hop.
type GeoInfo struct {
	Country string `json:"country"`
	Prov    string `json:"prov"`
}

// GetIPGroup analyzes an IP using nexttrace and returns its group based on ordered routing path.
func GetIPGroup(ip string) string {
	log.Printf("Analyzing IP: %s", ip)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./nexttrace", "-j", "-f", "1", ip)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("nexttrace timed out for IP %s", ip)
			return "TIMEOUT"
		}
		log.Printf("Error running nexttrace for IP %s: %v\nStderr: %s", ip, err, stderr.String())
		return "UNKNOWN_ERROR"
	}

	jsonStartIndex := strings.Index(out.String(), "{")
	if jsonStartIndex == -1 {
		log.Printf("No JSON object found in nexttrace output for IP %s", ip)
		return "NO_JSON"
	}
	jsonString := out.String()[jsonStartIndex:]

	var traceOutput NextTraceOutput
	err = json.Unmarshal([]byte(jsonString), &traceOutput)
	if err != nil {
		log.Printf("Error parsing nexttrace JSON for IP %s: %v", ip, err)
		return "JSON_PARSE_ERROR"
	}

	var path []string
	var lastLocation string
	locationsSet := make(map[string]struct{})

	for _, hopGroup := range traceOutput.Hops {
		var locationForThisTTL string
		for _, hop := range hopGroup {
			if hop.Success {
				var currentLocation string
				if hop.Geo.Prov != "" {
					p := hop.Geo.Prov
					p = strings.TrimSuffix(p, "省")
					p = strings.TrimSuffix(p, "市")
					p = strings.TrimSuffix(p, "自治区")
					p = strings.TrimSuffix(p, "特别行政区")
					currentLocation = p
				} else if hop.Geo.Country != "" && hop.Geo.Country != "Anycast" {
					currentLocation = hop.Geo.Country
				}

				if currentLocation != "" {
					locationForThisTTL = currentLocation
					break
				}
			}
		}

		if locationForThisTTL != "" && locationForThisTTL != lastLocation {
			path = append(path, locationForThisTTL)
			locationsSet[locationForThisTTL] = struct{}{}
			lastLocation = locationForThisTTL
		}
	}

	groupNames := make([]string, 0, len(config.Current.GroupRules))
	for groupName := range config.Current.GroupRules {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)

	for _, groupName := range groupNames {
		andConditions := config.Current.GroupRules[groupName]
		match := true
		for _, orConditions := range andConditions {
			orMatch := false
			for _, location := range orConditions {
				if _, found := locationsSet[location]; found {
					orMatch = true
					break
				}
			}
			if !orMatch {
				match = false
				break
			}
		}
		if match {
			return groupName
		}
	}

	if len(path) == 0 {
		return "UNKNOWN"
	}

	return strings.Join(path, "_")
}
