package services

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"github.com/google/uuid"
)

func EnsureTenantForTherapist(therapistID uuid.UUID) (uuid.UUID, error) {
	var tenantID uuid.UUID
	err := database.PostgresDB.QueryRow(
		`SELECT id FROM tenants WHERE therapist_id = $1`, therapistID,
	).Scan(&tenantID)
	if err == nil {
		return tenantID, nil
	}
	if err != sql.ErrNoRows {
		return uuid.Nil, err
	}

	var displayName string
	err = database.PostgresDB.QueryRow(
		`SELECT name FROM therapists WHERE id = $1`, therapistID,
	).Scan(&displayName)
	if err != nil {
		return uuid.Nil, err
	}

	err = database.PostgresDB.QueryRow(`
		INSERT INTO tenants (therapist_id, display_name)
		VALUES ($1, $2)
		RETURNING id
	`, therapistID, displayName).Scan(&tenantID)
	return tenantID, err
}

func TherapistOwnsTenant(therapistID, tenantID uuid.UUID) (bool, error) {
	var exists bool
	err := database.PostgresDB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM tenants
			WHERE id = $1 AND therapist_id = $2 AND is_active = TRUE
		)
	`, tenantID, therapistID).Scan(&exists)
	return exists, err
}

func GetTenantIDForTherapist(therapistID uuid.UUID) (uuid.UUID, error) {
	id, err := EnsureTenantForTherapist(therapistID)
	if err != nil {
		return uuid.Nil, err
	}
	if id == uuid.Nil {
		return uuid.Nil, errors.New("tenant not found")
	}
	return id, nil
}

func EnsurePatientProfileForUser(userID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	var patientID, tenantID uuid.UUID
	err := database.PostgresDB.QueryRow(`
		SELECT id, tenant_id FROM patients
		WHERE user_id = $1 AND deleted_at IS NULL AND status = 'active'
		ORDER BY created_at DESC LIMIT 1
	`, userID).Scan(&patientID, &tenantID)
	if err == nil {
		return patientID, tenantID, nil
	}
	if err != sql.ErrNoRows {
		return uuid.Nil, uuid.Nil, err
	}

	var therapistID uuid.UUID
	var fullName, email sql.NullString
	err = database.PostgresDB.QueryRow(`
		SELECT tuc.therapist_id,
			COALESCE(po.patient_name, u.username) AS full_name,
			po.patient_email
		FROM therapist_user_connections tuc
		JOIN users u ON u.id = tuc.user_id
		LEFT JOIN patient_onboardings po ON po.user_id = tuc.user_id AND po.therapist_id = tuc.therapist_id
		WHERE tuc.user_id = $1
		ORDER BY po.onboarded_at DESC NULLS LAST, tuc.connected_at DESC
		LIMIT 1
	`, userID).Scan(&therapistID, &fullName, &email)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	tenantID, err = EnsureTenantForTherapist(therapistID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	name := strings.TrimSpace(fullName.String)
	if name == "" {
		name = "Patient"
	}

	err = database.PostgresDB.QueryRow(`
		INSERT INTO patients (tenant_id, user_id, full_name, email, assigned_therapist_id, status)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, 'active')
		RETURNING id
	`, tenantID, userID, name, strings.ToLower(strings.TrimSpace(email.String)), therapistID).Scan(&patientID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	return patientID, tenantID, nil
}

func SyncConnectedUsersToPatients(tenantID, therapistID uuid.UUID) error {
	rows, err := database.PostgresDB.Query(`
		SELECT c.user_id, u.username
		FROM therapist_user_connections c
		JOIN users u ON c.user_id = u.id
		WHERE c.therapist_id = $1
	`, therapistID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var uID uuid.UUID
		var username string
		if err := rows.Scan(&uID, &username); err == nil {
			var exists bool
			_ = database.PostgresDB.QueryRow(`
				SELECT EXISTS(
					SELECT 1 FROM patients WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL
				)
			`, tenantID, uID).Scan(&exists)
			if !exists {
				var emailEncrypted sql.NullString
				_ = database.PostgresDB.QueryRow(`
					SELECT email_encrypted FROM user_recovery WHERE user_id = $1
				`, uID).Scan(&emailEncrypted)
				
				emailStr := ""
				if emailEncrypted.Valid && emailEncrypted.String != "" {
					if decrypted, err := utils.Decrypt(emailEncrypted.String); err == nil {
						emailStr = decrypted
					}
				}

				_, _ = database.PostgresDB.Exec(`
					INSERT INTO patients (tenant_id, user_id, full_name, email, assigned_therapist_id, status)
					VALUES ($1, $2, $3, NULLIF($4, ''), $5, 'active')
				`, tenantID, uID, username, strings.ToLower(strings.TrimSpace(emailStr)), therapistID)
			}
		}
	}
	return nil
}
