package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type dmWSIn struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	Content        string `json:"content,omitempty"`
}

// DMWebSocket handles realtime 1:1 therapist/patient messaging per tenant.
func DMWebSocket(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantId"))
	if err != nil {
		http.Error(w, "Invalid tenant", http.StatusBadRequest)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		token = extractBearerToken(r.Header.Get("Authorization"))
	}

	var userID uuid.UUID
	var role string

	if claims, ok := services.ValidateAccessToken(token); ok {
		userID, _ = uuid.Parse(claims.UserID)
		role = "therapist"
		if claims.TenantID != tenantID.String() {
			if owns, _ := services.TherapistOwnsTenant(userID, tenantID); !owns {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
	} else if uid, ok, err := services.ValidateSession(token); err == nil && ok {
		userID = uid
		if owns, _ := services.TherapistOwnsTenant(uid, tenantID); owns {
			role = "therapist"
		} else {
			var pt uuid.UUID
			err = database.PostgresDB.QueryRow(`
				SELECT tenant_id FROM patients WHERE user_id = $1 AND deleted_at IS NULL LIMIT 1
			`, uid).Scan(&pt)
			if err == sql.ErrNoRows || pt != tenantID {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			role = "patient"
		}
	} else {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	uid := userID.String()
	services.SubscribeDM(tenantID.String(), uid, role, func(evt services.DMEvent) error {
		return conn.WriteJSON(evt)
	})
	defer services.UnsubscribeDM(tenantID.String(), uid)

	for {
		var in dmWSIn
		if err := conn.ReadJSON(&in); err != nil {
			return
		}
		switch in.Type {
		case "typing.start":
			services.SetTyping(tenantID.String(), in.ConversationID, uid)
			services.PublishTyping(tenantID.String(), in.ConversationID, uid)
		case "message.send":
			if in.ConversationID == "" || in.Content == "" {
				continue
			}
			msg, err := insertDMMessage(tenantID.String(), in.ConversationID, uid, role, sendMessageRequest{Content: in.Content})
			if err != nil {
				continue
			}
			_ = conn.WriteJSON(services.DMEvent{
				Type: "message.sent", ConversationID: in.ConversationID,
				MessageID: msg.ID.Hex(), Timestamp: time.Now().Format(time.RFC3339),
			})
		}
	}
}
