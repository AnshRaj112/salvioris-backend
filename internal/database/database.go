package database

import (
	"context"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Client *mongo.Client
var DB *mongo.Database

func Connect(mongoURI string) error {
	// Use longer timeout for Atlas connections
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Parse connection string and set options
	clientOptions := options.Client().ApplyURI(mongoURI)
	
	// Set server selection timeout for Atlas
	clientOptions.SetServerSelectionTimeout(10 * time.Second)
	
	// For Atlas, ensure SSL is enabled (it's usually in the connection string)
	// The connection string from Atlas should include: ?ssl=true or &tls=true
	
	log.Printf("Attempting to connect to MongoDB...")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return err
	}

	// Ping the database with a separate context
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()
	
	err = client.Ping(pingCtx, nil)
	if err != nil {
		client.Disconnect(context.Background())
		return err
	}

	Client = client
	
	// Extract database name from URI or use default
	// If URI contains database name, use it; otherwise use "serenify"
	dbName := "serenify"
	if mongoURI != "" {
		// Try to extract database name from connection string
		// Format: mongodb://.../database_name?...
		parts := strings.Split(mongoURI, "/")
		if len(parts) > 3 {
			dbPart := strings.Split(parts[len(parts)-1], "?")[0]
			if dbPart != "" && dbPart != "serenify" {
				dbName = dbPart
			}
		}
	}
	
	DB = client.Database(dbName)

	log.Println("✅ Connected to MongoDB")
	return nil
}

func Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return Client.Disconnect(ctx)
}

