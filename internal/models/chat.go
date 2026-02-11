package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ChatMessageStatus represents the delivery/read status of a message for a given user.
// Valid values: "sent", "delivered", "read".
type ChatMessageStatus string

const (
	MessageStatusSent      ChatMessageStatus = "sent"
	MessageStatusDelivered ChatMessageStatus = "delivered"
	MessageStatusRead      ChatMessageStatus = "read"
)

// ChatMessage is stored in MongoDB and represents a single group message.
// We use a flat collection (one document per message) for scalability and pagination.
type ChatMessage struct {
	ID             primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	GroupID        string               `bson:"group_id" json:"group_id"`
	SenderID       string               `bson:"sender_id" json:"sender_id"`
	SenderUsername string               `bson:"sender_username" json:"sender_username"`
	Text           string               `bson:"text" json:"text"`
	CreatedAt      time.Time            `bson:"created_at" json:"created_at"`
	Status         ChatMessageStatus    `bson:"status" json:"status"` // global status from sender point-of-view
	DeliveredTo    []string             `bson:"delivered_to,omitempty" json:"delivered_to,omitempty"`
	ReadBy         []string             `bson:"read_by,omitempty" json:"read_by,omitempty"`
}

// ChatMessageReadUpdate is used when marking messages as read.
type ChatMessageReadUpdate struct {
	MessageID string `json:"message_id"`
	GroupID   string `json:"group_id"`
}


