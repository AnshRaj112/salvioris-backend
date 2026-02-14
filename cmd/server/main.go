package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/joho/godotenv"

	"github.com/AnshRaj112/serenify-backend/internal/config"
	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/handlers"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/routes"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"github.com/go-chi/chi/v5"
)

func main() {
	// Load env
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}
	// Load configuration
	cfg := config.Load()

	// Check encryption key (warn if not set, but don't fail)
	if cfg.EncryptionKey == "" {
		log.Println("⚠️  WARNING: ENCRYPTION_KEY not set. Recovery email encryption will not work.")
		log.Println("   To generate a key, run: openssl rand -base64 32")
		log.Println("   Set it in your environment: ENCRYPTION_KEY=<generated-key>")
	} else {
		// Validate encryption key format
		if _, err := utils.GetEncryptionKey(); err != nil {
			log.Printf("⚠️  WARNING: ENCRYPTION_KEY is invalid: %v", err)
			log.Println("   Recovery email encryption will not work.")
			log.Println("   Key must be base64-encoded 32 bytes. Generate with: openssl rand -base64 32")
		} else {
			log.Println("✅ Encryption key configured")
		}
	}

	// Connect to PostgreSQL
	log.Printf("Connecting to PostgreSQL...")
	if err := database.ConnectPostgres(cfg.PostgresURI); err != nil {
		log.Fatal("Failed to connect to PostgreSQL:", err)
	}
	defer database.DisconnectPostgres()

	// Connect to Redis
	log.Printf("Connecting to Redis...")
	if err := database.ConnectRedis(cfg.RedisURI); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	defer database.DisconnectRedis()

	// Initialize Cloudinary service
	if cfg.CloudinaryName != "" && cfg.CloudinaryAPIKey != "" && cfg.CloudinaryAPISecret != "" {
		if err := handlers.InitCloudinaryService(cfg); err != nil {
			log.Printf("Warning: Failed to initialize Cloudinary: %v", err)
			log.Println("File uploads will not be available")
		} else {
			log.Println("✅ Cloudinary service initialized")
		}
	} else {
		log.Println("Warning: Cloudinary credentials not found. File uploads will not be available")
	}

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

	// Ensure MongoDB indexes for chat history
	if err := services.EnsureChatIndexes(context.Background()); err != nil {
		log.Printf("⚠️  WARNING: failed to ensure MongoDB chat indexes: %v", err)
	} else {
		log.Println("✅ MongoDB chat indexes ensured")
	}

	// Start violation cleanup service
	// Cleans up violations older than 6 hours, runs every hour
	// Note: This does NOT delete blocked IPs - those are kept separately
	services.StartViolationCleanup(1, 6) // Run every 1 hour, delete violations older than 6 hours
	log.Println("✅ Violation cleanup service started (removes violations older than 6 hours)")

	// Setup router
	r := chi.NewRouter()

	// Custom CORS: set headers and respond to OPTIONS with 200 so preflight never gets 403
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	// Production: SecurityHeaders → GlobalRateLimit → LoginRateLimit (no host check; no CDN/proxy)
	// Non-production: Redis-based rate limit only
	if cfg.IsProduction() {
		for _, mw := range middleware.ProductionSecurity() {
			r.Use(mw)
		}
		log.Println("✅ Production security enabled (security headers, per-IP + login rate limiting)")
	} else {
		r.Use(middleware.RateLimitMiddleware)
	}

	// Health check (no rate limit)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Setup routes
	routes.SetupRoutes(r)

	// Log registered routes for debugging
	log.Println("📋 Registered routes:")
	log.Println("  GET  /health")
	log.Println("  POST /api/auth/user/signup")
	log.Println("  POST /api/auth/user/signin")
	log.Println("  POST /api/auth/therapist/signup")
	log.Println("  POST /api/auth/therapist/signin")
	log.Println("  GET  /api/therapist/status")
	log.Println("  GET  /api/therapist")
	log.Println("  POST /api/upload")
	log.Println("  GET  /api/admin/therapists/pending")
	log.Println("  GET  /api/admin/therapists/approved")
	log.Println("  PUT  /api/admin/therapists/approve")
	log.Println("  DELETE /api/admin/therapists/reject")
	log.Println("  POST /api/vent")
	log.Println("  GET  /api/vent")

	log.Printf("🚀 Serenify backend running on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
