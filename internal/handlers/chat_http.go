package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/services"
)

// LoadChatHistoryResponse is returned when loading historical messages from MongoDB.
type LoadChatHistoryResponse struct {
	Success  bool                 `json:"success"`
	Messages []services.ChatMessage `json:"messages"`
	HasMore  bool                 `json:"has_more"`
}

// LoadChatHistory loads paginated offline messages for a group.
// Query params:
//   group_id (required)
//   before   (optional RFC3339 timestamp for pagination)
//   limit    (optional, default 50)
func LoadChatHistory(w http.ResponseWriter, r *http.Request) {
	groupID := r.URL.Query().Get("group_id")
	if groupID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "group_id is required",
		})
		return
	}

	limit := int64(50)
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if parsed, err := strconv.ParseInt(lStr, 10, 64); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	var before *time.Time
	if bStr := r.URL.Query().Get("before"); bStr != "" {
		if t, err := time.Parse(time.RFC3339, bStr); err == nil {
			before = &t
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	msgs, hasMore, err := services.LoadChatMessages(ctx, groupID, before, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "failed to load messages",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LoadChatHistoryResponse{
		Success:  true,
		Messages: msgs,
		HasMore:  hasMore,
	})
}


