package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const AIDisclaimer = "AI-assisted insight. Not a medical diagnosis."

type AIInsightResult struct {
	Disclaimer string                 `json:"disclaimer"`
	Summary    string                 `json:"summary"`
	Insights   []string               `json:"insights"`
	RiskAlerts []string               `json:"risk_alerts,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

func CacheAIInsight(tenantID, patientID uuid.UUID, insightType string, output AIInsightResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = database.DB.Collection("ai_insights").InsertOne(ctx, bson.M{
		"tenant_id":  tenantID.String(),
		"patient_id": patientID.String(),
		"type":       insightType,
		"output":     output,
		"created_at": time.Now(),
		"expires_at": time.Now().Add(24 * time.Hour),
	})
}

func SummarizeSessionNote(plainText string, progressRating int) AIInsightResult {
	text := strings.TrimSpace(plainText)
	summary := text
	if len(summary) > 280 {
		summary = summary[:280] + "..."
	}
	if summary == "" {
		summary = "No session content to summarize yet."
	}
	insights := []string{}
	if progressRating > 0 {
		insights = append(insights, fmt.Sprintf("Progress rating recorded: %d/10", progressRating))
	}
	if strings.Contains(strings.ToLower(text), "anxiety") {
		insights = append(insights, "Session mentions anxiety — consider reviewing wellness trends.")
	}
	return AIInsightResult{
		Disclaimer: AIDisclaimer,
		Summary:    summary,
		Insights:   insights,
	}
}

func BuildPatientProgress(tenantID, patientID uuid.UUID, sessionCount int, trends map[string]interface{}) AIInsightResult {
	insights := []string{
		fmt.Sprintf("Total published sessions: %d", sessionCount),
	}
	risks := []string{}
	if v, ok := trends["risk_indicators"].([]string); ok {
		risks = v
	}
	if avg, ok := trends["avg_mood"].(float64); ok {
		insights = append(insights, fmt.Sprintf("Average mood (recent period): %.1f", avg))
	}
	return AIInsightResult{
		Disclaimer: AIDisclaimer,
		Summary:    "Patient progress snapshot based on session history and wellness data.",
		Insights:   insights,
		RiskAlerts: risks,
		Data:       trends,
	}
}

func FetchWellnessTrendsForAI(tenantID, patientID uuid.UUID, days int) map[string]interface{} {
	from := time.Now().AddDate(0, 0, -days+1)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cursor, _ := database.DB.Collection("wellness_entries").Find(ctx, bson.M{
		"tenant_id": tenantID.String(), "patient_id": patientID.String(),
		"entry_date": bson.M{"$gte": from},
	}, options.Find().SetSort(bson.D{{Key: "entry_date", Value: 1}}))

	type entry struct {
		Metrics struct {
			Mood    *int `bson:"mood"`
			Anxiety *int `bson:"anxiety"`
		} `bson:"metrics"`
	}
	var entries []entry
	_ = cursor.All(ctx, &entries)

	moodSum, moodN, anxSum, anxN := 0.0, 0, 0.0, 0
	for _, e := range entries {
		if e.Metrics.Mood != nil {
			moodSum += float64(*e.Metrics.Mood)
			moodN++
		}
		if e.Metrics.Anxiety != nil {
			anxSum += float64(*e.Metrics.Anxiety)
			anxN++
		}
	}
	trends := map[string]interface{}{"entries_count": len(entries), "period_days": days}
	risks := []string{}
	if moodN > 0 {
		avg := moodSum / float64(moodN)
		trends["avg_mood"] = avg
		if avg <= 3 {
			risks = append(risks, "low_mood")
		}
	}
	if anxN > 0 {
		avg := anxSum / float64(anxN)
		trends["avg_anxiety"] = avg
		if avg >= 7 {
			risks = append(risks, "elevated_anxiety")
		}
	}
	trends["risk_indicators"] = risks
	return trends
}

func ListRiskAlerts(tenantID uuid.UUID) ([]map[string]interface{}, error) {
	patients, err := database.PostgresDB.Query(`
		SELECT id, full_name FROM patients WHERE tenant_id = $1 AND deleted_at IS NULL AND status = 'active'
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer patients.Close()

	alerts := make([]map[string]interface{}, 0)
	for patients.Next() {
		var pid uuid.UUID
		var name string
		_ = patients.Scan(&pid, &name)
		trends := FetchWellnessTrendsForAI(tenantID, pid, 7)
		if risks, ok := trends["risk_indicators"].([]string); ok && len(risks) > 0 {
			alerts = append(alerts, map[string]interface{}{
				"patient_id": pid.String(), "patient_name": name,
				"risk_indicators": risks, "period_days": 7,
			})
		}
	}
	return alerts, nil
}

func CountPublishedNotes(tenantID, patientID uuid.UUID) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, _ := database.DB.Collection("session_notes").CountDocuments(ctx, bson.M{
		"tenant_id": tenantID.String(), "patient_id": patientID.String(), "status": "published",
	})
	return int(n)
}

func GetSessionNotePlainText(noteID string) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	oid, err := primitive.ObjectIDFromHex(noteID)
	if err != nil {
		return "", 0, err
	}
	var doc struct {
		PlainText       string `bson:"plain_text"`
		ProgressRating  int    `bson:"progress_rating"`
	}
	err = database.DB.Collection("session_notes").FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	return doc.PlainText, doc.ProgressRating, err
}
