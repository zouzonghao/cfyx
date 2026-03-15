package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

var (
	DB *sql.DB
)

// InitDB initializes the SQLite database connection.
func InitDB(filepath string) {
	var err error
	DB, err = sql.Open("sqlite", filepath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	createTableSQL := `CREATE TABLE IF NOT EXISTS ip_info (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"ip" TEXT NOT NULL,
		"add_time" DATETIME NOT NULL,
		"group_name" TEXT NOT NULL
	);`

	_, err = DB.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	log.Println("Database initialized and table created successfully.")
}

// InsertIP inserts a new IP record into the database.
func InsertIP(ip string, groupName string) error {
	insertSQL := `INSERT INTO ip_info(ip, add_time, group_name) VALUES (?, ?, ?)`
	statement, err := DB.Prepare(insertSQL)
	if err != nil {
		return err
	}
	_, err = statement.Exec(ip, time.Now(), groupName)
	return err
}

// GetLatestIPsByGroup retrieves the most recently added IPs for a specific group, up to a given limit.
func GetLatestIPsByGroup(groupName string, limit int) ([]string, error) {
	var ips []string
	querySQL := `SELECT ip FROM ip_info WHERE group_name = ? ORDER BY add_time DESC LIMIT ?`
	rows, err := DB.Query(querySQL, groupName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		ips = append(ips, ip)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return ips, nil
}

// FilterExistingIPs takes a slice of IPs and returns only those that do not already exist in the database.
func FilterExistingIPs(ips []string) ([]string, error) {
	if len(ips) == 0 {
		return []string{}, nil
	}

	placeholders := strings.Repeat("?,", len(ips)-1) + "?"
	querySQL := fmt.Sprintf(`SELECT ip FROM ip_info WHERE ip IN (%s)`, placeholders)

	args := make([]interface{}, len(ips))
	for i, v := range ips {
		args[i] = v
	}

	rows, err := DB.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	existingIPs := make(map[string]struct{})
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		existingIPs[ip] = struct{}{}
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	var newIPs []string
	for _, ip := range ips {
		if _, found := existingIPs[ip]; !found {
			newIPs = append(newIPs, ip)
		}
	}

	return newIPs, nil
}
