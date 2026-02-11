package services

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	chatGroupChannelPrefix   = "chat:group:"
	typingGroupChannelPrefix = "typing:group:"
	presenceKeyPrefix        = "presence:"

	// PresenceTTL defines how long a user is considered online without heartbeat.
	PresenceTTL = 60 * time.Second
)

// ChatEventType describes the kind of realtime event flowing over Redis/WebSockets.
type ChatEventType string

const (
	EventTypeMessage      ChatEventType = "message"
	EventTypeMessageAck   ChatEventType = "message_ack"
	EventTypeTypingStart  ChatEventType = "typing_start"
	EventTypeTypingStop   ChatEventType = "typing_stop"
	EventTypeReadReceipt  ChatEventType = "read_receipt"
	EventTypePresence     ChatEventType = "presence"
	EventTypeError        ChatEventType = "error"
	EventTypeServerNotice ChatEventType = "server_notice"
)

// ChatEvent is the generic payload sent over Redis and WebSockets.
type ChatEvent struct {
	Type      ChatEventType        `json:"type"`
	GroupID   string               `json:"group_id,omitempty"`
	Message   *models.ChatMessage  `json:"message,omitempty"`
	MessageID string               `json:"message_id,omitempty"`
	UserID    string               `json:"user_id,omitempty"`
	Username  string               `json:"username,omitempty"`
	Status    models.ChatMessageStatus `json:"status,omitempty"`
	Timestamp time.Time            `json:"timestamp"`
	Error     string               `json:"error,omitempty"`
}

// ChatHub manages active WebSocket connections per group in-process.
// It works together with Redis Pub/Sub so multiple backend instances stay in sync.
type ChatHub struct {
	mu        sync.RWMutex
	consumers map[string][]chan ChatEvent // key: groupID
}

var (
	defaultChatHub = &ChatHub{
		consumers: make(map[string][]chan ChatEvent),
	}
)

// DefaultChatHubSubscribe is a small wrapper used by handlers to subscribe to a group.
func DefaultChatHubSubscribe(groupID string) (<-chan ChatEvent, func()) {
	return defaultChatHub.SubscribeGroup(groupID)
}

// SubscribeGroup returns a channel that receives ChatEvents for a group.
// The caller MUST call the returned unsubscribe function when done.
func (h *ChatHub) SubscribeGroup(groupID string) (<-chan ChatEvent, func()) {
	ch := make(chan ChatEvent, 64)

	h.mu.Lock()
	h.consumers[groupID] = append(h.consumers[groupID], ch)
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.consumers[groupID]
		for i, c := range list {
			if c == ch {
				// Remove and close channel
				h.consumers[groupID] = append(list[:i], list[i+1:]...)
				close(c)
				break
			}
		}
		if len(h.consumers[groupID]) == 0 {
			delete(h.consumers, groupID)
		}
	}

	return ch, unsubscribe
}

// fanOutToLocalConsumers sends an event to all in-process subscribers for the group.
func (h *ChatHub) fanOutToLocalConsumers(evt ChatEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	consumers := h.consumers[evt.GroupID]
	for _, ch := range consumers {
		select {
		case ch <- evt:
		default:
			// Slow consumer; drop event to avoid blocking hub
		}
	}
}

// StartRedisChatSubscriber starts a long-lived goroutine that listens to Redis Pub/Sub
// for all group chat and typing channels and fans out events to local consumers.
func StartRedisChatSubscriber() {
	if database.RedisClient == nil {
		log.Println("chat_realtime: Redis client not initialized; skipping subscriber")
		return
	}

	ctx := context.Background()

	// Pattern subscribe to both chat and typing channels for all groups.
	pubsub := database.RedisClient.PSubscribe(ctx, chatGroupChannelPrefix+"*", typingGroupChannelPrefix+"*")

	go func() {
		defer pubsub.Close()
		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				log.Printf("chat_realtime: Redis PSubscribe error: %v", err)
				time.Sleep(time.Second)
				continue
			}

			var evt ChatEvent
			if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
				log.Printf("chat_realtime: failed to unmarshal event: %v", err)
				continue
			}

			// Extract group ID from channel name if missing
			if evt.GroupID == "" {
				if len(msg.Channel) > len(chatGroupChannelPrefix) && msg.Channel[:len(chatGroupChannelPrefix)] == chatGroupChannelPrefix {
					evt.GroupID = msg.Channel[len(chatGroupChannelPrefix):]
				} else if len(msg.Channel) > len(typingGroupChannelPrefix) && msg.Channel[:len(typingGroupChannelPrefix)] == typingGroupChannelPrefix {
					evt.GroupID = msg.Channel[len(typingGroupChannelPrefix):]
				}
			}

			defaultChatHub.fanOutToLocalConsumers(evt)
		}
	}()
}

// PublishChatEvent publishes a chat-related event to the appropriate Redis channel.
func PublishChatEvent(ctx context.Context, evt ChatEvent) error {
	if database.RedisClient == nil {
		return nil
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	var channel string
	switch evt.Type {
	case EventTypeMessage, EventTypeMessageAck, EventTypeReadReceipt, EventTypePresence:
		channel = chatGroupChannelPrefix + evt.GroupID
	case EventTypeTypingStart, EventTypeTypingStop:
		channel = typingGroupChannelPrefix + evt.GroupID
	default:
		// Default to chat channel
		channel = chatGroupChannelPrefix + evt.GroupID
	}

	return database.RedisClient.Publish(ctx, channel, payload).Err()
}

// SaveChatMessage persists a new message to MongoDB and returns the saved document.
func SaveChatMessage(ctx context.Context, msg *models.ChatMessage) (*models.ChatMessage, error) {
	if msg.ID.IsZero() {
		msg.ID = primitive.NewObjectID()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	if msg.Status == "" {
		msg.Status = models.MessageStatusSent
	}

	coll := database.DB.Collection("chat_messages")
	_, err := coll.InsertOne(ctx, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// MarkMessagesRead updates MongoDB to mark messages as read for a specific user.
// It also publishes read-receipt events via Redis.
func MarkMessagesRead(ctx context.Context, userID uuid.UUID, username string, updates []models.ChatMessageReadUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	coll := database.DB.Collection("chat_messages")

	for _, u := range updates {
		objID, err := primitive.ObjectIDFromHex(u.MessageID)
		if err != nil {
			continue
		}

		filter := bson.M{
			"_id":      objID,
			"group_id": u.GroupID,
		}
		update := bson.M{
			"$addToSet": bson.M{
				"read_by": userID.String(),
			},
			"$set": bson.M{
				"status": models.MessageStatusRead,
			},
		}

		if _, err := coll.UpdateOne(ctx, filter, update); err != nil {
			log.Printf("chat_realtime: failed to mark message read: %v", err)
			continue
		}

		// Publish read receipt
		evt := ChatEvent{
			Type:      EventTypeReadReceipt,
			GroupID:   u.GroupID,
			MessageID: u.MessageID,
			UserID:    userID.String(),
			Username:  username,
			Status:    models.MessageStatusRead,
			Timestamp: time.Now().UTC(),
		}
		if err := PublishChatEvent(ctx, evt); err != nil {
			log.Printf("chat_realtime: failed to publish read receipt: %v", err)
		}
	}

	return nil
}

// SetUserPresence marks a user as online with a TTL using Redis.
func SetUserPresence(ctx context.Context, userID uuid.UUID, status string) {
	if database.RedisClient == nil {
		return
	}
	key := presenceKeyPrefix + userID.String()
	err := database.RedisClient.Set(ctx, key, status, PresenceTTL).Err()
	if err != nil {
		log.Printf("chat_realtime: failed to set presence: %v", err)
	}
}


