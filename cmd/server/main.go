package main

import (
	"log"
	"net/http"
	"strings"
	
	"github.com/joho/godotenv"

	"github.com/AnshRaj112/serenify-backend/internal/config"
	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/routes"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func main() {
	// Load env
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}
	// Load configuration
	cfg := config.Load()

	// Log connection attempt (without showing password)
	log.Printf("Connecting to MongoDB...")
	if cfg.MongoURI != "" {
		// Mask password in log for security
		maskedURI := cfg.MongoURI
		if strings.Contains(maskedURI, "@") {
			parts := strings.Split(maskedURI, "@")
			if len(parts) > 0 && strings.Contains(parts[0], ":") {
				userPass := strings.Split(parts[0], ":")
				if len(userPass) >= 3 {
					maskedURI = strings.Replace(maskedURI, userPass[2], "***", 1)
				}
			}
		}
		log.Printf("MongoDB URI: %s", maskedURI)
	}

	// Connect to MongoDB
	if err := database.Connect(cfg.MongoURI); err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
		log.Println("\nTroubleshooting tips:")
		log.Println("1. Check if your IP is whitelisted in MongoDB Atlas")
		log.Println("2. Verify your connection string format (should use mongodb+srv:// for Atlas)")
		log.Println("3. Ensure username and password are correct")
		log.Println("4. Check if the cluster is running (not paused)")
		log.Println("See MONGODB_SETUP.md for detailed instructions")
	}
	defer database.Disconnect()

	// Setup router
	r := chi.NewRouter()

	// Setup CORS - Allow requests from frontend URL
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	r.Use(corsMiddleware.Handler)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Setup routes
	routes.SetupRoutes(r)

	log.Printf("🚀 Serenify backend running on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
