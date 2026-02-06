package routes

import (
	"github.com/AnshRaj112/serenify-backend/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(r *chi.Mux) {
	// Auth routes
	r.Post("/api/auth/user/signup", handlers.UserSignup)
	r.Post("/api/auth/user/signin", handlers.UserSignin)
	r.Post("/api/auth/therapist/signup", handlers.TherapistSignup)
	r.Post("/api/auth/therapist/signin", handlers.TherapistSignin)
	
	// Therapist status routes
	r.Get("/api/therapist/status", handlers.CheckTherapistStatus)
	r.Get("/api/therapist", handlers.GetTherapistByID)
	
	// File upload routes
	r.Post("/api/upload", handlers.UploadFile)
	
	// Admin routes
	r.Get("/api/admin/therapists/pending", handlers.GetPendingTherapists)
	r.Get("/api/admin/therapists/approved", handlers.GetApprovedTherapists)
	r.Put("/api/admin/therapists/approve", handlers.ApproveTherapist)
	r.Delete("/api/admin/therapists/reject", handlers.RejectTherapist)
	r.Get("/api/admin/violations", handlers.GetViolations)
	r.Get("/api/admin/blocked-ips", handlers.GetBlockedIPs)
	r.Put("/api/admin/unblock-ip", handlers.UnblockIP)
	
	// Vent routes
	r.Post("/api/vent", handlers.CreateVent)
	r.Get("/api/vent", handlers.GetVents)
	
	// Feedback routes
	r.Post("/api/feedback", handlers.SubmitFeedback)
	r.Get("/api/admin/feedbacks", handlers.GetFeedbacks)
}

