package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
)

func AnalyticsOverviewV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	var activePatients, sessionsMonth, appointmentsWeek int
	var revenueMonth sql.NullFloat64

	_ = database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM patients WHERE tenant_id = $1 AND deleted_at IS NULL AND status = 'active'
	`, tenantID).Scan(&activePatients)

	monthStart := time.Now().AddDate(0, -1, 0)
	_ = database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM appointments
		WHERE tenant_id = $1 AND therapist_id = $2 AND status = 'completed' AND starts_at >= $3
	`, tenantID, therapistID, monthStart).Scan(&sessionsMonth)

	weekStart := time.Now().AddDate(0, 0, -7)
	_ = database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM appointments
		WHERE tenant_id = $1 AND starts_at >= $2 AND status NOT IN ('cancelled')
	`, tenantID, weekStart).Scan(&appointmentsWeek)

	_ = database.PostgresDB.QueryRow(`
		SELECT COALESCE(SUM(total), 0) FROM invoices
		WHERE tenant_id = $1 AND status = 'paid' AND paid_at >= $2
	`, tenantID, monthStart).Scan(&revenueMonth)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"active_patients":      activePatients,
			"sessions_completed_month": sessionsMonth,
			"appointments_upcoming_week": appointmentsWeek,
			"revenue_month":          revenueMonth.Float64,
			"currency":               "INR",
		},
	})
}

func AnalyticsRevenueV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	period := r.URL.Query().Get("period")
	days := 30
	if period == "weekly" {
		days = 7
	}
	from := time.Now().AddDate(0, 0, -days)

	rows, err := database.PostgresDB.Query(`
		SELECT DATE(paid_at) as d, SUM(total) as amount, COUNT(*) as count
		FROM invoices
		WHERE tenant_id = $1 AND status = 'paid' AND paid_at >= $2
		GROUP BY DATE(paid_at) ORDER BY d
	`, tenantID, from)
	if err != nil {
		http.Error(w, "Failed to load revenue", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	series := make([]map[string]interface{}, 0)
	for rows.Next() {
		var d time.Time
		var amount float64
		var count int
		_ = rows.Scan(&d, &amount, &count)
		series = append(series, map[string]interface{}{
			"date": d.Format("2006-01-02"), "amount": amount, "count": count,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": series, "period_days": days})
}

func AnalyticsAppointmentsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	rows, err := database.PostgresDB.Query(`
		SELECT status, COUNT(*) FROM appointments
		WHERE tenant_id = $1 AND starts_at >= $2
		GROUP BY status
	`, tenantID, time.Now().AddDate(0, -1, 0))
	if err != nil {
		http.Error(w, "Failed to load stats", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	byStatus := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		_ = rows.Scan(&status, &n)
		byStatus[status] = n
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": byStatus})
}

func AnalyticsWellnessTrendsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())

	rows, err := database.PostgresDB.Query(`
		SELECT id, full_name FROM patients WHERE tenant_id = $1 AND deleted_at IS NULL LIMIT 50
	`, tenantID)
	if err != nil {
		http.Error(w, "Failed to load patients", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	summaries := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id uuid.UUID
		var name string
		_ = rows.Scan(&id, &name)
		pid := id.String()
		trends := services.FetchWellnessTrendsForAI(tenantID, id, 30)
		summaries = append(summaries, map[string]interface{}{
			"patient_id": pid, "patient_name": name, "trends": trends,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": summaries})
}
