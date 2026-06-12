package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type billingProfileRequest struct {
	ConsultationFee float64         `json:"consultation_fee"`
	SessionFee      float64         `json:"session_fee"`
	GSTRate         float64         `json:"gst_rate"`
	InvoicePrefix   string          `json:"invoice_prefix"`
	GSTNumber       string          `json:"gst_number,omitempty"`
	PackageFees     json.RawMessage `json:"package_fees,omitempty"`
}

type createInvoiceRequest struct {
	PatientID     string                  `json:"patient_id"`
	AppointmentID string                  `json:"appointment_id,omitempty"`
	LineItems     []models.InvoiceLineItem `json:"line_items,omitempty"`
	Notes         string                  `json:"notes,omitempty"`
	DueAt         string                  `json:"due_at,omitempty"`
}

type initiatePaymentRequest struct {
	InvoiceID string `json:"invoice_id"`
}

type collectPaymentRequest struct {
	InvoiceID string  `json:"invoice_id"`
	Provider  string  `json:"provider"`
	Amount    float64 `json:"amount"`
}

type verifyPaymentRequest struct {
	OrderID   string `json:"razorpay_order_id"`
	PaymentID string `json:"razorpay_payment_id"`
	Signature string `json:"razorpay_signature"`
	InvoiceID string `json:"invoice_id"`
}

func GetBillingProfileV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	p, err := services.GetBillingProfile(tenantID)
	if err != nil {
		http.Error(w, "Failed to load billing profile", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": p})
}

func UpdateBillingProfileV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	var req billingProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	_ = services.EnsureBillingProfile(tenantID)
	_, err := database.PostgresDB.Exec(`
		UPDATE billing_profiles SET
			consultation_fee = $2, session_fee = $3, gst_rate = $4,
			invoice_prefix = COALESCE(NULLIF($5,''), invoice_prefix),
			gst_number = $6, package_fees = $7, updated_at = NOW()
		WHERE tenant_id = $1
	`, tenantID, req.ConsultationFee, req.SessionFee, req.GSTRate,
		req.InvoicePrefix, nullStr(req.GSTNumber), nullableJSON(req.PackageFees))
	if err != nil {
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}
	p, _ := services.GetBillingProfile(tenantID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": p})
}

func ListInvoicesV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	status := r.URL.Query().Get("status")
	patientFilter := r.URL.Query().Get("patient_id")
	listInvoices(w, tenantID, patientFilter, status)
}

func CreateInvoiceV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	var req createInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	patientID, err := uuid.Parse(req.PatientID)
	if err != nil || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Invalid patient", http.StatusBadRequest)
		return
	}

	items := req.LineItems
	var aptID *uuid.UUID
	if req.AppointmentID != "" {
		id, e := uuid.Parse(req.AppointmentID)
		if e == nil {
			aptID = &id
			if len(items) == 0 {
				items, _ = services.LineItemsFromAppointment(tenantID, id)
			}
		}
	}
	if len(items) == 0 {
		http.Error(w, "line_items required", http.StatusBadRequest)
		return
	}

	profile, _ := services.GetBillingProfile(tenantID)
	subtotal := services.SumLineItems(items)
	gst, total := services.CalcInvoiceTotals(subtotal, profile.GSTRate)
	invNum, _ := services.NextInvoiceNumber(tenantID, profile.InvoicePrefix)

	var dueAt *time.Time
	if req.DueAt != "" {
		if t, e := time.Parse("2006-01-02", req.DueAt); e == nil {
			dueAt = &t
		}
	} else {
		t := time.Now().AddDate(0, 0, 7)
		dueAt = &t
	}

	itemsJSON, _ := json.Marshal(items)
	var id uuid.UUID
	err = database.PostgresDB.QueryRow(`
		INSERT INTO invoices (
			tenant_id, patient_id, invoice_number, appointment_id,
			subtotal, gst_amount, total, currency, status, due_at, line_items, notes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'draft',$9,$10,$11)
		RETURNING id
	`, tenantID, patientID, invNum, aptID, subtotal, gst, total, profile.Currency, dueAt, itemsJSON, nullStr(req.Notes)).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to create invoice", http.StatusInternalServerError)
		return
	}

	inv, _ := getInvoice(tenantID, id)
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	services.AuditV2Tenant(r, tenantID, "INVOICE_CREATED", "invoice", id.String(), therapistID.String())
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": inv})
}

func GetInvoiceV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	invID, ok := parsePatientIDParam(chi.URLParam(r, "invoiceId"))
	if !ok {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}
	inv, err := getInvoice(tenantID, invID)
	if err == sql.ErrNoRows {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load invoice", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": inv})
}

func SendInvoiceV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	invID, ok := parsePatientIDParam(chi.URLParam(r, "invoiceId"))
	if !ok {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}
	_, err := database.PostgresDB.Exec(`
		UPDATE invoices SET status = 'sent', updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND status = 'draft'
	`, invID, tenantID)
	if err != nil {
		http.Error(w, "Failed to send invoice", http.StatusInternalServerError)
		return
	}
	inv, _ := getInvoice(tenantID, invID)
	services.NotifyPatientByID(inv.PatientID, "Invoice ready", inv.InvoiceNumber+" — please review payment", "invoice")
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": inv})
}

func VerifyPatientPaymentV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	var req verifyPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	if !services.VerifyRazorpayPaymentSignature(req.OrderID, req.PaymentID, req.Signature) {
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}
	invID, _ := uuid.Parse(req.InvoiceID)
	inv, err := getInvoice(tenantID, invID)
	if err != nil || inv.PatientID != patientID {
		http.Error(w, "Invoice not found", http.StatusNotFound)
		return
	}
	if err := markInvoicePaid(invID, req.PaymentID, "razorpay"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func GenerateInvoicePDFV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	invID, ok := parsePatientIDParam(chi.URLParam(r, "invoiceId"))
	if !ok {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}
	inv, err := getInvoice(tenantID, invID)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var patientName, tenantName string
	_ = database.PostgresDB.QueryRow(`SELECT full_name FROM patients WHERE id = $1`, inv.PatientID).Scan(&patientName)
	_ = database.PostgresDB.QueryRow(`SELECT display_name FROM tenants WHERE id = $1`, tenantID).Scan(&tenantName)

	if cloudinaryService == nil {
		http.Error(w, "File storage not configured", http.StatusServiceUnavailable)
		return
	}
	url, err := services.UploadInvoicePDF(context.Background(), cloudinaryService, tenantID.String(), inv, patientName, tenantName)
	if err != nil {
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
		return
	}
	_, _ = database.PostgresDB.Exec(`UPDATE invoices SET pdf_url = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2`,
		invID, tenantID, url)
	inv.PDFURL = url
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": inv, "pdf_url": url})
}

func InitiatePaymentV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	var req initiatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	invID, err := uuid.Parse(req.InvoiceID)
	if err != nil {
		http.Error(w, "Invalid invoice_id", http.StatusBadRequest)
		return
	}
	resp, err := createPaymentOrder(tenantID, invID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func VerifyPaymentV2(w http.ResponseWriter, r *http.Request) {
	var req verifyPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	if !services.VerifyRazorpayPaymentSignature(req.OrderID, req.PaymentID, req.Signature) {
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}
	invID, _ := uuid.Parse(req.InvoiceID)
	if err := markInvoicePaid(invID, req.PaymentID, "razorpay"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func ReceptionCollectPaymentV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	var req collectPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	invID, err := uuid.Parse(req.InvoiceID)
	if err != nil {
		http.Error(w, "Invalid invoice_id", http.StatusBadRequest)
		return
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = "cash"
	}
	inv, err := getInvoice(tenantID, invID)
	if err != nil {
		http.Error(w, "Invoice not found", http.StatusNotFound)
		return
	}
	amount := req.Amount
	if amount <= 0 {
		amount = inv.Total
	}
	_, _ = database.PostgresDB.Exec(`
		INSERT INTO payments (tenant_id, invoice_id, provider, amount, status)
		VALUES ($1, $2, $3, $4, 'succeeded')
	`, tenantID, invID, provider, amount)
	_, _ = database.PostgresDB.Exec(`
		UPDATE invoices SET status = 'paid', paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, invID, tenantID)
	inv, _ = getInvoice(tenantID, invID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": inv})
}

func ListMyInvoicesV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	listInvoices(w, tenantID, patientID.String(), r.URL.Query().Get("status"))
}

func PayMyInvoiceV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	invID, ok := parsePatientIDParam(chi.URLParam(r, "invoiceId"))
	if !ok {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}
	inv, err := getInvoice(tenantID, invID)
	if err != nil || inv.PatientID != patientID {
		http.Error(w, "Invoice not found", http.StatusNotFound)
		return
	}
	resp, err := createPaymentOrder(tenantID, invID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func ListPaymentsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	rows, err := database.PostgresDB.Query(`
		SELECT id, tenant_id, invoice_id, provider, external_id, amount, status, refunded_amount, created_at
		FROM payments WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 100
	`, tenantID)
	if err != nil {
		http.Error(w, "Failed to list payments", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	payments := make([]models.Payment, 0)
	for rows.Next() {
		var p models.Payment
		var ext sql.NullString
		_ = rows.Scan(&p.ID, &p.TenantID, &p.InvoiceID, &p.Provider, &ext, &p.Amount, &p.Status, &p.RefundedAmount, &p.CreatedAt)
		p.ExternalID = ext.String
		payments = append(payments, p)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": payments})
}

func createPaymentOrder(tenantID, invID uuid.UUID) (map[string]interface{}, error) {
	if !services.RazorpayEnabled() {
		return nil, errRazorpayNotConfigured
	}
	inv, err := getInvoice(tenantID, invID)
	if err != nil {
		return nil, err
	}
	if inv.Status == "paid" || inv.Status == "cancelled" {
		return nil, errInvoiceNotPayable
	}
	order, err := services.CreateRazorpayOrder(inv.Total, inv.Currency, inv.InvoiceNumber)
	if err != nil {
		return nil, err
	}
	_, _ = database.PostgresDB.Exec(`
		INSERT INTO payments (tenant_id, invoice_id, provider, external_id, amount, status)
		VALUES ($1, $2, 'razorpay', $3, $4, 'pending')
	`, tenantID, invID, order.ID, inv.Total)
	return map[string]interface{}{
		"order_id": order.ID,
		"amount":   order.Amount,
		"currency": order.Currency,
		"key_id":   services.RazorpayKeyID(),
		"invoice_id": invID.String(),
	}, nil
}

var (
	errRazorpayNotConfigured = &billingErr{"razorpay not configured"}
	errInvoiceNotPayable   = &billingErr{"invoice not payable"}
)

type billingErr struct{ msg string }

func (e *billingErr) Error() string { return e.msg }

func markInvoicePaid(invID uuid.UUID, externalID, provider string) error {
	var tenantID uuid.UUID
	var total float64
	err := database.PostgresDB.QueryRow(`
		SELECT tenant_id, total FROM invoices WHERE id = $1
	`, invID).Scan(&tenantID, &total)
	if err != nil {
		return err
	}
	_, err = database.PostgresDB.Exec(`
		UPDATE payments SET status = 'succeeded', external_id = COALESCE(NULLIF($3,''), external_id)
		WHERE invoice_id = $1 AND tenant_id = $2 AND status = 'pending'
	`, invID, tenantID, externalID)
	if err != nil {
		return err
	}
	_, err = database.PostgresDB.Exec(`
		UPDATE invoices SET status = 'paid', paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, invID, tenantID)
	return err
}

func listInvoices(w http.ResponseWriter, tenantID uuid.UUID, patientID, status string) {
	query := `
		SELECT id, tenant_id, patient_id, invoice_number, appointment_id,
			subtotal, gst_amount, total, currency, status, due_at, paid_at, pdf_url, line_items, notes, created_at, updated_at
		FROM invoices WHERE tenant_id = $1
	`
	args := []interface{}{tenantID}
	n := 2
	if patientID != "" {
		query += fmt.Sprintf(` AND patient_id = $%d`, n)
		args = append(args, patientID)
		n++
	}
	if status != "" {
		query += fmt.Sprintf(` AND status = $%d`, n)
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := database.PostgresDB.Query(query, args...)
	if err != nil {
		http.Error(w, "Failed to list invoices", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	items := make([]models.Invoice, 0)
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			http.Error(w, "Failed to read invoices", http.StatusInternalServerError)
			return
		}
		items = append(items, inv)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

func getInvoice(tenantID, id uuid.UUID) (models.Invoice, error) {
	row := database.PostgresDB.QueryRow(`
		SELECT id, tenant_id, patient_id, invoice_number, appointment_id,
			subtotal, gst_amount, total, currency, status, due_at, paid_at, pdf_url, line_items, notes, created_at, updated_at
		FROM invoices WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	return scanInvoiceRow(row)
}

func scanInvoice(rows *sql.Rows) (models.Invoice, error) {
	var inv models.Invoice
	var aptID sql.NullString
	var due, paid sql.NullTime
	var pdf, notes sql.NullString
	var lineJSON []byte
	err := rows.Scan(&inv.ID, &inv.TenantID, &inv.PatientID, &inv.InvoiceNumber, &aptID,
		&inv.Subtotal, &inv.GSTAmount, &inv.Total, &inv.Currency, &inv.Status,
		&due, &paid, &pdf, &lineJSON, &notes, &inv.CreatedAt, &inv.UpdatedAt)
	if err != nil {
		return inv, err
	}
	if aptID.Valid {
		id := uuid.MustParse(aptID.String)
		inv.AppointmentID = &id
	}
	if due.Valid {
		t := due.Time
		inv.DueAt = &t
	}
	if paid.Valid {
		t := paid.Time
		inv.PaidAt = &t
	}
	inv.PDFURL = pdf.String
	inv.Notes = notes.String
	_ = json.Unmarshal(lineJSON, &inv.LineItems)
	return inv, nil
}

func scanInvoiceRow(row *sql.Row) (models.Invoice, error) {
	var inv models.Invoice
	var aptID sql.NullString
	var due, paid sql.NullTime
	var pdf, notes sql.NullString
	var lineJSON []byte
	err := row.Scan(&inv.ID, &inv.TenantID, &inv.PatientID, &inv.InvoiceNumber, &aptID,
		&inv.Subtotal, &inv.GSTAmount, &inv.Total, &inv.Currency, &inv.Status,
		&due, &paid, &pdf, &lineJSON, &notes, &inv.CreatedAt, &inv.UpdatedAt)
	if err != nil {
		return inv, err
	}
	if aptID.Valid {
		id := uuid.MustParse(aptID.String)
		inv.AppointmentID = &id
	}
	if due.Valid {
		t := due.Time
		inv.DueAt = &t
	}
	if paid.Valid {
		t := paid.Time
		inv.PaidAt = &t
	}
	inv.PDFURL = pdf.String
	inv.Notes = notes.String
	_ = json.Unmarshal(lineJSON, &inv.LineItems)
	return inv, nil
}

func nullableJSON(b json.RawMessage) interface{} {
	if len(b) == 0 {
		return nil
	}
	return []byte(b)
}
