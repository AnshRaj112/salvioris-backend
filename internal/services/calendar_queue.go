package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

const calendarQueueKey = "queue:calendar:sync"

type CalendarJob struct {
	Action        string `json:"action"` // create | update | delete
	AppointmentID string `json:"appointment_id"`
	TenantID      string `json:"tenant_id"`
}

func EnqueueCalendarSync(action string, tenantID, appointmentID uuid.UUID) {
	if database.RedisClient == nil {
		go processCalendarJob(CalendarJob{
			Action: action, AppointmentID: appointmentID.String(), TenantID: tenantID.String(),
		})
		return
	}
	job := CalendarJob{
		Action: action, AppointmentID: appointmentID.String(), TenantID: tenantID.String(),
	}
	data, _ := json.Marshal(job)
	ctx := context.Background()
	if err := database.RedisClient.LPush(ctx, calendarQueueKey, data).Err(); err != nil {
		log.Printf("calendar queue push failed: %v", err)
		go processCalendarJob(job)
	}
}

func StartCalendarWorker() {
	if database.RedisClient == nil {
		log.Println("⚠️  Calendar worker: Redis unavailable, inline sync only")
		return
	}
	go func() {
		ctx := context.Background()
		for {
			result, err := database.RedisClient.BRPop(ctx, 5*time.Second, calendarQueueKey).Result()
			if err != nil || len(result) < 2 {
				continue
			}
			var job CalendarJob
			if json.Unmarshal([]byte(result[1]), &job) == nil {
				processCalendarJob(job)
			}
		}
	}()
	log.Println("✅ Calendar sync worker started")
}

func processCalendarJob(job CalendarJob) {
	if err := SyncAppointmentToGoogle(job); err != nil {
		log.Printf("calendar sync %s %s: %v", job.Action, job.AppointmentID, err)
	}
}
