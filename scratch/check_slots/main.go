package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	connStr := "postgresql://salvioris_main:7Xj0ffHrfkAE8t7YqYEMgpPSm9dssivw@dpg-d641vkkr85hc73biad1g-a.oregon-postgres.render.com/salvioris"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, tenant_id, therapist_id, day_of_week, start_time::text, end_time::text, slot_duration_min, is_active FROM availability_slots")
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("Availability Slots:")
	for rows.Next() {
		var id, tenantID, therapistID, startTime, endTime string
		var dayOfWeek, duration int
		var isActive bool
		err = rows.Scan(&id, &tenantID, &therapistID, &dayOfWeek, &startTime, &endTime, &duration, &isActive)
		if err != nil {
			log.Fatalf("scan failed: %v", err)
		}
		fmt.Printf("Slot ID: %s, Day: %d, Time: %s - %s, Active: %t\n", id, dayOfWeek, startTime, endTime, isActive)
	}
}
