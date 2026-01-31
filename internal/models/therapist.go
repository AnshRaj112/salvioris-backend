package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Therapist struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`

	Name     string `bson:"name" json:"name"`
	Email    string `bson:"email" json:"email"`
	Password string `bson:"password" json:"-"` // Don't return password in JSON

	// License and professional info
	LicenseNumber      string `bson:"license_number" json:"license_number"`
	LicenseState       string `bson:"license_state" json:"license_state"`
	YearsOfExperience  int    `bson:"years_of_experience" json:"years_of_experience"`
	Specialization     string `bson:"specialization,omitempty" json:"specialization,omitempty"`
	Phone              string `bson:"phone" json:"phone"`

	// Education
	CollegeDegree      string `bson:"college_degree" json:"college_degree"`
	MastersInstitution string `bson:"masters_institution" json:"masters_institution"`
	PsychologistType   string `bson:"psychologist_type" json:"psychologist_type"`

	// Professional details
	SuccessfulCases int    `bson:"successful_cases" json:"successful_cases"`
	DSMAwareness    string `bson:"dsm_awareness" json:"dsm_awareness"`
	TherapyTypes    string `bson:"therapy_types" json:"therapy_types"`

	// File paths (stored as strings, files uploaded separately)
	CertificateImagePath string `bson:"certificate_image_path,omitempty" json:"certificate_image_path,omitempty"`
	DegreeImagePath      string `bson:"degree_image_path,omitempty" json:"degree_image_path,omitempty"`

	// Approval status
	IsApproved bool `bson:"is_approved" json:"is_approved"`
}

