package main

import (
	"cf-optimizer/config"
	"cf-optimizer/database"
	"cf-optimizer/modes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	nexttraceAPiURL   = "https://api.github.com/repos/nxtrace/NTrace-core/releases/latest"
	nexttraceFallback = "v1.5.0"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func getNexttraceFilename() string {
	arch := runtime.GOARCH
	os := runtime.GOOS
	switch {
	case os == "linux" && arch == "amd64":
		return "nexttrace_linux_amd64"
	case os == "linux" && arch == "arm64":
		return "nexttrace_linux_arm64"
	case os == "darwin" && arch == "arm64":
		return "nexttrace_darwin_arm64"
	case os == "darwin" && arch == "amd64":
		return "nexttrace_darwin_amd64"
	case os == "darwin":
		return "nexttrace_darwin_universal"
	case os == "windows" && arch == "amd64":
		return "nexttrace_windows_amd64.exe"
	default:
		return ""
	}
}

func fetchLatestVersion() (string, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(nexttraceAPiURL)
	if err != nil {
		return nexttraceFallback, "", fmt.Errorf("failed to fetch latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nexttraceFallback, "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nexttraceFallback, "", fmt.Errorf("failed to read response: %w", err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nexttraceFallback, "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	filename := getNexttraceFilename()
	if filename == "" {
		return release.TagName, "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	for _, asset := range release.Assets {
		if asset.Name == filename {
			return release.TagName, asset.BrowserDownloadURL, nil
		}
	}

	return release.TagName, "", fmt.Errorf("asset not found: %s", filename)
}

func checkDependencies() {
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		log.Fatal("Error: config.yaml not found. Please create config.yaml before starting the service.")
	}

	if _, err := os.Stat("./nexttrace"); os.IsNotExist(err) {
		log.Println("Attempting to fetch latest nexttrace version from GitHub...")
		_, downloadURL, err := fetchLatestVersion()
		if err != nil {
			filename := getNexttraceFilename()
			if filename == "" {
				log.Fatalf(`Error: nexttrace binary not found at ./nexttrace and your platform is unsupported.

Please visit https://github.com/nxtrace/NTrace-core/releases to find a suitable binary.`)
			}
			log.Printf("Warning: Could not fetch latest version (%v), using fallback version %s", err, nexttraceFallback)
			downloadURL = fmt.Sprintf("https://github.com/nxtrace/NTrace-core/releases/download/%s/%s", nexttraceFallback, filename)
		}

		log.Fatalf(`Error: nexttrace binary not found at ./nexttrace. 

Please download nexttrace from: 
  %s

Or visit: https://github.com/nxtrace/NTrace-core/releases

After downloading, rename the file to "nexttrace" and make it executable:
  mv %s nexttrace
  chmod +x nexttrace`, downloadURL, getNexttraceFilename())
	}

	if err := exec.Command("./nexttrace", "--version").Run(); err != nil {
		log.Fatal("Error: nexttrace binary is not executable or not a valid nexttrace binary.")
	}

	log.Println("Dependency check passed: config.yaml and nexttrace are available.")
}

func main() {
	checkDependencies()

	fullMode := flag.Bool("full", false, "Enable full mode to fetch from all providers.")
	flag.Parse()

	configPath, _ := filepath.Abs("config.yaml")
	config.LoadConfig(configPath)
	database.InitDB("./ip_data.db")
	defer database.DB.Close()

	// Use flag value if provided, otherwise use config file value
	useFullMode := *fullMode || config.Current.FullMode

	// Pass the mode flag to the modes package so handlers can access it.
	modes.IsFullMode = useFullMode
	http.HandleFunc("/gethosts", modes.GetHostsHandler)

	if useFullMode {
		modes.RunFullMode()
	} else {
		modes.RunMinimalMode()
	}

	log.Println("Server starting on :37377...")
	log.Println("Access http://localhost:37377/gethosts to get the hosts file.")

	go func() {
		if err := http.ListenAndServe(":37377", nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Println("Service started. Running in background...")
	select {}
}
