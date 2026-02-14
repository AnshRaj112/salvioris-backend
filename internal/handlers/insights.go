package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
)

// RecordActivityRequest is the JSON body for POST /api/activity
type RecordActivityRequest struct {
	Path string `json:"path"`
}

// RecordActivity records a page view (or other activity). User ID is optional (from session).
func RecordActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var body RecordActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid JSON"})
		return
	}
	path := body.Path
	if path == "" {
		path = r.URL.Path
	}
	if len(path) > 500 {
		path = path[:500]
	}

	var userID *uuid.UUID
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token != "" {
		if id, ok, _ := services.ValidateSession(token); ok {
			userID = &id
		}
	}

	if userID != nil {
		_, err := database.PostgresDB.Exec(`
			INSERT INTO activity_events (user_id, path, event_type, created_at)
			VALUES ($1, $2, 'page_view', NOW())
		`, userID, path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to record activity"})
			return
		}
	} else {
		_, err := database.PostgresDB.Exec(`
			INSERT INTO activity_events (user_id, path, event_type, created_at)
			VALUES (NULL, $1, 'page_view', NOW())
		`, path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to record activity"})
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// GetInsights returns analytics for the admin dashboard (new users per day, recurring users, top pages).
func GetInsights(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	now := time.Now().UTC()
	to := now
	from := now.AddDate(0, 0, -30) // default last 30 days
	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t.UTC()
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t.UTC()
		}
	}
	if from.After(to) {
		from, to = to, from
	}
	toEnd := to.AddDate(0, 0, 1) // exclusive upper bound (end of "to" day)

	// New users per day
	type dayCount struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	newUsersPerDay := make([]dayCount, 0)
	rows, err := database.PostgresDB.Query(`
		SELECT (created_at)::date AS d, COUNT(*)
		FROM users
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY (created_at)::date
		ORDER BY d
	`, from, toEnd)
	if err != nil {
		log.Printf("[GetInsights] Failed to fetch new users: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to fetch new users"})
		return
	}
	for rows.Next() {
		var d time.Time
		var c int
		if err := rows.Scan(&d, &c); err != nil {
			rows.Close()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to scan"})
			return
		}
		newUsersPerDay = append(newUsersPerDay, dayCount{Date: d.Format("2006-01-02"), Count: c})
	}
	rows.Close()

	// Active users per day (distinct user_id from activity_events per day)
	activeUsersPerDay := make([]dayCount, 0)
	rows, err = database.PostgresDB.Query(`
		SELECT (created_at)::date AS d, COUNT(DISTINCT user_id)
		FROM activity_events
		WHERE user_id IS NOT NULL AND created_at >= $1 AND created_at < $2
		GROUP BY (created_at)::date
		ORDER BY d
	`, from, toEnd)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to fetch active users"})
		return
	}
	for rows.Next() {
		var d time.Time
		var c int
		if err := rows.Scan(&d, &c); err != nil {
			rows.Close()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to scan"})
			return
		}
		activeUsersPerDay = append(activeUsersPerDay, dayCount{Date: d.Format("2006-01-02"), Count: c})
	}
	rows.Close()

	// Recurring users: users who had activity on 2+ distinct days in the range
	var recurringCount int
	err = database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT user_id
			FROM activity_events
			WHERE user_id IS NOT NULL AND created_at >= $1 AND created_at < $2
			GROUP BY user_id
			HAVING COUNT(DISTINCT (created_at)::date) >= 2
		) t
	`, from, toEnd).Scan(&recurringCount)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to fetch recurring users"})
		return
	}

	// Top pages (by view count)
	type pageCount struct {
		Path  string `json:"path"`
		Count int    `json:"count"`
	}
	topPages := make([]pageCount, 0)
	rows, err = database.PostgresDB.Query(`
		SELECT path, COUNT(*) AS c
		FROM activity_events
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY path
		ORDER BY c DESC
		LIMIT 20
	`, from, toEnd)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to fetch top pages"})
		return
	}
	for rows.Next() {
		var path string
		var c int
		if err := rows.Scan(&path, &c); err != nil {
			rows.Close()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to scan"})
			return
		}
		topPages = append(topPages, pageCount{Path: path, Count: c})
	}
	rows.Close()

	// Total new users in period
	var totalNewUsers int
	_ = database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM users
		WHERE created_at >= $1 AND created_at < $2
	`, from, toEnd).Scan(&totalNewUsers)

	// Total active users (distinct) in period
	var totalActiveUsers int
	_ = database.PostgresDB.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM activity_events
		WHERE user_id IS NOT NULL AND created_at >= $1 AND created_at < $2
	`, from, toEnd).Scan(&totalActiveUsers)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":             true,
		"from":                from.Format("2006-01-02"),
		"to":                  to.Format("2006-01-02"),
		"new_users_per_day":   newUsersPerDay,
		"active_users_per_day": activeUsersPerDay,
		"recurring_users_count": recurringCount,
		"top_pages":           topPages,
		"total_new_users":     totalNewUsers,
		"total_active_users":  totalActiveUsers,
	})
}
