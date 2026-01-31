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
}

