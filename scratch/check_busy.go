package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	connStr := "postgresql://salvioris_main:7Xj0ffHrfkAE8t7YqYEMgpPSm9dssivw@dpg-d641vkkr85hc73biad1g-a.oregon-postgres.render.com/salvioris"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// 1. Get all appointments for 2026-06-16
	rows, err := db.Query("SELECT id, therapist_id, starts_at, ends_at, status FROM appointments WHERE starts_at >= '2026-06-16 00:00:00' AND starts_at <= '2026-06-16 23:59:59'")
	if err != nil {
		log.Fatalf("query appointments failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("Appointments on 2026-06-16:")
	for rows.Next() {
		var id, therapistID, status string
		var startsAt, endsAt time.Time
		err = rows.Scan(&id, &therapistID, &startsAt, &endsAt, &status)
		if err != nil {
			log.Fatalf("scan failed: %v", err)
		}
		fmt.Printf("Apt ID: %s, Therapist: %s, Start: %s, End: %s, Status: %s\n", id, therapistID, startsAt, endsAt, status)
	}

	// 2. Check if Google Calendar sync is enabled for the therapist
	var syncEnabled bool
	var therapistID, tenantID string
	err = db.QueryRow("SELECT tenant_id, therapist_id, sync_enabled FROM calendar_integrations LIMIT 1").Scan(&tenantID, &therapistID, &syncEnabled)
	if err == nil {
		fmt.Printf("Calendar Integration: Tenant: %s, Therapist: %s, Sync Enabled: %t\n", tenantID, therapistID, syncEnabled)
	} else {
		fmt.Println("No Calendar Integrations found in DB:", err)
	}
}
