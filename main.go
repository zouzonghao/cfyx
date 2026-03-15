package main

import (
	"cf-optimizer/config"
	"cf-optimizer/database"
	"cf-optimizer/modes"
	"flag"
	"log"
	"net/http"
)

func main() {
	fullMode := flag.Bool("full", false, "Enable full mode to fetch from all providers.")
	flag.Parse()

	config.LoadConfig("config.yaml")
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
