package handlers

import (
	"net/http"

	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/services"
)

func ConnectGoogleCalendarV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	url, err := services.GoogleAuthURL(tenantID, therapistID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"auth_url": url})
}

func GoogleCalendarCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}
	if err := services.HandleGoogleCallback(code, state); err != nil {
		http.Error(w, "OAuth failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<html><body><h2>Google Calendar connected.</h2><p>You can close this window.</p></body></html>"))
}

func DisconnectGoogleCalendarV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	if err := services.DisconnectGoogleCalendar(tenantID, therapistID); err != nil {
		http.Error(w, "Failed to disconnect", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func CalendarStatusV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	connected := false
	if services.GoogleCalendarEnabled() {
		// lightweight check via integration row
		type row struct{}
		_ = tenantID
		connected = services.HasCalendarIntegration(tenantID, therapistID)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"configured": services.GoogleCalendarEnabled(),
		"connected":  connected,
	})
}
