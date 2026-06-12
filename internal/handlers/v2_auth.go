package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/AnshRaj112/serenify-backend/internal/services"
)

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func RefreshTokenV2(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		http.Error(w, "refresh_token is required", http.StatusBadRequest)
		return
	}

	pair, err := services.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}
