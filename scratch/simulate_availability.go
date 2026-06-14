package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func main() {
	connStr := "postgresql://salvioris_main:7Xj0ffHrfkAE8t7YqYEMgpPSm9dssivw@dpg-d641vkkr85hc73biad1g-a.oregon-postgres.render.com/salvioris"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	therapistID := uuid.MustParse("e73d8d42-8246-4e4f-8ece-c415a954d9d9")
	var tenantID uuid.UUID
	err = db.QueryRow("SELECT id FROM tenants WHERE therapist_id = $1", therapistID).Scan(&tenantID)
	if err != nil {
		log.Fatalf("failed to get tenant: %v", err)
	}

	dateStr := "2026-06-16"
	day, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		log.Fatalf("failed to parse date: %v", err)
	}

	fmt.Printf("Weekday for %s: %d (%s)\n", dateStr, int(day.Weekday()), day.Weekday())

	// 1. Fetch availability slots
	rows, err := db.Query(`
		SELECT start_time::text, end_time::text, slot_duration_min
		FROM availability_slots
		WHERE tenant_id = $1 AND therapist_id = $2 AND day_of_week = $3 AND is_active = TRUE
	`, tenantID, therapistID, int(day.Weekday()))
	if err != nil {
		log.Fatalf("failed to get slots: %v", err)
	}
	defer rows.Close()

	type slot struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	var slots []slot
	for rows.Next() {
		var start, end string
		var dur int
		_ = rows.Scan(&start, &end, &dur)
		slots = append(slots, slot{Start: start, End: end})
	}
	fmt.Printf("Fetched %d slots: %v\n", len(slots), slots)

	dayStart := day
	dayEnd := day.Add(24 * time.Hour).Add(-1 * time.Second)

	// 2. Fetch busy ranges from database
	bookingRows, err := db.Query(`
		SELECT starts_at, ends_at FROM appointments
		WHERE therapist_id = $1 AND status NOT IN ('cancelled', 'no_show', 'pending_payment')
		AND starts_at >= $2 AND starts_at <= $3
	`, therapistID, dayStart, dayEnd)
	if err != nil {
		log.Fatalf("failed to query appointments: %v", err)
	}
	defer bookingRows.Close()

	type busyRange struct {
		Start time.Time
		End   time.Time
	}
	var busyRanges []busyRange
	for bookingRows.Next() {
		var bs, be time.Time
		if err := bookingRows.Scan(&bs, &be); err == nil {
			busyRanges = append(busyRanges, busyRange{Start: bs, End: be})
		}
	}
	fmt.Printf("Fetched %d busy ranges from database: %v\n", len(busyRanges), busyRanges)

	// 3. Check slots overlap
	freeSlots := make([]slot, 0)
	for _, s := range slots {
		sStartStr := dateStr + "T" + s.Start
		if len(s.Start) == 5 {
			sStartStr += ":00"
		}
		sEndStr := dateStr + "T" + s.End
		if len(s.End) == 5 {
			sEndStr += ":00"
		}
		
		slotStart, err1 := time.Parse("2006-01-02T15:04:05", sStartStr)
		slotEnd, err2 := time.Parse("2006-01-02T15:04:05", sEndStr)
		fmt.Printf("Slot parsed Start: %v (Err: %v), End: %v (Err: %v)\n", slotStart, err1, slotEnd, err2)

		overlap := false
		for _, busy := range busyRanges {
			if slotStart.Before(busy.End) && slotEnd.After(busy.Start) {
				overlap = true
				break
			}
		}
		if !overlap {
			freeSlots = append(freeSlots, s)
		}
	}

	fmt.Printf("Free slots: %v\n", freeSlots)
}
