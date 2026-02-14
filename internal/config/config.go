package config

import (
	"os"
	"strings"
)

type Config struct {
	MongoURI            string
	PostgresURI         string
	RedisURI            string
	JWTSecret           string
	EncryptionKey       string
	Port                string
	FrontendURL         string
	AllowedOrigins      []string // CORS: from ALLOWED_ORIGINS or FRONTEND_URL(s); must include production frontend origin
	CloudinaryName      string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string
	Host                string   // Raw HOST env (e.g. https://backend.salvioris.com)
	AllowedHost         string   // Hostname only for strict host check (production only)
	Environment         string   // ENV: production, development, etc.
}

func Load() *Config {
	env := strings.ToLower(strings.TrimSpace(getEnv("ENV", "development")))
	host := getEnv("HOST", "http://localhost:8080")

	// AllowedHost is only set in production; host check is skipped in development
	var allowedHost string
	if env == "production" {
		allowedHost = host
		if strings.HasPrefix(allowedHost, "https://") {
			allowedHost = strings.TrimPrefix(allowedHost, "https://")
		} else if strings.HasPrefix(allowedHost, "http://") {
			allowedHost = strings.TrimPrefix(allowedHost, "http://")
		}
		if idx := strings.Index(allowedHost, "/"); idx != -1 {
			allowedHost = allowedHost[:idx]
		}
		if idx := strings.Index(allowedHost, ":"); idx != -1 {
			allowedHost = allowedHost[:idx]
		}
		allowedHost = strings.TrimSpace(allowedHost)
	}

	// CORS: allow multiple origins so production frontend (e.g. https://salvioris.com) works
	allowedOrigins := parseOrigins(getEnv("ALLOWED_ORIGINS", ""))
	if len(allowedOrigins) == 0 {
		for _, u := range []string{getEnv("FRONTEND_URL", "http://localhost:3000"), getEnv("FRONTEND_URL_2", ""), getEnv("FRONTEND_URL_3", "")} {
			u = strings.TrimSpace(u)
			if u != "" {
				allowedOrigins = append(allowedOrigins, u)
			}
		}
	}
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:3000"}
	}

	return &Config{
		MongoURI:            getEnv("MONGODB_URI", getEnv("MONGO_URI", "mongodb://localhost:27017/serenify")),
		PostgresURI:         getEnv("POSTGRES_URI", "postgres://localhost:5432/serenify?sslmode=disable"),
		RedisURI:            getEnv("REDIS_URI", "redis://localhost:6379/0"),
		JWTSecret:           getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		EncryptionKey:       getEnv("ENCRYPTION_KEY", ""),
		Host:                host,
		AllowedHost:         allowedHost,
		Environment:         env,
		Port:                getEnv("PORT", "8080"),
		FrontendURL:         getEnv("FRONTEND_URL", "http://localhost:3000"),
		AllowedOrigins:      allowedOrigins,
		CloudinaryName:      getEnv("CLOUDINARY_CLOUD_NAME", ""),
		CloudinaryAPIKey:    getEnv("CLOUDINARY_API_KEY", ""),
		CloudinaryAPISecret: getEnv("CLOUDINARY_API_SECRET", ""),
	}
}

func parseOrigins(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// IsProduction returns true when ENV is set to "production".
func (c *Config) IsProduction() bool {
	return strings.ToLower(strings.TrimSpace(c.Environment)) == "production"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
