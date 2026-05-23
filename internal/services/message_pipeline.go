package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/pkg/crypto"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SecureChatMessage represents the pure client-side encrypted message structure
type SecureChatMessage struct {
	ID         string    `json:"id,omitempty"`
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	DeviceID   string    `json:"device_id"`
	Ciphertext string    `json:"ciphertext"`
	Nonce      string    `json:"nonce"`
	Signature  string    `json:"signature"`
	Timestamp  time.Time `json:"timestamp"`
	Status     string    `json:"status"`
}

// ProcessSecureMessageIngress validates and processes an incoming E2EE message packet.
func ProcessSecureMessageIngress(ctx context.Context, msg SecureChatMessage) error {
	if msg.GroupID == "" || msg.SenderID == "" || msg.Ciphertext == "" || msg.Signature == "" {
		return errors.New("invalid E2EE payload: missing required fields")
	}

	// 1. Resolve the sender device's Ed25519 signing public key
	var signPubKeyB64 string
	err := database.PostgresDB.QueryRowContext(ctx, `
		SELECT sign_pub_key 
		FROM user_devices 
		WHERE user_id = $1 AND device_token = $2
		LIMIT 1
	`, msg.SenderID, msg.DeviceID).Scan(&signPubKeyB64)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("verification failed: sender device keyring not registered")
		}
		return fmt.Errorf("failed to fetch device identity keys: %w", err)
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(signPubKeyB64)
	if err != nil {
		return fmt.Errorf("invalid device signature key formatting: %w", err)
	}

	// 2. Compute E2EE Binding Hash: Hash(Ciphertext || GroupID || SenderID)
	// Binds the payload content to the sender identity and target group channel.
	h := sha256.New()
	h.Write([]byte(msg.Ciphertext))
	h.Write([]byte(msg.GroupID))
	h.Write([]byte(msg.SenderID))
	bindingHash := h.Sum(nil)

	sigBytes, err := base64.StdEncoding.DecodeString(msg.Signature)
	if err != nil {
		return fmt.Errorf("failed to parse E2EE signature: %w", err)
	}

	// 3. Verify digital signature
	isValid, err := crypto.VerifyEd25519Signature(pubKeyBytes, bindingHash, sigBytes)
	if err != nil || !isValid {
		return errors.New("cryptographic signature mismatch: payload tampered or compromised")
	}

	// 4. Asynchronously save E2EE ciphertext to MongoDB
	SaveSecureChatMessageAsync(msg)

	return nil
}

// SaveSecureChatMessageAsync saves the E2EE envelope to MongoDB chat_messages collection.
func SaveSecureChatMessageAsync(msg SecureChatMessage) {
	go func(m SecureChatMessage) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if m.Timestamp.IsZero() {
			m.Timestamp = time.Now().UTC()
		}
		if m.Status == "" {
			m.Status = "delivered"
		}

		col := database.DB.Collection("chat_messages")
		
		doc := map[string]interface{}{
			"group_id":   m.GroupID,
			"sender_id":  m.SenderID,
			"device_id":  m.DeviceID,
			"ciphertext": m.Ciphertext,
			"nonce":      m.Nonce,
			"signature":  m.Signature,
			"timestamp":  m.Timestamp,
			"status":     m.Status,
		}

		if m.ID != "" {
			if oid, err := primitive.ObjectIDFromHex(m.ID); err == nil {
				doc["_id"] = oid
			}
		}

		_, _ = col.InsertOne(ctx, doc)
	}(msg)
}
