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

	rows, err := db.Query("SELECT id, name FROM therapists")
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("Therapists:")
	for rows.Next() {
		var id, name string
		err = rows.Scan(&id, &name)
		if err != nil {
			log.Fatalf("scan failed: %v", err)
		}
		fmt.Printf("Therapist ID: %s, Name: %s\n", id, name)
	}

	// Also print the availability slots with therapist_id
	rows2, err := db.Query("SELECT id, therapist_id, day_of_week FROM availability_slots")
	if err != nil {
		log.Fatalf("query slots failed: %v", err)
	}
	defer rows2.Close()

	fmt.Println("\nAvailability Slots with Therapist ID:")
	for rows2.Next() {
		var id, therapistID string
		var day int
		err = rows2.Scan(&id, &therapistID, &day)
		if err != nil {
			log.Fatalf("scan failed: %v", err)
		}
		fmt.Printf("Slot ID: %s, Therapist ID: %s, Day: %d\n", id, therapistID, day)
	}
}
