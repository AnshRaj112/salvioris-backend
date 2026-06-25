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

	// Disable RLS temporarily to inspect
	_, _ = db.Exec("ALTER TABLE patients DISABLE ROW LEVEL SECURITY")

	fmt.Println("--- PATIENTS ---")
	pRows, err := db.Query("SELECT id, tenant_id, user_id, full_name, email FROM patients")
	if err != nil {
		log.Println("Patients error:", err)
	} else {
		defer pRows.Close()
		for pRows.Next() {
			var id, tenantID string
			var userID, name, email sql.NullString
			if err := pRows.Scan(&id, &tenantID, &userID, &name, &email); err == nil {
				fmt.Printf("ID: %s | TenantID: %s | UserID: %s | Name: %s | Email: %s\n", id, tenantID, userID.String, name.String, email.String)
			}
		}
	}

	// Re-enable RLS
	_, _ = db.Exec("ALTER TABLE patients ENABLE ROW LEVEL SECURITY")
}
