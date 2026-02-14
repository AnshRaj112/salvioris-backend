package services

import (
	"context"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ChatMessage struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	GroupID   string             `bson:"group_id" json:"group_id"`
	SenderID  string             `bson:"sender_id" json:"sender_id"`
	Username  string             `bson:"username,omitempty" json:"username,omitempty"`
	Message   string             `bson:"message" json:"message"`
	Timestamp time.Time          `bson:"timestamp" json:"timestamp"`
	Status    string             `bson:"status" json:"status"` // e.g. "delivered", "read"
}

// EnsureChatIndexes configures indexes for the chat_messages collection.
// Called on startup from main after Mongo has connected.
func EnsureChatIndexes(ctx context.Context) error {
	col := database.DB.Collection("chat_messages")

	// Compound index on (group_id, timestamp) to support efficient pagination.
	models := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().SetName("idx_group_timestamp"),
		},
	}

	for _, m := range models {
		if _, err := col.Indexes().CreateOne(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// SaveChatMessageAsync persists a message to MongoDB asynchronously.
// The caller should NOT block on this; fire-and-forget is acceptable.
func SaveChatMessageAsync(msg ChatMessage) {
	go func(m ChatMessage) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if m.Timestamp.IsZero() {
			m.Timestamp = time.Now().UTC()
		}
		if m.Status == "" {
			m.Status = "delivered"
		}

		col := database.DB.Collection("chat_messages")
		_, _ = col.InsertOne(ctx, m)
	}(msg)
}

// LoadChatMessages returns paginated chat history for a group.
// Pagination is based on timestamp + limit (newest-first scrolling).
func LoadChatMessages(ctx context.Context, groupID string, before *time.Time, limit int64) ([]ChatMessage, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	col := database.DB.Collection("chat_messages")

	filter := bson.M{
		"group_id": groupID,
	}
	if before != nil {
		filter["timestamp"] = bson.M{"$lt": before.UTC()}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(limit + 1)

	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, false, err
	}
	defer cur.Close(ctx)

	var msgs []ChatMessage
	for cur.Next(ctx) {
		var m ChatMessage
		if err := cur.Decode(&m); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	if err := cur.Err(); err != nil {
		return nil, false, err
	}

	hasMore := int64(len(msgs)) > limit
	if hasMore {
		msgs = msgs[:len(msgs)-1]
	}

	// Reverse to oldest-first for the UI.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, hasMore, nil
}


