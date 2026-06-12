package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
)

func RazorpayWebhookV2(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	sig := r.Header.Get("X-Razorpay-Signature")
	if !services.VerifyRazorpayWebhook(body, sig) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	var payload struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID      string `json:"id"`
					OrderID string `json:"order_id"`
					Status  string `json:"status"`
				} `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if payload.Event == "payment.captured" || payload.Payload.Payment.Entity.Status == "captured" {
		orderID := payload.Payload.Payment.Entity.OrderID
		paymentID := payload.Payload.Payment.Entity.ID
		var invID uuid.UUID
		err := database.PostgresDB.QueryRow(`
			SELECT invoice_id FROM payments WHERE external_id = $1 AND provider = 'razorpay' LIMIT 1
		`, orderID).Scan(&invID)
		if err == nil {
			if err := markInvoicePaid(invID, paymentID, "razorpay"); err != nil {
				log.Printf("webhook payment mark failed: %v", err)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
