package services

import (
	"context"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func EnsureV2MongoIndexes(ctx context.Context) error {
	indexes := []struct {
		coll string
		models []mongo.IndexModel
	}{
		{
			coll: "session_notes",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "patient_id", Value: 1}, {Key: "session_number", Value: 1}}},
				{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "plain_text", Value: "text"}}},
			},
		},
		{
			coll: "session_note_versions",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "note_id", Value: 1}, {Key: "version", Value: -1}}},
			},
		},
		{
			coll: "wellness_entries",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "patient_id", Value: 1}, {Key: "entry_date", Value: -1}}},
				{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "patient_id", Value: 1}, {Key: "entry_date", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			coll: "patient_journals",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "patient_id", Value: 1}, {Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
			},
		},
		{
			coll: "dm_conversations",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "last_message_at", Value: -1}}},
				{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "patient_id", Value: 1}, {Key: "therapist_id", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			coll: "ai_insights",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "patient_id", Value: 1}, {Key: "created_at", Value: -1}}},
			},
		},
		{
			coll: "dm_messages",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "content", Value: "text"}}},
			},
		},
	}

	for _, idx := range indexes {
		if _, err := database.DB.Collection(idx.coll).Indexes().CreateMany(ctx, idx.models); err != nil {
			return err
		}
	}
	return nil
}
