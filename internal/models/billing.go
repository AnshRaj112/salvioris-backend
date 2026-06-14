package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type BillingProfile struct {
	TenantID           uuid.UUID       `json:"tenant_id"`
	ConsultationFee    float64         `json:"consultation_fee"`
	SessionFee         float64         `json:"session_fee"`
	SessionFeeInPerson float64         `json:"session_fee_in_person"`
	SessionFeeChat     float64         `json:"session_fee_chat"`
	SessionFeeVoice    float64         `json:"session_fee_voice"`
	SessionFeeVideo    float64         `json:"session_fee_video"`
	PackageFees        json.RawMessage `json:"package_fees,omitempty"`
	GSTRate            float64         `json:"gst_rate"`
	InvoicePrefix      string          `json:"invoice_prefix"`
	Currency           string          `json:"currency"`
	GSTNumber          string          `json:"gst_number,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type InvoiceLineItem struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

type Invoice struct {
	ID            uuid.UUID         `json:"id"`
	TenantID      uuid.UUID         `json:"tenant_id"`
	PatientID     uuid.UUID         `json:"patient_id"`
	InvoiceNumber string            `json:"invoice_number"`
	AppointmentID *uuid.UUID        `json:"appointment_id,omitempty"`
	Subtotal      float64           `json:"subtotal"`
	GSTAmount     float64           `json:"gst_amount"`
	Total         float64           `json:"total"`
	Currency      string            `json:"currency"`
	Status        string            `json:"status"`
	DueAt         *time.Time        `json:"due_at,omitempty"`
	PaidAt        *time.Time        `json:"paid_at,omitempty"`
	PDFURL        string            `json:"pdf_url,omitempty"`
	LineItems     []InvoiceLineItem `json:"line_items"`
	Notes         string            `json:"notes,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type Payment struct {
	ID             uuid.UUID `json:"id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	InvoiceID      uuid.UUID `json:"invoice_id"`
	Provider       string    `json:"provider"`
	ExternalID     string    `json:"external_id,omitempty"`
	Amount         float64   `json:"amount"`
	Status         string    `json:"status"`
	RefundedAmount float64   `json:"refunded_amount"`
	CreatedAt      time.Time `json:"created_at"`
}
