package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ChatWebSocketUpgrade is the shared upgrader for chat WebSocket connections.
var chatUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// CORS for WebSocket is handled at the HTTP layer already.
		// Here we allow all origins; you can tighten this by checking r.Header["Origin"].
		return true
	},
}

// ChatClientMessage represents messages coming from the frontend over WebSocket.
type ChatClientMessage struct {
	Type      string `json:"type"` // "message", "typing_start", "typing_stop", "read", "ping"
	GroupID   string `json:"group_id"`
	Text      string `json:"text,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

// ChatWebSocket handles real-time group chat over WebSocket.
// Authentication is done via the existing session token (Authorization: Bearer <token>).
// Each connection is currently bound to a single group via the `group_id` query parameter.
func ChatWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate user via session token
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		// Fallback: allow token via query parameter for browser WebSocket clients
		token = r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing session token", http.StatusUnauthorized)
			return
		}
	}

	userID, ok, err := services.ValidateSession(token)
	if err != nil || !ok {
		http.Error(w, "invalid session token", http.StatusUnauthorized)
		return
	}

	groupID := r.URL.Query().Get("group_id")
	if groupID == "" {
		http.Error(w, "group_id is required", http.StatusBadRequest)
		return
	}

	// Ensure user is a member of the group (PostgreSQL)
	if !isUserMemberOfGroup(userID, groupID) {
		http.Error(w, "you must be a member of this group", http.StatusForbidden)
		return
	}

	username, _ := services.GetUsernameByID(userID.String())

	conn, err := chatUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mark user as online
	services.SetUserPresence(ctx, userID, "online")

	// Subscribe to local events for this group (fed by Redis subscriber)
	eventsCh, unsubscribe := services.DefaultChatHubSubscribe(groupID)
	defer unsubscribe()

	// Writer goroutine: forward events from hub to this WebSocket connection
	go func() {
		for evt := range eventsCh {
			// Deliver only events relevant to this group (already filtered)
			if err := conn.WriteJSON(evt); err != nil {
				return
			}
		}
	}()

	// Reader loop: handle client messages
	conn.SetReadLimit(64 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(appData string) error {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			// On disconnect, rely on TTL-based presence expiry
			return
		}

		var msg ChatClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		msg.GroupID = strings.TrimSpace(msg.GroupID)
		if msg.GroupID == "" {
			msg.GroupID = groupID
		}

		switch msg.Type {
		case "message":
			handleIncomingChatMessage(ctx, conn, userID, username, msg)
		case "typing_start":
			_ = services.PublishChatEvent(ctx, services.ChatEvent{
				Type:      services.EventTypeTypingStart,
				GroupID:   groupID,
				UserID:    userID.String(),
				Username:  username,
				Timestamp: time.Now().UTC(),
			})
		case "typing_stop":
			_ = services.PublishChatEvent(ctx, services.ChatEvent{
				Type:      services.EventTypeTypingStop,
				GroupID:   groupID,
				UserID:    userID.String(),
				Username:  username,
				Timestamp: time.Now().UTC(),
			})
		case "read":
			if msg.MessageID != "" {
				_ = services.MarkMessagesRead(ctx, userID, username, []models.ChatMessageReadUpdate{
					{
						MessageID: msg.MessageID,
						GroupID:   groupID,
					},
				})
			}
		case "ping":
			// Refresh presence TTL
			services.SetUserPresence(ctx, userID, "online")
		default:
			// Ignore unknown types
		}
	}
}

// handleIncomingChatMessage validates, persists to MongoDB, publishes via Redis,
// and sends an acknowledgement back to the sender.
func handleIncomingChatMessage(
	ctx context.Context,
	conn *websocket.Conn,
	userID uuid.UUID,
	username string,
	msg ChatClientMessage,
) {
	text := strings.TrimSpace(msg.Text)
	if text == "" || msg.GroupID == "" {
		return
	}

	chatMsg := &models.ChatMessage{
		GroupID:        msg.GroupID,
		SenderID:       userID.String(),
		SenderUsername: username,
		Text:           text,
		CreatedAt:      time.Now().UTC(),
		Status:         models.MessageStatusSent,
	}

	saved, err := services.SaveChatMessage(ctx, chatMsg)
	if err != nil {
		// Send error event back
		_ = conn.WriteJSON(services.ChatEvent{
			Type:      services.EventTypeError,
			GroupID:   msg.GroupID,
			Error:     "failed to persist message",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Publish message event
	evt := services.ChatEvent{
		Type:    services.EventTypeMessage,
		GroupID: msg.GroupID,
		Message: saved,
	}
	_ = services.PublishChatEvent(ctx, evt)

	// Send message acknowledgement specifically to sender
	ack := services.ChatEvent{
		Type:    services.EventTypeMessageAck,
		GroupID: msg.GroupID,
		Message: saved,
	}
	_ = conn.WriteJSON(ack)
}

// isUserMemberOfGroup checks membership in the SQL group_members table.
func isUserMemberOfGroup(userID uuid.UUID, groupID string) bool {
	var exists bool
	err := database.PostgresDB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)
	`, groupID, userID).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}


