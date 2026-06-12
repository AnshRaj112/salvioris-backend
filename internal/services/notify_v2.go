package services

import (
	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

func NotifyUser(userID uuid.UUID, role, title, message, notifType string) {
	if userID == uuid.Nil {
		return
	}
	_, _ = database.PostgresDB.Exec(`
		INSERT INTO notifications (recipient_id, recipient_role, title, message, type, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, FALSE, NOW())
	`, userID, role, title, message, notifType)
}

func NotifyPatientByID(patientID uuid.UUID, title, message, notifType string) {
	var userID uuid.UUID
	err := database.PostgresDB.QueryRow(
		`SELECT user_id FROM patients WHERE id = $1 AND user_id IS NOT NULL`, patientID,
	).Scan(&userID)
	if err != nil {
		return
	}
	NotifyUser(userID, "user", title, message, notifType)
}
