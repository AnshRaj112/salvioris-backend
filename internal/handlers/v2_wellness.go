package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type wellnessRequest struct {
	EntryDate  string                `json:"entry_date,omitempty"`
	Metrics    models.WellnessMetrics `json:"metrics"`
	Reflection string                `json:"reflection,omitempty"`
}

func CreateWellnessV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())

	var req wellnessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	entryDate := truncateDate(time.Now())
	if req.EntryDate != "" {
		if t, err := time.Parse("2006-01-02", req.EntryDate); err == nil {
			entryDate = truncateDate(t)
		}
	}

	ctx, cancel := mongoCtx()
	defer cancel()
	now := time.Now()

	entry := models.WellnessEntry{
		TenantID:   tenantID.String(),
		PatientID:  patientID.String(),
		EntryDate:  entryDate,
		Metrics:    req.Metrics,
		Reflection: strings.TrimSpace(req.Reflection),
		UpdatedAt:  now,
	}

	filter := bson.M{
		"tenant_id": tenantID.String(),
		"patient_id": patientID.String(),
		"entry_date": entryDate,
	}
	update := bson.M{
		"$set": bson.M{
			"metrics":    entry.Metrics,
			"reflection": entry.Reflection,
			"updated_at": entry.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"_id":        primitive.NewObjectID(),
			"tenant_id":  entry.TenantID,
			"patient_id": entry.PatientID,
			"entry_date": entry.EntryDate,
			"created_at": now,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var result models.WellnessEntry
	err := database.DB.Collection("wellness_entries").FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		http.Error(w, "Failed to save wellness entry: " + err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": result})
}

func ListMyWellnessV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	listWellness(w, r, tenantID.String(), patientID.String())
}

func ListPatientWellnessV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}
	listWellness(w, r, tenantID.String(), patientID.String())
}

func listWellness(w http.ResponseWriter, r *http.Request, tenantID, patientID string) {
	from, to := parseDateRange(r)
	ctx, cancel := mongoCtx()
	defer cancel()

	filter := bson.M{
		"tenant_id":  tenantID,
		"patient_id": patientID,
		"entry_date": bson.M{"$gte": from, "$lte": to},
	}
	cursor, err := database.DB.Collection("wellness_entries").Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "entry_date", Value: -1}}))
	if err != nil {
		http.Error(w, "Failed to list wellness entries", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var entries []models.WellnessEntry
	_ = cursor.All(ctx, &entries)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": entries})
}

func WellnessTrendsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}

	period := r.URL.Query().Get("period")
	days := 7
	if period == "monthly" {
		days = 30
	}

	from := truncateDate(time.Now().AddDate(0, 0, -days+1))
	to := truncateDate(time.Now())

	ctx, cancel := mongoCtx()
	defer cancel()
	cursor, err := database.DB.Collection("wellness_entries").Find(ctx, bson.M{
		"tenant_id": tenantID.String(), "patient_id": patientID.String(),
		"entry_date": bson.M{"$gte": from, "$lte": to},
	}, options.Find().SetSort(bson.D{{Key: "entry_date", Value: 1}}))
	if err != nil {
		http.Error(w, "Failed to load trends", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var entries []models.WellnessEntry
	_ = cursor.All(ctx, &entries)

	trends := computeWellnessTrends(entries, days)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": trends})
}

func truncateDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func parseDateRange(r *http.Request) (time.Time, time.Time) {
	to := truncateDate(time.Now())
	from := to.AddDate(0, 0, -29)
	if s := r.URL.Query().Get("from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			from = truncateDate(t)
		}
	}
	if s := r.URL.Query().Get("to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			to = truncateDate(t)
		}
	}
	return from, to
}

func computeWellnessTrends(entries []models.WellnessEntry, days int) map[string]interface{} {
	type agg struct{ sum float64; count int }
	mood, anxiety, stress, sleep, energy := agg{}, agg{}, agg{}, agg{}, agg{}

	for _, e := range entries {
		if e.Metrics.Mood != nil {
			mood.sum += float64(*e.Metrics.Mood); mood.count++
		}
		if e.Metrics.Anxiety != nil {
			anxiety.sum += float64(*e.Metrics.Anxiety); anxiety.count++
		}
		if e.Metrics.Stress != nil {
			stress.sum += float64(*e.Metrics.Stress); stress.count++
		}
		if e.Metrics.SleepHours != nil {
			sleep.sum += *e.Metrics.SleepHours; sleep.count++
		}
		if e.Metrics.Energy != nil {
			energy.sum += float64(*e.Metrics.Energy); energy.count++
		}
	}

	avg := func(a agg) *float64 {
		if a.count == 0 {
			return nil
		}
		v := a.sum / float64(a.count)
		return &v
	}

	risk := []string{}
	if anxiety.count > 0 && anxiety.sum/float64(anxiety.count) >= 7 {
		risk = append(risk, "elevated_anxiety")
	}
	if mood.count > 0 && mood.sum/float64(mood.count) <= 3 {
		risk = append(risk, "low_mood")
	}

	return map[string]interface{}{
		"period_days":      days,
		"entries_count":    len(entries),
		"avg_mood":         avg(mood),
		"avg_anxiety":      avg(anxiety),
		"avg_stress":       avg(stress),
		"avg_sleep_hours":  avg(sleep),
		"avg_energy":       avg(energy),
		"risk_indicators":  risk,
	}
}

func ListAllPatientsWellnessV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	from, to := parseDateRange(r)
	ctx, cancel := mongoCtx()
	defer cancel()

	filter := bson.M{
		"tenant_id":  tenantID.String(),
		"entry_date": bson.M{"$gte": from, "$lte": to},
	}
	cursor, err := database.DB.Collection("wellness_entries").Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "entry_date", Value: -1}}))
	if err != nil {
		http.Error(w, "Failed to list wellness entries", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var entries []models.WellnessEntry
	_ = cursor.All(ctx, &entries)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": entries})
}
