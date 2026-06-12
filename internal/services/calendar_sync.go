package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/config"
	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var googleOAuthConfig *oauth2.Config

func InitGoogleCalendar(cfg *config.Config) {
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		return
	}
	googleOAuthConfig = &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURI,
		Scopes:       []string{calendar.CalendarEventsScope},
		Endpoint:     google.Endpoint,
	}
}

func GoogleCalendarEnabled() bool {
	return googleOAuthConfig != nil
}

func GoogleAuthURL(tenantID, therapistID uuid.UUID) (string, error) {
	if !GoogleCalendarEnabled() {
		return "", fmt.Errorf("google calendar not configured")
	}
	state := fmt.Sprintf("%s:%s", tenantID.String(), therapistID.String())
	return googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce), nil
}

func HandleGoogleCallback(code, state string) error {
	if !GoogleCalendarEnabled() {
		return fmt.Errorf("google calendar not configured")
	}
	parts := strings.SplitN(state, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid oauth state")
	}
	tenantID, err := uuid.Parse(parts[0])
	if err != nil {
		return err
	}
	therapistID, err := uuid.Parse(parts[1])
	if err != nil {
		return err
	}

	ctx := context.Background()
	tok, err := googleOAuthConfig.Exchange(ctx, code)
	if err != nil {
		return err
	}

	accessEnc, _ := utils.Encrypt(tok.AccessToken)
	refreshEnc, _ := utils.Encrypt(tok.RefreshToken)

	_, err = database.PostgresDB.Exec(`
		INSERT INTO calendar_integrations (tenant_id, therapist_id, access_token_enc, refresh_token_enc, token_expires_at, sync_enabled)
		VALUES ($1, $2, $3, $4, $5, TRUE)
		ON CONFLICT (tenant_id, therapist_id) DO UPDATE SET
			access_token_enc = EXCLUDED.access_token_enc,
			refresh_token_enc = EXCLUDED.refresh_token_enc,
			token_expires_at = EXCLUDED.token_expires_at,
			sync_enabled = TRUE,
			updated_at = NOW()
	`, tenantID, therapistID, accessEnc, refreshEnc, tok.Expiry)
	return err
}

func SyncAppointmentToGoogle(job CalendarJob) error {
	if !GoogleCalendarEnabled() {
		return nil
	}
	appointmentID, err := uuid.Parse(job.AppointmentID)
	if err != nil {
		return err
	}
	tenantID, err := uuid.Parse(job.TenantID)
	if err != nil {
		return err
	}

	lockKey := "cal:sync:lock:" + appointmentID.String()
	if database.RedisClient != nil {
		ok, _ := database.RedisClient.SetNX(context.Background(), lockKey, "1", 60*time.Second).Result()
		if !ok {
			return nil
		}
		defer database.RedisClient.Del(context.Background(), lockKey)
	}

	var therapistID uuid.UUID
	var patientName, aptType, status, meetingLink, location, notes sql.NullString
	var startsAt, endsAt time.Time
	var cancelledAt sql.NullTime

	err = database.PostgresDB.QueryRow(`
		SELECT a.therapist_id, p.full_name, a.type, a.status, a.starts_at, a.ends_at,
			a.meeting_link, a.location, a.notes, a.cancelled_at
		FROM appointments a
		JOIN patients p ON p.id = a.patient_id
		WHERE a.id = $1 AND a.tenant_id = $2
	`, appointmentID, tenantID).Scan(
		&therapistID, &patientName, &aptType, &status, &startsAt, &endsAt,
		&meetingLink, &location, &notes, &cancelledAt,
	)
	if err != nil {
		return err
	}

	svc, integrationID, err := calendarService(tenantID, therapistID)
	if err != nil {
		return err
	}
	if svc == nil {
		return nil
	}

	var externalID string
	_ = database.PostgresDB.QueryRow(`
		SELECT external_event_id FROM calendar_event_mappings
		WHERE appointment_id = $1 AND integration_id = $2
	`, appointmentID, integrationID).Scan(&externalID)

	action := job.Action
	if status.String == "cancelled" {
		action = "delete"
	}

	switch action {
	case "delete":
		if externalID == "" {
			return nil
		}
		if err := svc.Events.Delete("primary", externalID).Do(); err != nil {
			markSyncFailed(appointmentID, integrationID)
			return err
		}
		_, _ = database.PostgresDB.Exec(`DELETE FROM calendar_event_mappings WHERE appointment_id = $1 AND integration_id = $2`,
			appointmentID, integrationID)
		return nil
	default:
		event := &calendar.Event{
			Summary:     fmt.Sprintf("Session: %s", patientName.String),
			Description: fmt.Sprintf("Type: %s\n%s", aptType.String, notes.String),
			Start:       &calendar.EventDateTime{DateTime: startsAt.Format(time.RFC3339), TimeZone: "UTC"},
			End:         &calendar.EventDateTime{DateTime: endsAt.Format(time.RFC3339), TimeZone: "UTC"},
			Location:    location.String,
		}
		if meetingLink.Valid && meetingLink.String != "" {
			event.Description += "\nMeeting: " + meetingLink.String
		}

		if externalID != "" {
			_, err = svc.Events.Update("primary", externalID, event).Do()
		} else {
			var created *calendar.Event
			created, err = svc.Events.Insert("primary", event).Do()
			if err == nil {
				externalID = created.Id
				_, _ = database.PostgresDB.Exec(`
					INSERT INTO calendar_event_mappings (tenant_id, appointment_id, integration_id, external_event_id, sync_status, last_synced_at)
					VALUES ($1, $2, $3, $4, 'synced', NOW())
					ON CONFLICT (appointment_id, integration_id) DO UPDATE SET
						external_event_id = EXCLUDED.external_event_id,
						sync_status = 'synced', last_synced_at = NOW()
				`, tenantID, appointmentID, integrationID, externalID)
			}
		}
		if err != nil {
			markSyncFailed(appointmentID, integrationID)
			return err
		}
		_, _ = database.PostgresDB.Exec(`
			UPDATE calendar_event_mappings SET sync_status = 'synced', last_synced_at = NOW()
			WHERE appointment_id = $1 AND integration_id = $2
		`, appointmentID, integrationID)
		return nil
	}
}

func calendarService(tenantID, therapistID uuid.UUID) (*calendar.Service, uuid.UUID, error) {
	var integrationID uuid.UUID
	var accessEnc, refreshEnc string
	var expiresAt sql.NullTime
	var syncEnabled bool

	err := database.PostgresDB.QueryRow(`
		SELECT id, access_token_enc, refresh_token_enc, token_expires_at, sync_enabled
		FROM calendar_integrations
		WHERE tenant_id = $1 AND therapist_id = $2
	`, tenantID, therapistID).Scan(&integrationID, &accessEnc, &refreshEnc, &expiresAt, &syncEnabled)
	if err == sql.ErrNoRows || !syncEnabled {
		return nil, uuid.Nil, nil
	}
	if err != nil {
		return nil, uuid.Nil, err
	}

	access, _ := utils.Decrypt(accessEnc)
	refresh, _ := utils.Decrypt(refreshEnc)
	tok := &oauth2.Token{AccessToken: access, RefreshToken: refresh}
	if expiresAt.Valid {
		tok.Expiry = expiresAt.Time
	}

	ctx := context.Background()
	ts := googleOAuthConfig.TokenSource(ctx, tok)
	client := oauth2.NewClient(ctx, ts)
	newTok, err := ts.Token()
	if err != nil {
		return nil, integrationID, err
	}
	if newTok.AccessToken != access {
		enc, _ := utils.Encrypt(newTok.AccessToken)
		_, _ = database.PostgresDB.Exec(`
			UPDATE calendar_integrations SET access_token_enc = $1, token_expires_at = $2, updated_at = NOW()
			WHERE id = $3
		`, enc, newTok.Expiry, integrationID)
		client = oauth2.NewClient(ctx, ts)
	}

	svc, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	return svc, integrationID, err
}

func markSyncFailed(appointmentID, integrationID uuid.UUID) {
	_, _ = database.PostgresDB.Exec(`
		UPDATE calendar_event_mappings SET sync_status = 'failed', last_synced_at = NOW()
		WHERE appointment_id = $1 AND integration_id = $2
	`, appointmentID, integrationID)
}

func HasCalendarIntegration(tenantID, therapistID uuid.UUID) bool {
	var syncEnabled bool
	err := database.PostgresDB.QueryRow(`
		SELECT sync_enabled FROM calendar_integrations
		WHERE tenant_id = $1 AND therapist_id = $2
	`, tenantID, therapistID).Scan(&syncEnabled)
	return err == nil && syncEnabled
}

func DisconnectGoogleCalendar(tenantID, therapistID uuid.UUID) error {
	_, err := database.PostgresDB.Exec(`
		UPDATE calendar_integrations SET sync_enabled = FALSE, updated_at = NOW()
		WHERE tenant_id = $1 AND therapist_id = $2
	`, tenantID, therapistID)
	return err
}

func LogCalendarStatus() {
	if GoogleCalendarEnabled() {
		log.Println("✅ Google Calendar OAuth configured")
	} else {
		log.Println("⚠️  Google Calendar not configured (set GOOGLE_CLIENT_ID/SECRET)")
	}
}
