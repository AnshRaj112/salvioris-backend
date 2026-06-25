package main

import (
	"fmt"
	"log"
	"os"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")
	uri := os.Getenv("POSTGRES_URI")
	if uri == "" {
		log.Fatal("POSTGRES_URI is empty")
	}

	err := database.ConnectPostgres(uri)
	if err != nil {
		log.Fatal(err)
	}
	defer database.PostgresDB.Close()

	userID, err := uuid.Parse("7fb0748e-bf12-46ec-b3ad-a7acb5116ae9")
	if err != nil {
		log.Fatal(err)
	}

	patientID, tenantID, err := services.EnsurePatientProfileForUser(userID)
	if err != nil {
		fmt.Printf("EnsurePatientProfileForUser failed: %v\n", err)
	} else {
		fmt.Printf("EnsurePatientProfileForUser succeeded: PatientID=%s, TenantID=%s\n", patientID, tenantID)
	}
}
