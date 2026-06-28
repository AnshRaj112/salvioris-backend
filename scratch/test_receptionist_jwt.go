package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
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
	database.PostgresDB = db
	services.InitJWT(os.Getenv("JWT_SECRET"))

	// Check if there are receptionists in the DB
	fmt.Println("--- RECEPTIONISTS ---")
	rows, err := db.Query("SELECT id, tenant_id, email, name FROM receptionists LIMIT 5")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var receptionistID, tenantID uuid.UUID
	var email, name string
	found := false

	for rows.Next() {
		if err := rows.Scan(&receptionistID, &tenantID, &email, &name); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID: %s | TenantID: %s | Email: %s | Name: %s\n", receptionistID, tenantID, email, name)
		found = true
	}

	if !found {
		fmt.Println("No receptionists found. We will create a dummy one for testing.")
		// We need a therapist first to get a valid therapist_id and tenant_id
		var therapistID string
		err := db.QueryRow("SELECT id FROM therapists LIMIT 1").Scan(&therapistID)
		if err != nil {
			log.Fatal("Cannot run test: No therapists in database to associate receptionist with:", err)
		}
		var tid string
		err = db.QueryRow("SELECT id FROM tenants WHERE therapist_id = $1 LIMIT 1", therapistID).Scan(&tid)
		if err != nil {
			log.Fatal("Cannot run test: No tenant for therapist:", err)
		}

		receptionistID = uuid.New()
		tenantID = uuid.MustParse(tid)
		email = "test-receptionist@example.com"
		name = "Test Receptionist"

		_, err = db.Exec(`
			INSERT INTO receptionists (id, tenant_id, therapist_id, name, email, password_hash)
			VALUES ($1, $2, $3, $4, $5, 'dummyhash')
		`, receptionistID, tenantID, therapistID, name, email)
		if err != nil {
			log.Fatal("Failed to insert test receptionist:", err)
		}
		fmt.Printf("Created test receptionist ID: %s | TenantID: %s\n", receptionistID, tenantID)

		// Clean up later
		defer func() {
			_, _ = db.Exec("DELETE FROM receptionists WHERE id = $1", receptionistID)
			fmt.Println("Cleaned up test receptionist.")
		}()
	}

	// Now try issuing tokens for receptionistID and tenantID
	fmt.Println("\n--- ISSUING TOKENS ---")
	tokenPair, err := services.IssueReceptionistTokens(receptionistID, tenantID)
	if err != nil {
		log.Fatal("Failed to issue receptionist tokens:", err)
	}
	fmt.Println("Successfully issued tokens!")
	fmt.Println("AccessToken:", tokenPair.AccessToken[:20]+"...")
	fmt.Println("RefreshToken:", tokenPair.RefreshToken[:20]+"...")

	// Verify the access token claims
	fmt.Println("\n--- VALIDATING ACCESS TOKEN ---")
	claims, valid := services.ValidateReceptionistAccessToken(tokenPair.AccessToken)
	if !valid {
		log.Fatal("Access token is invalid")
	}
	fmt.Printf("Valid! UserID: %s, TenantID: %s, Role: %s\n", claims.UserID, claims.TenantID, claims.Role)

	// Refresh the tokens
	fmt.Println("\n--- REFRESHING ACCESS TOKEN ---")
	refreshedPair, err := services.RefreshAccessToken(tokenPair.RefreshToken)
	if err != nil {
		log.Fatal("Failed to refresh token:", err)
	}
	fmt.Println("Successfully refreshed tokens!")
	fmt.Println("Refreshed AccessToken:", refreshedPair.AccessToken[:20]+"...")
	fmt.Println("Refreshed RefreshToken:", refreshedPair.RefreshToken[:20]+"...")

	// Verify refreshed access token
	fmt.Println("\n--- VALIDATING REFRESHED ACCESS TOKEN ---")
	refreshedClaims, valid := services.ValidateReceptionistAccessToken(refreshedPair.AccessToken)
	if !valid {
		log.Fatal("Refreshed access token is invalid")
	}
	fmt.Printf("Valid! UserID: %s, TenantID: %s, Role: %s\n", refreshedClaims.UserID, refreshedClaims.TenantID, refreshedClaims.Role)
	fmt.Println("\nALL TESTS PASSED SUCCESSFULLY!")
}
