package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func AISummarizeSessionV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	var req struct {
		NoteID    string `json:"note_id"`
		PatientID string `json:"patient_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NoteID == "" {
		http.Error(w, "note_id required", http.StatusBadRequest)
		return
	}

	plain, rating, err := services.GetSessionNotePlainText(req.NoteID)
	if err != nil {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}
	result := services.SummarizeSessionNote(plain, rating)
	if services.LLMEnabled() && plain != "" {
		if summary, err := services.SummarizeWithLLM("Summarize this therapy session note in 3-4 sentences. Do not diagnose.\n\n" + plain); err == nil {
			result.Summary = summary
			result.Insights = append(result.Insights, "Enhanced with AI summarization")
		}
	}
	if req.PatientID != "" {
		if pid, e := uuid.Parse(req.PatientID); e == nil {
			services.CacheAIInsight(tenantID, pid, "session_summary", result)
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": result})
}

func AIPatientProgressV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}

	trends := services.FetchWellnessTrendsForAI(tenantID, patientID, 30)
	sessions := services.CountPublishedNotes(tenantID, patientID)
	result := services.BuildPatientProgress(tenantID, patientID, sessions, trends)
	services.CacheAIInsight(tenantID, patientID, "progress_summary", result)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": result})
}

func AIMoodAnalysisV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}

	trends := services.FetchWellnessTrendsForAI(tenantID, patientID, 30)
	insights := []string{}
	if n, ok := trends["entries_count"].(int); ok {
		insights = append(insights, "Wellness entries logged: "+servicesItoa(n))
	}
	result := services.AIInsightResult{
		Disclaimer: services.AIDisclaimer,
		Summary:    "Mood and wellness trend analysis for the last 30 days.",
		Insights:   insights,
		RiskAlerts: strSlice(trends["risk_indicators"]),
		Data:       trends,
	}
	services.CacheAIInsight(tenantID, patientID, "mood_analysis", result)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": result})
}

func AIRiskAlertsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	alerts, err := services.ListRiskAlerts(tenantID)
	if err != nil {
		http.Error(w, "Failed to load alerts", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": alerts, "disclaimer": services.AIDisclaimer,
	})
}

func servicesItoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func strSlice(v interface{}) []string {
	if s, ok := v.([]string); ok {
		return s
	}
	return nil
}
