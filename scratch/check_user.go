package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")
	uri := os.Getenv("POSTGRES_URI")
	if uri == "" {
		log.Fatal("POSTGRES_URI is empty")
	}

	db, err := sql.Open("postgres", uri)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("--- USERS ---")
	uRows, err := db.Query("SELECT id, username FROM users")
	if err != nil {
		log.Fatal(err)
	}
	defer uRows.Close()
	for uRows.Next() {
		var id, username string
		if err := uRows.Scan(&id, &username); err == nil {
			fmt.Printf("ID: %s | Username: %s\n", id, username)
		}
	}

	fmt.Println("\n--- THERAPIST USER CONNECTIONS ---")
	cRows, err := db.Query("SELECT id, therapist_id, user_id, connection_type FROM therapist_user_connections")
	if err != nil {
		log.Fatal(err)
	}
	defer cRows.Close()
	for cRows.Next() {
		var id, therapistID, userID, cType string
		if err := cRows.Scan(&id, &therapistID, &userID, &cType); err == nil {
			fmt.Printf("ID: %s | TherapistID: %s | UserID: %s | Type: %s\n", id, therapistID, userID, cType)
		}
	}
}
