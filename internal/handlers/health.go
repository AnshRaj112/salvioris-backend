package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
)

func HealthLiveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func HealthReadiness(w http.ResponseWriter, _ *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	checks := map[string]string{"postgres": "ok", "mongo": "ok", "redis": "ok"}
	status := http.StatusOK

	if database.PostgresDB == nil {
		checks["postgres"] = "down"
		status = http.StatusServiceUnavailable
	} else if err := database.PostgresDB.PingContext(ctx); err != nil {
		checks["postgres"] = "down"
		status = http.StatusServiceUnavailable
	}

	if database.Client == nil {
		checks["mongo"] = "down"
		status = http.StatusServiceUnavailable
	} else if err := database.Client.Ping(ctx, nil); err != nil {
		checks["mongo"] = "down"
		status = http.StatusServiceUnavailable
	}

	if database.RedisClient == nil {
		checks["redis"] = "down"
		status = http.StatusServiceUnavailable
	} else if err := database.RedisClient.Ping(ctx).Err(); err != nil {
		checks["redis"] = "down"
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": map[bool]string{true: "ready", false: "degraded"}[status == http.StatusOK],
		"checks": checks,
	})
}
