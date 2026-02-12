package services

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

// ChatEvent represents the payload broadcast over Redis and WebSocket.
type ChatEvent struct {
	Type      string    `json:"type"`
	GroupID   string    `json:"group_id,omitempty"`
	SenderID  string    `json:"sender_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// UserConnection tracks a single user's WebSocket connection and group subscriptions.
type UserConnection struct {
	UserID       uuid.UUID
	Conn         ChatConn
	SubscribedTo map[string]struct{}
	mu           sync.RWMutex
}

// ChatConn is the minimal interface our WebSocket implementation must satisfy.
type ChatConn interface {
	WriteJSON(v interface{}) error
	ReadJSON(dest interface{}) error
	Close() error
}

// ChatHub is a global registry of user connections.
type ChatHub struct {
	mu          sync.RWMutex
	connections map[uuid.UUID]*UserConnection
}

var (
	chatHub      = &ChatHub{connections: make(map[uuid.UUID]*UserConnection)}
	redisStarted sync.Once
)

// RegisterUserConnection registers or replaces a user's connection.
func RegisterUserConnection(userID uuid.UUID, conn ChatConn) *UserConnection {
	uc := &UserConnection{
		UserID:       userID,
		Conn:         conn,
		SubscribedTo: make(map[string]struct{}),
	}

	chatHub.mu.Lock()
	chatHub.connections[userID] = uc
	chatHub.mu.Unlock()

	return uc
}

// UnregisterUserConnection removes a user's connection.
func UnregisterUserConnection(userID uuid.UUID) {
	chatHub.mu.Lock()
	delete(chatHub.connections, userID)
	chatHub.mu.Unlock()
}

// SubscribeUserToGroup tracks a subscription in-memory for fan-out.
func SubscribeUserToGroup(userID uuid.UUID, groupID string) {
	chatHub.mu.RLock()
	uc, ok := chatHub.connections[userID]
	chatHub.mu.RUnlock()
	if !ok {
		return
	}
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.SubscribedTo[groupID] = struct{}{}
}

// UnsubscribeUserFromGroup removes a subscription.
func UnsubscribeUserFromGroup(userID uuid.UUID, groupID string) {
	chatHub.mu.RLock()
	uc, ok := chatHub.connections[userID]
	chatHub.mu.RUnlock()
	if !ok {
		return
	}
	uc.mu.Lock()
	defer uc.mu.Unlock()
	delete(uc.SubscribedTo, groupID)
}

// FanOutChatEvent sends an event to all local connections subscribed to the group.
func FanOutChatEvent(event ChatEvent) {
	if event.GroupID == "" {
		return
	}

	chatHub.mu.RLock()
	defer chatHub.mu.RUnlock()

	for _, uc := range chatHub.connections {
		uc.mu.RLock()
		_, subscribed := uc.SubscribedTo[event.GroupID]
		uc.mu.RUnlock()
		if !subscribed {
			continue
		}

		// Non-blocking best-effort send.
		go func(c ChatConn) {
			if err := c.WriteJSON(event); err != nil {
				log.Printf("error writing chat event to websocket: %v", err)
			}
		}(uc.Conn)
	}
}

// StartRedisChatSubscriber ensures a single shared Redis listener per instance.
func StartRedisChatSubscriber(ctx context.Context) {
	redisStarted.Do(func() {
		go runRedisSubscriber(ctx)
	})
}

func runRedisSubscriber(ctx context.Context) {
	client := database.RedisClient
	if client == nil {
		log.Println("Redis client not initialized; chat subscriber not started")
		return
	}

	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		func() {
			pubsub := client.PSubscribe(ctx, "chat:group:*")
			defer pubsub.Close()

			log.Println("✅ Chat Redis subscriber started (pattern: chat:group:*)")

			for {
				msg, err := pubsub.ReceiveMessage(ctx)
				if err != nil {
					log.Printf("Redis subscriber error: %v", err)
					time.Sleep(backoff)
					backoff *= 2
					if backoff > 30*time.Second {
						backoff = 30 * time.Second
					}
					return
				}

				backoff = time.Second

				var event ChatEvent
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					log.Printf("failed to unmarshal chat event: %v", err)
					continue
				}

				// Fan out to local connections.
				FanOutChatEvent(event)
			}
		}()
	}
}

// PublishChatEvent publishes an event to Redis; called when a message is received over WebSocket.
func PublishChatEvent(ctx context.Context, event ChatEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	channel := "chat:group:" + event.GroupID
	return database.RedisClient.Publish(ctx, channel, data).Err()
}


