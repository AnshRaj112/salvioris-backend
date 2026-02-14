package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// CORS is handled at the HTTP layer already.
		return true
	},
}

// wsMessage is the client<->server WebSocket payload.
type wsMessage struct {
	Type    string   `json:"type"`
	GroupID string   `json:"group_id,omitempty"`
	Groups  []string `json:"groups,omitempty"`
	Text    string   `json:"text,omitempty"`
}

// ChatWebSocket establishes a single Discord-style WebSocket connection per user.
// Client sends "subscribe"/"unsubscribe"/"message" events as documented in CHAT_SYSTEM_REDESIGN.md.
func ChatWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket connections from browsers can't set custom headers easily,
	// so we support authentication via query parameter `token` (session_token)
	// as well as the standard Authorization Bearer header for flexibility.
	token := r.URL.Query().Get("token")
	if token == "" {
		// Fallback to Authorization header (used by HTTP APIs)
		token = extractBearerToken(r.Header.Get("Authorization"))
	}

	userUUID, ok, err := services.ValidateSession(token)
	if err != nil || !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	wsConn := &wsConnWrapper{Conn: conn}
	uc := services.RegisterUserConnection(userUUID, wsConn)

	// Ensure presence is cleaned up.
	defer func() {
		services.UnregisterUserConnection(userUUID)
		conn.Close()
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start Redis subscriber (no-op if already started).
	services.StartRedisChatSubscriber(ctx)

	for {
		var msg wsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		switch msg.Type {
		case "subscribe":
			handleSubscribe(userUUID, uc, msg)
		case "unsubscribe":
			handleUnsubscribe(userUUID, uc, msg)
		case "message":
			handleIncomingChatMessage(ctx, userUUID, msg)
		case "ping":
			_ = conn.WriteJSON(map[string]string{"type": "pong"})
		default:
			_ = conn.WriteJSON(map[string]string{
				"type":  "error",
				"error": "unknown message type",
			})
		}
	}
}

// wsConnWrapper adapts *websocket.Conn to services.ChatConn.
type wsConnWrapper struct {
	Conn *websocket.Conn
}

func (w *wsConnWrapper) WriteJSON(v interface{}) error {
	return w.Conn.WriteJSON(v)
}

func (w *wsConnWrapper) ReadJSON(dest interface{}) error {
	return w.Conn.ReadJSON(dest)
}

func (w *wsConnWrapper) Close() error {
	return w.Conn.Close()
}

func handleSubscribe(userID uuid.UUID, uc *services.UserConnection, msg wsMessage) {
	targets := msg.Groups
	if msg.GroupID != "" {
		targets = append(targets, msg.GroupID)
	}
	for _, g := range targets {
		if g == "" {
			continue
		}
		services.SubscribeUserToGroup(userID, g)
	}
}

func handleUnsubscribe(userID uuid.UUID, uc *services.UserConnection, msg wsMessage) {
	targets := msg.Groups
	if msg.GroupID != "" {
		targets = append(targets, msg.GroupID)
	}
	for _, g := range targets {
		if g == "" {
			continue
		}
		services.UnsubscribeUserFromGroup(userID, g)
	}
}

// handleIncomingChatMessage validates membership, publishes via Redis, and persists to Mongo.
func handleIncomingChatMessage(ctx context.Context, userID uuid.UUID, msg wsMessage) {
	if msg.GroupID == "" || msg.Text == "" {
		return
	}

	// Validate membership (creator implicitly has membership).
	ok, username := services.CanUserSendToGroup(userID.String(), msg.GroupID)
	if !ok {
		return
	}

	event := services.ChatEvent{
		Type:      "message",
		GroupID:   msg.GroupID,
		SenderID:  userID.String(),
		Username:  username,
		Message:   msg.Text,
		Timestamp: time.Now().UTC(),
	}

	// Publish to Redis FIRST. All instances receive, then fan-out.
	_ = services.PublishChatEvent(ctx, event)

	// Persist asynchronously to Mongo.
	services.SaveChatMessageAsync(services.ChatMessage{
		GroupID:   msg.GroupID,
		SenderID:  userID.String(),
		Username:  username,
		Message:   msg.Text,
		Timestamp: event.Timestamp,
		Status:    "delivered",
	})
}


