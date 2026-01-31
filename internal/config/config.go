package config

import (
	"os"
)

type Config struct {
	MongoURI         string
	JWTSecret        string
	Port             string
	FrontendURL      string
	CloudinaryName   string
	CloudinaryAPIKey string
	CloudinaryAPISecret string
}

func Load() *Config {
	return &Config{
		MongoURI:          getEnv("MONGODB_URI", getEnv("MONGO_URI", "mongodb://localhost:27017/serenify")),
		JWTSecret:         getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		Port:              getEnv("PORT", "8080"),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:3000"),
		CloudinaryName:    getEnv("CLOUDINARY_CLOUD_NAME", ""),
		CloudinaryAPIKey:  getEnv("CLOUDINARY_API_KEY", ""),
		CloudinaryAPISecret: getEnv("CLOUDINARY_API_SECRET", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

