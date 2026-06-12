package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/AnshRaj112/serenify-backend/internal/config"
)

var (
	razorpayKeyID        string
	razorpayKeySecret    string
	razorpayWebhookSecret string
)

func InitRazorpay(cfg *config.Config) {
	razorpayKeyID = cfg.RazorpayKeyID
	razorpayKeySecret = cfg.RazorpayKeySecret
	razorpayWebhookSecret = cfg.RazorpayWebhookSecret
}

func RazorpayEnabled() bool {
	return razorpayKeyID != "" && razorpayKeySecret != ""
}

func RazorpayKeyID() string { return razorpayKeyID }

type RazorpayOrder struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
	Currency string `json:"currency"`
}

func CreateRazorpayOrder(amountRupees float64, currency, receipt string) (*RazorpayOrder, error) {
	if !RazorpayEnabled() {
		return nil, fmt.Errorf("razorpay not configured")
	}
	paise := int(amountRupees * 100)
	body := fmt.Sprintf(`{"amount":%d,"currency":"%s","receipt":"%s"}`, paise, currency, receipt)
	req, err := http.NewRequest("POST", "https://api.razorpay.com/v1/orders", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(razorpayKeyID, razorpayKeySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("razorpay error: %s", string(data))
	}
	var order RazorpayOrder
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func VerifyRazorpayPaymentSignature(orderID, paymentID, signature string) bool {
	payload := orderID + "|" + paymentID
	mac := hmac.New(sha256.New, []byte(razorpayKeySecret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func VerifyRazorpayWebhook(body []byte, signature string) bool {
	secret := razorpayWebhookSecret
	if secret == "" {
		secret = razorpayKeySecret
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
