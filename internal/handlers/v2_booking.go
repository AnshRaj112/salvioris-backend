package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type bookingInitiateRequest struct {
	TherapistID string `json:"therapist_id"`
	Type        string `json:"type"`
	StartsAt    string `json:"starts_at"`
	Notes       string `json:"notes,omitempty"`
}

type bookingVerifyRequest struct {
	OrderID       string `json:"razorpay_order_id"`
	PaymentID     string `json:"razorpay_payment_id"`
	Signature     string `json:"razorpay_signature"`
	InvoiceID     string `json:"invoice_id"`
	AppointmentID string `json:"appointment_id"`
}

func parseTimeStr(s string) (time.Time, error) {
	if len(s) == 5 {
		return time.Parse("15:04", s)
	}
	return time.Parse("15:04:05", s)
}

func GetTherapistAvailabilityForPatientV2(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	therapistID, err := uuid.Parse(chi.URLParam(r, "therapistId"))
	if err != nil {
		http.Error(w, "Invalid therapist ID", http.StatusBadRequest)
		return
	}

	tenantID, err := services.EnsureTenantForTherapist(therapistID)
	if err != nil {
		http.Error(w, "Therapist tenant not found", http.StatusNotFound)
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	timezoneStr := "Asia/Kolkata"
	_ = database.PostgresDB.QueryRow(`SELECT timezone FROM tenants WHERE id = $1`, tenantID).Scan(&timezoneStr)
	loc, _ := time.LoadLocation(timezoneStr)
	if loc == nil {
		loc = time.UTC
	}

	day, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		http.Error(w, "Invalid date format (use YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	// 1. Fetch doctor's availability slots for this day of week
	rows, err := database.PostgresDB.Query(`
		SELECT start_time::text, end_time::text, slot_duration_min
		FROM availability_slots
		WHERE tenant_id = $1 AND therapist_id = $2 AND day_of_week = $3 AND is_active = TRUE
	`, tenantID, therapistID, int(day.Weekday()))
	if err != nil {
		http.Error(w, "Failed to load therapist availability slots", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type slot struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	slots := make([]slot, 0)
	for rows.Next() {
		var start, end string
		var dur int
		_ = rows.Scan(&start, &end, &dur)
		if dur <= 0 {
			dur = 60
		}

		startTime, err1 := parseTimeStr(start)
		endTime, err2 := parseTimeStr(end)
		if err1 != nil || err2 != nil {
			continue
		}

		for current := startTime; !current.Add(time.Duration(dur) * time.Minute).After(endTime); current = current.Add(time.Duration(dur) * time.Minute) {
			slotStartStr := current.Format("15:04:05")
			slotEndStr := current.Add(time.Duration(dur) * time.Minute).Format("15:04:05")
			slots = append(slots, slot{Start: slotStartStr, End: slotEndStr})
		}
	}

	// 2. Define day start and end times for queries
	dayStart := day
	dayEnd := day.Add(24 * time.Hour).Add(-1 * time.Second)

	// 3. Fetch busy ranges from existing database bookings
	bookingRows, err := database.PostgresDB.Query(`
		SELECT starts_at, ends_at FROM appointments
		WHERE therapist_id = $1 AND status NOT IN ('cancelled', 'no_show', 'pending_payment')
		AND starts_at >= $2 AND starts_at <= $3
	`, therapistID, dayStart, dayEnd)
	
	type busyRange struct {
		Start time.Time
		End   time.Time
	}
	var busyRanges []busyRange
	if err == nil {
		defer bookingRows.Close()
		for bookingRows.Next() {
			var bs, be time.Time
			if err := bookingRows.Scan(&bs, &be); err == nil {
				busyRanges = append(busyRanges, busyRange{Start: bs, End: be})
			}
		}
	}

	// 4. Fetch busy ranges from Google Calendar
	gRanges, gErr := services.GetGoogleCalendarBusyTimes(tenantID, therapistID, dayStart, dayEnd)
	if gErr == nil {
		for _, gr := range gRanges {
			busyRanges = append(busyRanges, busyRange{Start: gr.Start, End: gr.End})
		}
	}

	// 5. Check which slots are free
	freeSlots := make([]slot, 0)
	for _, s := range slots {
		// Construct absolute time for slot start/end in UTC/Z format
		sStartStr := dateStr + "T" + s.Start
		if len(s.Start) == 5 {
			sStartStr += ":00"
		}
		sEndStr := dateStr + "T" + s.End
		if len(s.End) == 5 {
			sEndStr += ":00"
		}
		
		// Parse in the therapist's local timezone
		slotStart, err1 := time.ParseInLocation("2006-01-02T15:04:05", sStartStr, loc)
		slotEnd, err2 := time.ParseInLocation("2006-01-02T15:04:05", sEndStr, loc)
		if err1 != nil || err2 != nil {
			continue
		}

		overlap := false
		for _, busy := range busyRanges {
			// standard overlap check: startA < endB and endA > startB
			if slotStart.Before(busy.End) && slotEnd.After(busy.Start) {
				overlap = true
				break
			}
		}
		if !overlap {
			freeSlots = append(freeSlots, s)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": freeSlots, "date": dateStr})
}

func InitiateBookingV2(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req bookingInitiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	therapistID, err := uuid.Parse(req.TherapistID)
	if err != nil {
		http.Error(w, "Invalid therapist ID", http.StatusBadRequest)
		return
	}

	tenantID, err := services.EnsureTenantForTherapist(therapistID)
	if err != nil {
		http.Error(w, "Therapist tenant not found", http.StatusNotFound)
		return
	}

	startsAt, err := services.ParseRFC3339(req.StartsAt)
	if err != nil {
		http.Error(w, "Invalid starts_at time (RFC3339 format required)", http.StatusBadRequest)
		return
	}

	timezoneStr := "Asia/Kolkata"
	_ = database.PostgresDB.QueryRow(`SELECT timezone FROM tenants WHERE id = $1`, tenantID).Scan(&timezoneStr)
	loc, _ := time.LoadLocation(timezoneStr)
	if loc == nil {
		loc = time.UTC
	}

	startsAtLocal := startsAt.In(loc)
	timeOnlyStr := startsAtLocal.Format("15:04:05")

	// Resolve slot duration for this specific block from availability settings
	slotDuration := 60 // default fallback
	var dbSlotDur int
	err = database.PostgresDB.QueryRow(`
		SELECT slot_duration_min 
		FROM availability_slots
		WHERE tenant_id = $1 AND therapist_id = $2 AND day_of_week = $3 
		AND start_time <= $4::time AND end_time >= $4::time AND is_active = TRUE
		LIMIT 1
	`, tenantID, therapistID, int(startsAtLocal.Weekday()), timeOnlyStr).Scan(&dbSlotDur)
	if err == nil && dbSlotDur > 0 {
		slotDuration = dbSlotDur
	}

	endsAt := startsAt.Add(time.Duration(slotDuration) * time.Minute)

	// Validate slot is not conflicted in DB
	conflict, err := services.TherapistHasConflict(therapistID, startsAt, endsAt, nil)
	if err != nil || conflict {
		http.Error(w, "This time slot is already booked", http.StatusConflict)
		return
	}

	// Validate against Google Calendar busy times
	gBusy, err := services.GetGoogleCalendarBusyTimes(tenantID, therapistID, startsAt.Add(-1*time.Minute), endsAt.Add(1*time.Minute))
	if err == nil {
		for _, busy := range gBusy {
			if startsAt.Before(busy.End) && endsAt.After(busy.Start) {
				http.Error(w, "This time slot conflicts with the therapist's Google Calendar", http.StatusConflict)
				return
			}
		}
	}

	// Resolve the patient ID under the therapist's tenant
	var patientID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		SELECT id FROM patients WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, tenantID, userID).Scan(&patientID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Profile doesn't exist yet, insert it automatically
			var username string
			_ = database.PostgresDB.QueryRow("SELECT username FROM users WHERE id = $1", userID).Scan(&username)
			if username == "" {
				username = "Patient"
			}
			
			var emailEncrypted sql.NullString
			_ = database.PostgresDB.QueryRow(`
				SELECT email_encrypted FROM user_recovery WHERE user_id = $1
			`, userID).Scan(&emailEncrypted)

			emailStr := ""
			if emailEncrypted.Valid && emailEncrypted.String != "" {
				if decrypted, err := utils.Decrypt(emailEncrypted.String); err == nil {
					emailStr = decrypted
				}
			}

			err = database.PostgresDB.QueryRow(`
				INSERT INTO patients (tenant_id, user_id, full_name, email, assigned_therapist_id, status)
				VALUES ($1, $2, $3, NULLIF($4, ''), $5, 'active')
				RETURNING id
			`, tenantID, userID, username, strings.ToLower(strings.TrimSpace(emailStr)), therapistID).Scan(&patientID)
			if err != nil {
				http.Error(w, "Failed to create patient profile link: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Database error looking up patient record", http.StatusInternalServerError)
			return
		}
	}

	// Resolve the fee configuration
	profile, err := services.GetBillingProfile(tenantID)
	if err != nil {
		http.Error(w, "Failed to retrieve therapist billing profile", http.StatusInternalServerError)
		return
	}

	fee := 0.0
	desc := ""
	switch req.Type {
	case "in_person":
		fee = profile.SessionFeeInPerson
		desc = "In-Person Session"
	case "chat":
		fee = profile.SessionFeeChat
		desc = "Chat Session"
	case "voice":
		fee = profile.SessionFeeVoice
		desc = "Voice Call Session"
	case "video":
		fee = profile.SessionFeeVideo
		desc = "Video Call Session"
	default:
		fee = profile.SessionFee
		desc = "Therapy Session"
	}

	// Fallback to defaults
	if fee == 0 {
		if req.Type == "in_person" {
			fee = profile.SessionFee
		} else {
			fee = profile.ConsultationFee
			if fee == 0 {
				fee = profile.SessionFee
			}
		}
	}
	if fee == 0 {
		fee = 1000.0 // ultimate fallback
	}

	// Write draft appointment with status 'pending_payment'
	var appointmentID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		INSERT INTO appointments (
			tenant_id, patient_id, therapist_id, type, status, starts_at, ends_at, notes
		) VALUES ($1, $2, $3, $4, 'pending_payment', $5, $6, $7)
		RETURNING id
	`, tenantID, patientID, therapistID, req.Type, startsAt, endsAt, nullStr(req.Notes)).Scan(&appointmentID)
	if err != nil {
		http.Error(w, "Failed to create booking draft: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create draft invoice
	subtotal := fee
	gst, total := services.CalcInvoiceTotals(subtotal, profile.GSTRate)
	invNum, _ := services.NextInvoiceNumber(tenantID, profile.InvoicePrefix)
	dueAt := time.Now().AddDate(0, 0, 1)

	lineItem := models.InvoiceLineItem{Description: desc, Amount: fee}
	itemsJSON, _ := json.Marshal([]models.InvoiceLineItem{lineItem})

	var invoiceID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		INSERT INTO invoices (
			tenant_id, patient_id, invoice_number, appointment_id,
			subtotal, gst_amount, total, currency, status, due_at, line_items, notes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'draft',$9,$10,$11)
		RETURNING id
	`, tenantID, patientID, invNum, appointmentID, subtotal, gst, total, profile.Currency, dueAt, itemsJSON, "Direct booking fee payment").Scan(&invoiceID)
	if err != nil {
		http.Error(w, "Failed to generate draft invoice: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate Razorpay Order
	if !services.RazorpayEnabled() {
		http.Error(w, "Payment provider integration is disabled", http.StatusServiceUnavailable)
		return
	}

	order, err := services.CreateRazorpayOrder(total, profile.Currency, invNum)
	if err != nil {
		http.Error(w, "Razorpay order generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Record payment status
	_, err = database.PostgresDB.Exec(`
		INSERT INTO payments (tenant_id, invoice_id, provider, external_id, amount, status)
		VALUES ($1, $2, 'razorpay', $3, $4, 'pending')
	`, tenantID, invoiceID, order.ID, total)
	if err != nil {
		http.Error(w, "Failed to record payment transaction details", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"order_id":       order.ID,
		"amount":         order.Amount,
		"currency":       order.Currency,
		"key_id":         services.RazorpayKeyID(),
		"invoice_id":     invoiceID.String(),
		"appointment_id": appointmentID.String(),
	})
}

func VerifyBookingPaymentV2(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req bookingVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 1. Verify Payment signature
	if !services.VerifyRazorpayPaymentSignature(req.OrderID, req.PaymentID, req.Signature) {
		http.Error(w, "Invalid payment signature verification failed", http.StatusBadRequest)
		return
	}

	aptID, err := uuid.Parse(req.AppointmentID)
	if err != nil {
		http.Error(w, "Invalid appointment ID", http.StatusBadRequest)
		return
	}
	invID, err := uuid.Parse(req.InvoiceID)
	if err != nil {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}

	// Verify ownership of the appointment by the user
	var aptTenantID, aptTherapistID uuid.UUID
	var patientID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		SELECT tenant_id, therapist_id, patient_id FROM appointments WHERE id = $1
	`, aptID).Scan(&aptTenantID, &aptTherapistID, &patientID)
	if err != nil {
		http.Error(w, "Appointment not found", http.StatusNotFound)
		return
	}

	var ptUserID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		SELECT user_id FROM patients WHERE id = $1
	`, patientID).Scan(&ptUserID)
	if err != nil || ptUserID != userID {
		http.Error(w, "Unauthorized access to this booking", http.StatusForbidden)
		return
	}

	// 2. Transact: mark invoice as paid, payment succeeded, appointment scheduled
	tx, err := database.PostgresDB.Begin()
	if err != nil {
		http.Error(w, "Failed to start database verification transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE payments SET status = 'succeeded', external_id = $3
		WHERE invoice_id = $1 AND tenant_id = $2 AND status = 'pending'
	`, invID, aptTenantID, req.PaymentID)
	if err != nil {
		http.Error(w, "Failed to update payment status", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(`
		UPDATE invoices SET status = 'paid', paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, invID, aptTenantID)
	if err != nil {
		http.Error(w, "Failed to mark invoice as paid", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(`
		UPDATE appointments SET status = 'scheduled', updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND status = 'pending_payment'
	`, aptID, aptTenantID)
	if err != nil {
		http.Error(w, "Failed to activate scheduled appointment status", http.StatusInternalServerError)
		return
	}

	// 3. Establish therapist-user relationship connection if not already present
	_, err = tx.Exec(`
		INSERT INTO therapist_user_connections (id, therapist_id, user_id, connected_at, connection_type)
		VALUES (gen_random_uuid(), $1, $2, NOW(), 'booking')
		ON CONFLICT (therapist_id, user_id) DO NOTHING
	`, aptTherapistID, userID)
	if err != nil {
		http.Error(w, "Failed to establish user connection link", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Transaction commit failure", http.StatusInternalServerError)
		return
	}

	// 4. Trigger calendar synchronization
	services.EnqueueCalendarSync("create", aptTenantID, aptID)

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}
