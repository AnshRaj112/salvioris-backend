package services

import (
	"testing"

	"github.com/AnshRaj112/serenify-backend/internal/models"
)

func TestCalcInvoiceTotals(t *testing.T) {
	gst, total := CalcInvoiceTotals(1000, 18)
	if gst != 180 {
		t.Fatalf("gst: got %v want 180", gst)
	}
	if total != 1180 {
		t.Fatalf("total: got %v want 1180", total)
	}
}

func TestSumLineItems(t *testing.T) {
	sum := SumLineItems([]models.InvoiceLineItem{
		{Amount: 500}, {Amount: 250.5},
	})
	if sum != 750.5 {
		t.Fatalf("got %v", sum)
	}
}
