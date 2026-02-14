package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
)

const (
	chatRecentKeyPrefix = "chat:group:"
	chatRecentKeySuffix = ":recent"
	chatRecentMaxLen    = 50
	chatRecentTTL       = 1 * time.Hour
)

func chatRecentKey(groupID string) string {
	return chatRecentKeyPrefix + groupID + chatRecentKeySuffix
}

// PushMessageToRecentCache adds a message to the Redis recent cache (newest at head).
// Call after saving to Mongo. LPUSH + LTRIM keeps last 50.
func PushMessageToRecentCache(msg ChatMessage) {
	if database.RedisClient == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := chatRecentKey(msg.GroupID)
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	pipe := database.RedisClient.Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, chatRecentMaxLen-1)
	pipe.Expire(ctx, key, chatRecentTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("chat_cache: push failed for group %s: %v", msg.GroupID, err)
	}
}

// GetRecentMessagesFromCache returns the most recent messages for a group (oldest-first).
// Only valid when before is nil (initial load). Returns (messages, true) on hit, (nil, false) on miss.
func GetRecentMessagesFromCache(ctx context.Context, groupID string) ([]ChatMessage, bool) {
	if database.RedisClient == nil {
		return nil, false
	}

	key := chatRecentKey(groupID)
	raw, err := database.RedisClient.LRange(ctx, key, 0, -1).Result()
	if err != nil || len(raw) == 0 {
		return nil, false
	}

	var msgs []ChatMessage
	for i := len(raw) - 1; i >= 0; i-- {
		var m ChatMessage
		if json.Unmarshal([]byte(raw[i]), &m) != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs, true
}

// LoadChatMessagesWithCache returns history for a group. For initial load (before==nil), tries
// Redis first. On miss, fetches from Mongo and warms the cache.
func LoadChatMessagesWithCache(ctx context.Context, groupID string, before *time.Time, limit int64) ([]ChatMessage, bool, error) {
	if before == nil && limit <= chatRecentMaxLen {
		if cached, ok := GetRecentMessagesFromCache(ctx, groupID); ok {
			out := cached
			if int64(len(cached)) > limit {
				out = cached[:limit]
			}
			hasMore := int64(len(cached)) >= limit
			return out, hasMore, nil
		}
	}

	msgs, hasMore, err := LoadChatMessages(ctx, groupID, before, limit)
	if err != nil {
		return nil, false, err
	}
	if before == nil && len(msgs) > 0 {
		WarmRecentCache(ctx, groupID, msgs)
	}
	return msgs, hasMore, nil
}

// WarmRecentCache stores messages in Redis (oldest at tail). Call on Mongo fetch for initial load.
func WarmRecentCache(ctx context.Context, groupID string, msgs []ChatMessage) {
	if database.RedisClient == nil || len(msgs) == 0 {
		return
	}

	key := chatRecentKey(groupID)
	pipe := database.RedisClient.Pipeline()
	for i := len(msgs) - 1; i >= 0; i-- {
		data, err := json.Marshal(msgs[i])
		if err != nil {
			continue
		}
		pipe.RPush(ctx, key, data)
	}
	pipe.LTrim(ctx, key, 0, chatRecentMaxLen-1)
	pipe.Expire(ctx, key, chatRecentTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("chat_cache: warm failed for group %s: %v", groupID, err)
	}
}
