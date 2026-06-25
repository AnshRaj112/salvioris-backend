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

	// 1. Check if RLS is enabled
	var rlsEnabled bool
	err = db.QueryRow(`
		SELECT relrowsecurity FROM pg_class WHERE relname = 'patients'
	`).Scan(&rlsEnabled)
	if err != nil {
		log.Fatal("Failed to check relrowsecurity:", err)
	}
	fmt.Printf("RLS enabled on patients: %v\n", rlsEnabled)

	// 2. Query patients directly
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM patients").Scan(&count)
	if err != nil {
		fmt.Printf("Query without RLS context failed: %v\n", err)
	} else {
		fmt.Printf("Query without RLS context succeeded, count: %d\n", count)
	}

	// 3. Try to set config and query
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("SELECT set_config('app.tenant_id', 'fd4f7dbb-bbb9-4806-bc1a-e44fe2e27714', true)")
	if err != nil {
		fmt.Printf("Set config failed: %v\n", err)
	} else {
		var countTx int
		err = tx.QueryRow("SELECT COUNT(*) FROM patients").Scan(&countTx)
		if err != nil {
			fmt.Printf("Query inside Tx with RLS failed: %v\n", err)
		} else {
			fmt.Printf("Query inside Tx with RLS succeeded, count: %d\n", countTx)
		}
	}
}
