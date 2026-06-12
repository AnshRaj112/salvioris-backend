package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SessionNote struct {
	ID                      primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID                string             `bson:"tenant_id" json:"tenant_id"`
	PatientID               string             `bson:"patient_id" json:"patient_id"`
	TherapistID             string             `bson:"therapist_id" json:"therapist_id"`
	SessionNumber           int                `bson:"session_number" json:"session_number"`
	AppointmentID           string             `bson:"appointment_id,omitempty" json:"appointment_id,omitempty"`
	Status                  string             `bson:"status" json:"status"`
	SessionDate             time.Time          `bson:"session_date" json:"session_date"`
	PatientSnapshot         map[string]string  `bson:"patient_snapshot,omitempty" json:"patient_snapshot,omitempty"`
	TherapistSnapshot       map[string]string  `bson:"therapist_snapshot,omitempty" json:"therapist_snapshot,omitempty"`
	Content                 interface{}        `bson:"content,omitempty" json:"content,omitempty"`
	PlainText               string             `bson:"plain_text,omitempty" json:"plain_text,omitempty"`
	FollowUpRecommendations string             `bson:"follow_up_recommendations,omitempty" json:"follow_up_recommendations,omitempty"`
	ProgressRating          int                `bson:"progress_rating,omitempty" json:"progress_rating,omitempty"`
	Attachments             []string           `bson:"attachments,omitempty" json:"attachments,omitempty"`
	CreatedAt               time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt               time.Time          `bson:"updated_at" json:"updated_at"`
	PublishedAt             *time.Time         `bson:"published_at,omitempty" json:"published_at,omitempty"`
}

type SessionNoteVersion struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	NoteID        string             `bson:"note_id" json:"note_id"`
	TenantID      string             `bson:"tenant_id" json:"tenant_id"`
	Version       int                `bson:"version" json:"version"`
	Content       interface{}        `bson:"content,omitempty" json:"content,omitempty"`
	PlainText     string             `bson:"plain_text,omitempty" json:"plain_text,omitempty"`
	ChangedBy     string             `bson:"changed_by" json:"changed_by"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
}

type WellnessMetrics struct {
	Mood                  *int   `bson:"mood,omitempty" json:"mood,omitempty"`
	Anxiety               *int   `bson:"anxiety,omitempty" json:"anxiety,omitempty"`
	Stress                *int   `bson:"stress,omitempty" json:"stress,omitempty"`
	SleepHours            *float64 `bson:"sleep_hours,omitempty" json:"sleep_hours,omitempty"`
	SleepQuality          *int   `bson:"sleep_quality,omitempty" json:"sleep_quality,omitempty"`
	Energy                *int   `bson:"energy,omitempty" json:"energy,omitempty"`
	Focus                 *int   `bson:"focus,omitempty" json:"focus,omitempty"`
	MedicationAdherence   *bool  `bson:"medication_adherence,omitempty" json:"medication_adherence,omitempty"`
	MedicationNotes       string `bson:"medication_notes,omitempty" json:"medication_notes,omitempty"`
}

type WellnessEntry struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID   string             `bson:"tenant_id" json:"tenant_id"`
	PatientID  string             `bson:"patient_id" json:"patient_id"`
	EntryDate  time.Time          `bson:"entry_date" json:"entry_date"`
	Metrics    WellnessMetrics    `bson:"metrics" json:"metrics"`
	Reflection string             `bson:"reflection,omitempty" json:"reflection,omitempty"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time          `bson:"updated_at" json:"updated_at"`
}

type TherapistComment struct {
	TherapistID string    `bson:"therapist_id" json:"therapist_id"`
	Comment     string    `bson:"comment" json:"comment"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
}

type PatientJournal struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID           string             `bson:"tenant_id" json:"tenant_id"`
	PatientID          string             `bson:"patient_id" json:"patient_id"`
	UserID             string             `bson:"user_id" json:"user_id"`
	Title              string             `bson:"title" json:"title"`
	Content            string             `bson:"content" json:"content"`
	MoodTag            string             `bson:"mood_tag,omitempty" json:"mood_tag,omitempty"`
	IsPrivate          bool               `bson:"is_private" json:"is_private"`
	TherapistComments  []TherapistComment `bson:"therapist_comments,omitempty" json:"therapist_comments,omitempty"`
	CreatedAt          time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt          time.Time          `bson:"updated_at" json:"updated_at"`
}
