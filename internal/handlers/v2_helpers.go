package handlers

import (
	"context"
	"database/sql"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

func mongoCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 8*time.Second)
}

func patientBelongsToTenant(tenantID, patientID uuid.UUID) bool {
	var exists bool
	_ = database.PostgresDB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM patients
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`, patientID, tenantID).Scan(&exists)
	return exists
}

func parsePatientIDParam(s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	return id, err == nil
}

func loadPatientSnapshot(patientID uuid.UUID) map[string]string {
	var name string
	var dob sql.NullTime
	var gender sql.NullString
	_ = database.PostgresDB.QueryRow(`
		SELECT full_name, date_of_birth, gender FROM patients WHERE id = $1
	`, patientID).Scan(&name, &dob, &gender)
	snap := map[string]string{"name": name}
	if dob.Valid {
		snap["dob"] = dob.Time.Format("2006-01-02")
	}
	if gender.Valid {
		snap["gender"] = gender.String
	}
	return snap
}

func loadTherapistSnapshot(therapistID uuid.UUID) map[string]string {
	var name, license sql.NullString
	_ = database.PostgresDB.QueryRow(`
		SELECT name, license_number FROM therapists WHERE id = $1
	`, therapistID).Scan(&name, &license)
	snap := map[string]string{}
	if name.Valid {
		snap["name"] = name.String
	}
	if license.Valid {
		snap["license"] = license.String
	}
	return snap
}
