package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type taskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	DueAt       string `json:"due_at,omitempty"`
	ReminderAt  string `json:"reminder_at,omitempty"`
}

type completeTaskRequest struct {
	PatientNotes string `json:"patient_notes,omitempty"`
}

func ListPatientTasksV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}
	listTasks(w, tenantID, patientID, r.URL.Query().Get("status"))
}

func CreateTaskV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}

	var req taskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	dueAt, _ := parseOptionalTime(req.DueAt)
	reminderAt, _ := parseOptionalTime(req.ReminderAt)

	var id uuid.UUID
	err := database.PostgresDB.QueryRow(`
		INSERT INTO tasks (tenant_id, patient_id, assigned_by, title, description, category, due_at, reminder_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id
	`, tenantID, patientID, therapistID, req.Title, nullStr(req.Description),
		nullStr(req.Category), dueAt, reminderAt).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	task, _ := getTask(tenantID, id)
	services.NotifyPatientByID(patientID, "New task assigned", req.Title, "task_assigned")
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": task})
}

func UpdateTaskV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	taskID, ok := parsePatientIDParam(chi.URLParam(r, "taskId"))
	if !ok {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var body struct {
		taskRequest
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	dueAt, _ := parseOptionalTime(body.DueAt)
	_, err := database.PostgresDB.Exec(`
		UPDATE tasks SET
			title = COALESCE(NULLIF($3,''), title),
			description = COALESCE(NULLIF($4,''), description),
			category = COALESCE(NULLIF($5,''), category),
			due_at = COALESCE($6, due_at),
			status = COALESCE(NULLIF($7,''), status),
			updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, taskID, tenantID, body.Title, body.Description, body.Category, dueAt, body.Status)
	if err != nil {
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	task, err := getTask(tenantID, taskID)
	if err == sql.ErrNoRows {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": task})
}

func ListMyTasksV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	listTasks(w, tenantID, patientID, r.URL.Query().Get("status"))
}

func CompleteTaskV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	taskID, ok := parsePatientIDParam(chi.URLParam(r, "taskId"))
	if !ok {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var req completeTaskRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	res, err := database.PostgresDB.Exec(`
		UPDATE tasks SET status = 'completed', completed_at = NOW(),
			patient_notes = COALESCE(NULLIF($4,''), patient_notes), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND patient_id = $3 AND status != 'completed'
	`, taskID, tenantID, patientID, strings.TrimSpace(req.PatientNotes))
	if err != nil {
		http.Error(w, "Failed to complete task", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	task, _ := getTask(tenantID, taskID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": task})
}

func listTasks(w http.ResponseWriter, tenantID, patientID uuid.UUID, status string) {
	query := `
		SELECT id, tenant_id, patient_id, assigned_by, title, description, category,
			due_at, reminder_at, status, completed_at, patient_notes, created_at, updated_at
		FROM tasks WHERE tenant_id = $1 AND patient_id = $2
	`
	args := []interface{}{tenantID, patientID}
	if status != "" {
		query += ` AND status = $3`
		args = append(args, status)
	}
	query += ` ORDER BY due_at NULLS LAST, created_at DESC`

	rows, err := database.PostgresDB.Query(query, args...)
	if err != nil {
		http.Error(w, "Failed to list tasks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := make([]models.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			http.Error(w, "Failed to read tasks", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, t)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": tasks})
}

func getTask(tenantID, id uuid.UUID) (models.Task, error) {
	row := database.PostgresDB.QueryRow(`
		SELECT id, tenant_id, patient_id, assigned_by, title, description, category,
			due_at, reminder_at, status, completed_at, patient_notes, created_at, updated_at
		FROM tasks WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	return scanTaskRow(row)
}

func scanTask(rows *sql.Rows) (models.Task, error) {
	var t models.Task
	var desc, cat, notes sql.NullString
	var due, reminder, completed sql.NullTime
	err := rows.Scan(&t.ID, &t.TenantID, &t.PatientID, &t.AssignedBy, &t.Title,
		&desc, &cat, &due, &reminder, &t.Status, &completed, &notes, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	t.Description = desc.String
	t.Category = cat.String
	t.PatientNotes = notes.String
	if due.Valid {
		v := due.Time
		t.DueAt = &v
	}
	if reminder.Valid {
		v := reminder.Time
		t.ReminderAt = &v
	}
	if completed.Valid {
		v := completed.Time
		t.CompletedAt = &v
	}
	return t, nil
}

func scanTaskRow(row *sql.Row) (models.Task, error) {
	var t models.Task
	var desc, cat, notes sql.NullString
	var due, reminder, completed sql.NullTime
	err := row.Scan(&t.ID, &t.TenantID, &t.PatientID, &t.AssignedBy, &t.Title,
		&desc, &cat, &due, &reminder, &t.Status, &completed, &notes, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	t.Description = desc.String
	t.Category = cat.String
	t.PatientNotes = notes.String
	if due.Valid {
		v := due.Time
		t.DueAt = &v
	}
	if reminder.Valid {
		v := reminder.Time
		t.ReminderAt = &v
	}
	if completed.Valid {
		v := completed.Time
		t.CompletedAt = &v
	}
	return t, nil
}

func parseOptionalTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
