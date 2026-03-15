package main

import (
	"database/sql"
	"encoding/csv"
	"log"
	"os"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./ip_data.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, ip, add_time, group_name FROM ip_info ORDER BY id")
	if err != nil {
		log.Fatalf("Failed to query database: %v", err)
	}
	defer rows.Close()

	file, err := os.Create("ip_info.csv")
	if err != nil {
		log.Fatalf("Failed to create csv file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"id", "ip", "add_time", "group_name"})

	for rows.Next() {
		var id int
		var ip, addTime, groupName string
		if err := rows.Scan(&id, &ip, &addTime, &groupName); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		record := []string{
			strconv.Itoa(id),
			ip,
			addTime,
			groupName,
		}
		if err := writer.Write(record); err != nil {
			log.Printf("Failed to write record to csv: %v", err)
		}
	}

	if err = rows.Err(); err != nil {
		log.Fatalf("Error during rows iteration: %v", err)
	}

	log.Println("Successfully exported data to ip_info.csv")
}
