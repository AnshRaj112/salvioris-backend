package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
)

type DMEvent struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id"`
	TenantID       string `json:"tenant_id"`
	SenderID       string `json:"sender_id"`
	SenderRole     string `json:"sender_role"`
	MessageID      string `json:"message_id,omitempty"`
	Content        string `json:"content,omitempty"`
	Timestamp      string `json:"timestamp,omitempty"`
}

type dmSubscriber struct {
	userID   string
	role     string
	tenantID string
	send     func(DMEvent) error
}

var (
	dmMu          sync.RWMutex
	dmSubscribers = map[string]map[string]*dmSubscriber{} // tenantID -> userID -> sub
	dmRedisOnce   sync.Once
)

func dmChannel(tenantID string) string {
	return fmt.Sprintf("dm:tenant:%s", tenantID)
}

func StartDMHub() {
	dmRedisOnce.Do(func() {
		if database.RedisClient == nil {
			return
		}
		go func() {
			ctx := context.Background()
			pubsub := database.RedisClient.Subscribe(ctx, dmChannel("*"))
			// Redis pattern subscribe not in go-redis simple API - use per-tenant publish only
			_ = pubsub
		}()
	})
}

func SubscribeDM(tenantID, userID, role string, send func(DMEvent) error) {
	dmMu.Lock()
	defer dmMu.Unlock()
	if dmSubscribers[tenantID] == nil {
		dmSubscribers[tenantID] = make(map[string]*dmSubscriber)
	}
	dmSubscribers[tenantID][userID] = &dmSubscriber{userID: userID, role: role, tenantID: tenantID, send: send}
}

func UnsubscribeDM(tenantID, userID string) {
	dmMu.Lock()
	defer dmMu.Unlock()
	if m := dmSubscribers[tenantID]; m != nil {
		delete(m, userID)
	}
}

func BroadcastDM(tenantID string, evt DMEvent, excludeUserID string) {
	dmMu.RLock()
	defer dmMu.RUnlock()
	for uid, sub := range dmSubscribers[tenantID] {
		if uid == excludeUserID {
			continue
		}
		_ = sub.send(evt)
	}
	if database.RedisClient != nil {
		data, _ := json.Marshal(evt)
		_ = database.RedisClient.Publish(context.Background(), dmChannel(tenantID), data).Err()
	}
}

func SetTyping(tenantID, conversationID, userID string) {
	if database.RedisClient == nil {
		return
	}
	key := fmt.Sprintf("dm:typing:%s:%s", conversationID, userID)
	_ = database.RedisClient.Set(context.Background(), key, "1", 5*time.Second).Err()
}

func PublishTyping(tenantID, conversationID, userID string) {
	BroadcastDM(tenantID, DMEvent{
		Type: "typing", ConversationID: conversationID, TenantID: tenantID, SenderID: userID,
	}, userID)
}
