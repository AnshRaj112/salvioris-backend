package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("MONGODB_URI is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("salvioris")

	fmt.Println("--- DM CONVERSATIONS ---")
	cursor, err := db.Collection("dm_conversations").Find(ctx, bson.M{})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	var convos []bson.M
	if err := cursor.All(ctx, &convos); err == nil {
		for _, c := range convos {
			fmt.Printf("Convo ID: %v | TenantID: %v | PatientID: %v | TherapistID: %v | LastMsg: %v\n",
				c["_id"], c["tenant_id"], c["patient_id"], c["therapist_id"], c["last_message_preview"])
		}
	} else {
		log.Println("Error decoding conversations:", err)
	}

	fmt.Println("\n--- DM MESSAGES (last 10) ---")
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(10)
	mCursor, err := db.Collection("dm_messages").Find(ctx, bson.M{}, opts)
	if err != nil {
		log.Fatal(err)
	}
	defer mCursor.Close(ctx)

	var msgs []bson.M
	if err := mCursor.All(ctx, &msgs); err == nil {
		for _, m := range msgs {
			fmt.Printf("Msg ID: %v | ConvoID: %v | SenderID: %v | Role: %v | Content: %v\n",
				m["_id"], m["conversation_id"], m["sender_id"], m["sender_role"], m["content"])
		}
	} else {
		log.Println("Error decoding messages:", err)
	}
}
