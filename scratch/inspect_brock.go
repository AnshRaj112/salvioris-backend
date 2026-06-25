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

	var id, tenantID, userID, fullName, status string
	var deletedAt sql.NullTime
	err = db.QueryRow(`
		SELECT id, tenant_id, user_id, full_name, status, deleted_at
		FROM patients WHERE user_id = '7fb0748e-bf12-46ec-b3ad-a7acb5116ae9'
	`).Scan(&id, &tenantID, &userID, &fullName, &status, &deletedAt)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Patient: ID=%s, TenantID=%s, UserID=%s, Name=%s, Status=%s, DeletedAt=%v\n",
		id, tenantID, userID, fullName, status, deletedAt.Time)
	
	// Re-enable RLS
	_, _ = db.Exec("ALTER TABLE patients ENABLE ROW LEVEL SECURITY")
}
