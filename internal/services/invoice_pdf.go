package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/AnshRaj112/serenify-backend/internal/models"
)

type InvoicePDFUploader interface {
	UploadString(ctx context.Context, content, folder, filename string) (string, error)
}

func BuildInvoiceHTML(inv models.Invoice, patientName, tenantName string) string {
	var rows strings.Builder
	for _, li := range inv.LineItems {
		rows.WriteString(fmt.Sprintf("<tr><td>%s</td><td align='right'>%.2f</td></tr>",
			li.Description, li.Amount))
	}
	due := ""
	if inv.DueAt != nil {
		due = inv.DueAt.Format("2006-01-02")
	}
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>%s</title>
<style>body{font-family:sans-serif;padding:40px}table{width:100%%;border-collapse:collapse}td,th{padding:8px;border-bottom:1px solid #ddd}</style>
</head><body>
<h1>Tax Invoice</h1>
<p><strong>%s</strong></p>
<p>Invoice: %s<br>Patient: %s<br>Date: %s<br>Due: %s</p>
<table><tr><th>Description</th><th>Amount (%s)</th></tr>%s</table>
<p>Subtotal: %.2f<br>GST: %.2f<br><strong>Total: %.2f</strong></p>
<p>Status: %s</p>
</body></html>`,
		inv.InvoiceNumber, tenantName, inv.InvoiceNumber, patientName,
		inv.CreatedAt.Format("2006-01-02"), due, inv.Currency, rows.String(),
		inv.Subtotal, inv.GSTAmount, inv.Total, inv.Status)
}

func UploadInvoicePDF(ctx context.Context, uploader InvoicePDFUploader, tenantID string, inv models.Invoice, patientName, tenantName string) (string, error) {
	if uploader == nil {
		return "", fmt.Errorf("cloudinary not configured")
	}
	html := BuildInvoiceHTML(inv, patientName, tenantName)
	folder := fmt.Sprintf("invoices/%s", tenantID)
	filename := strings.ReplaceAll(inv.InvoiceNumber, "/", "-")
	return uploader.UploadString(ctx, html, folder, filename)
}
