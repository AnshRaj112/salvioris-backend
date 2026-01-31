package config

import (
	"os"
	"strings"
)

type Config struct {
	MongoURI      string
	JWTSecret     string
	Port          string
	FrontendURL   string
	AllowedOrigins []string
}

func Load() *Config {
	allowedOriginsStr := getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001")
	allowedOrigins := strings.Split(allowedOriginsStr, ",")
	// Trim whitespace from each origin
	for i, origin := range allowedOrigins {
		allowedOrigins[i] = strings.TrimSpace(origin)
	}

	return &Config{
		MongoURI:       getEnv("MONGODB_URI", getEnv("MONGO_URI", "mongodb://localhost:27017/serenify")),
		JWTSecret:      getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		Port:           getEnv("PORT", "8080"),
		FrontendURL:    getEnv("FRONTEND_URL", "http://localhost:3000"),
		AllowedOrigins: allowedOrigins,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

