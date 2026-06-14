package services

import (
	"fmt"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

var validAptTypes = map[string]bool{
	"online": true, "in_person": true, "walk_in": true, "emergency": true,
	"chat": true, "voice": true, "video": true,
}

func ValidateAppointmentType(t string) bool {
	return validAptTypes[t]
}

func TherapistHasConflict(therapistID uuid.UUID, startsAt, endsAt time.Time, excludeID *uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM appointments
			WHERE therapist_id = $1 AND status NOT IN ('cancelled', 'no_show', 'pending_payment')
			AND starts_at < $3 AND ends_at > $2
	`
	args := []interface{}{therapistID, startsAt, endsAt}
	if excludeID != nil {
		query += ` AND id != $4`
		args = append(args, *excludeID)
	}
	query += `)`
	var exists bool
	err := database.PostgresDB.QueryRow(query, args...).Scan(&exists)
	return exists, err
}

func TherapistInTenant(tenantID, therapistID uuid.UUID) bool {
	var ok bool
	_ = database.PostgresDB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1 AND therapist_id = $2)
	`, tenantID, therapistID).Scan(&ok)
	return ok
}

func ParseRFC3339(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func DefaultDuration(min int) time.Duration {
	if min <= 0 {
		min = 60
	}
	return time.Duration(min) * time.Minute
}

func FormatTimeOnly(t time.Time) string {
	return t.Format("15:04:05")
}

func ParseTimeOnly(s string) (time.Time, error) {
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time: %s", s)
}
