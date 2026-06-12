package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DMConversation struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID             string             `bson:"tenant_id" json:"tenant_id"`
	PatientID            string             `bson:"patient_id" json:"patient_id"`
	TherapistID          string             `bson:"therapist_id" json:"therapist_id"`
	LastMessageAt        time.Time          `bson:"last_message_at" json:"last_message_at"`
	LastMessagePreview   string             `bson:"last_message_preview" json:"last_message_preview"`
	UnreadCountPatient   int                `bson:"unread_count_patient" json:"unread_count_patient"`
	UnreadCountTherapist int                `bson:"unread_count_therapist" json:"unread_count_therapist"`
	CreatedAt            time.Time          `bson:"created_at" json:"created_at"`
}

type DMMessage struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID       string             `bson:"tenant_id" json:"tenant_id"`
	ConversationID string             `bson:"conversation_id" json:"conversation_id"`
	SenderID       string             `bson:"sender_id" json:"sender_id"`
	SenderRole     string             `bson:"sender_role" json:"sender_role"`
	Type           string             `bson:"type" json:"type"`
	Content        string             `bson:"content" json:"content"`
	AttachmentURL  string             `bson:"attachment_url,omitempty" json:"attachment_url,omitempty"`
	ReadAt         *time.Time         `bson:"read_at,omitempty" json:"read_at,omitempty"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
}
