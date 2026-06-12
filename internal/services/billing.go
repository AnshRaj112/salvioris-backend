package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/google/uuid"
)

func EnsureBillingProfile(tenantID uuid.UUID) error {
	_, err := database.PostgresDB.Exec(`
		INSERT INTO billing_profiles (tenant_id) VALUES ($1)
		ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID)
	return err
}

func GetBillingProfile(tenantID uuid.UUID) (models.BillingProfile, error) {
	_ = EnsureBillingProfile(tenantID)
	var p models.BillingProfile
	var consult, session, gst sql.NullFloat64
	var prefix, currency, gstNum sql.NullString
	var packages sql.NullString
	err := database.PostgresDB.QueryRow(`
		SELECT tenant_id, consultation_fee, session_fee, package_fees, gst_rate,
			invoice_prefix, currency, gst_number, created_at, updated_at
		FROM billing_profiles WHERE tenant_id = $1
	`, tenantID).Scan(&p.TenantID, &consult, &session, &packages, &gst,
		&prefix, &currency, &gstNum, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	p.ConsultationFee = consult.Float64
	p.SessionFee = session.Float64
	p.GSTRate = gst.Float64
	if p.GSTRate == 0 {
		p.GSTRate = 18
	}
	p.InvoicePrefix = prefix.String
	if p.InvoicePrefix == "" {
		p.InvoicePrefix = "INV"
	}
	p.Currency = currency.String
	if p.Currency == "" {
		p.Currency = "INR"
	}
	p.GSTNumber = gstNum.String
	if packages.Valid {
		p.PackageFees = json.RawMessage(packages.String)
	}
	return p, nil
}

func CalcInvoiceTotals(subtotal, gstRate float64) (gst, total float64) {
	gst = math.Round(subtotal*gstRate/100*100) / 100
	total = math.Round((subtotal+gst)*100) / 100
	return
}

func NextInvoiceNumber(tenantID uuid.UUID, prefix string) (string, error) {
	var count int
	err := database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM invoices WHERE tenant_id = $1
	`, tenantID).Scan(&count)
	if err != nil {
		return "", err
	}
	year := time.Now().Year()
	return fmt.Sprintf("%s-%d-%04d", prefix, year, count+1), nil
}

func LineItemsFromAppointment(tenantID uuid.UUID, appointmentID uuid.UUID) ([]models.InvoiceLineItem, error) {
	profile, err := GetBillingProfile(tenantID)
	if err != nil {
		return nil, err
	}
	var aptType string
	err = database.PostgresDB.QueryRow(`
		SELECT type FROM appointments WHERE id = $1 AND tenant_id = $2
	`, appointmentID, tenantID).Scan(&aptType)
	if err != nil {
		return nil, err
	}
	amount := profile.SessionFee
	desc := "Session fee"
	if aptType == "online" {
		amount = profile.ConsultationFee
		if amount == 0 {
			amount = profile.SessionFee
		}
		desc = "Online consultation"
	}
	if amount == 0 {
		amount = profile.SessionFee
	}
	return []models.InvoiceLineItem{{Description: desc, Amount: amount}}, nil
}

func SumLineItems(items []models.InvoiceLineItem) float64 {
	var s float64
	for _, it := range items {
		s += it.Amount
	}
	return s
}

func CreateDraftInvoiceFromAppointment(tenantID, appointmentID uuid.UUID) error {
	var exists int
	_ = database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM invoices WHERE tenant_id = $1 AND appointment_id = $2
	`, tenantID, appointmentID).Scan(&exists)
	if exists > 0 {
		return nil
	}

	var patientID uuid.UUID
	err := database.PostgresDB.QueryRow(`
		SELECT patient_id FROM appointments WHERE id = $1 AND tenant_id = $2 AND status = 'completed'
	`, appointmentID, tenantID).Scan(&patientID)
	if err != nil {
		return err
	}

	items, err := LineItemsFromAppointment(tenantID, appointmentID)
	if err != nil || len(items) == 0 {
		return err
	}

	profile, _ := GetBillingProfile(tenantID)
	subtotal := SumLineItems(items)
	gst, total := CalcInvoiceTotals(subtotal, profile.GSTRate)
	invNum, err := NextInvoiceNumber(tenantID, profile.InvoicePrefix)
	if err != nil {
		return err
	}
	dueAt := time.Now().AddDate(0, 0, 7)
	itemsJSON, _ := json.Marshal(items)
	_, err = database.PostgresDB.Exec(`
		INSERT INTO invoices (
			tenant_id, patient_id, invoice_number, appointment_id,
			subtotal, gst_amount, total, currency, status, due_at, line_items
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'draft',$9,$10)
	`, tenantID, patientID, invNum, appointmentID, subtotal, gst, total, profile.Currency, dueAt, itemsJSON)
	return err
}
