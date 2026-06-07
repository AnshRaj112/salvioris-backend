package routes

import (
	"github.com/AnshRaj112/serenify-backend/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(r *chi.Mux) {
	// Privacy-first auth routes
	r.Post("/api/auth/signup", handlers.PrivacySignup)
	r.Post("/api/auth/signin", handlers.PrivacySignin)
	r.Get("/api/auth/me", handlers.GetMe)
	r.Post("/api/auth/check-username", handlers.CheckUsernameAvailability)
	r.Post("/api/auth/forgot-username", handlers.ForgotUsername)
	r.Post("/api/auth/forgot-password", handlers.ForgotPassword)
	r.Post("/api/auth/reset-password", handlers.ResetPassword)
	
	// Legacy auth routes (for backward compatibility - can be removed later)
	r.Post("/api/auth/user/signup", handlers.UserSignup)
	r.Post("/api/auth/user/signin", handlers.UserSignin)
	r.Post("/api/auth/therapist/signup", handlers.TherapistSignup)
	r.Post("/api/auth/therapist/signin", handlers.TherapistSignin)
	
	// Therapist status routes
	r.Get("/api/therapist/status", handlers.CheckTherapistStatus)
	r.Get("/api/therapist", handlers.GetTherapistByID)
	r.Get("/api/therapist/me", handlers.GetTherapistMe)
	r.Put("/api/therapist/profile", handlers.UpdateTherapistProfile)

	// Therapist referral code system (Flow 1)
	r.Post("/api/therapist/referrals", handlers.GenerateReferralCode)
	r.Get("/api/therapist/referrals", handlers.ListReferralCodes)
	r.Put("/api/therapist/referrals/{id}/revoke", handlers.RevokeReferralCode)
	r.Get("/api/therapist/referrals/analytics", handlers.GetReferralAnalytics)

	// Therapist connection & dashboard system (Flow 3 / Relationship management)
	r.Get("/api/therapist/connections", handlers.GetConnectedUsers)
	r.Get("/api/therapist/connection-requests", handlers.GetPendingRequests)
	r.Put("/api/therapist/connection-requests/{id}/respond", handlers.RespondToRequest)
	r.Delete("/api/therapist/connections/{userId}", handlers.DisconnectUser)

	// Therapist patient onboarding
	r.Post("/api/therapist/onboard-patient", handlers.OnboardPatient)
	r.Get("/api/therapist/onboarded-patients", handlers.ListOnboardedPatients)
	r.Delete("/api/therapist/onboarded-patients/{id}", handlers.RemoveOnboardedPatient)

	// User-driven therapist search, profile viewing, and connection request (Flow 2 / Flow 3)
	r.Get("/api/auth/validate-referral", handlers.ValidateReferralCode)
	r.Get("/api/therapists", handlers.SearchTherapists)
	r.Get("/api/therapists/{id}", handlers.GetTherapistProfile)
	r.Post("/api/therapists/{id}/connect", handlers.RequestConnection)
	r.Delete("/api/therapists/{id}/disconnect", handlers.UserDisconnectTherapist)

	// Notification services
	r.Get("/api/notifications", handlers.GetNotifications)
	r.Put("/api/notifications/{id}/read", handlers.MarkNotificationRead)
	
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
	r.Get("/api/admin/users", handlers.GetUsers)
	r.Delete("/api/admin/users", handlers.DeleteUser)
	r.Get("/api/admin/groups", handlers.AdminGetAllGroups)
	r.Get("/api/admin/groups/members", handlers.AdminGetGroupMembers)
	r.Delete("/api/admin/groups", handlers.AdminDeleteGroup)
	r.Get("/api/admin/insights", handlers.GetInsights)
	r.Get("/api/admin/reports", handlers.GetAbuseReports)
	r.Post("/api/admin/groups/block", handlers.AdminBlockGroupMember)

	// Activity tracking (for analytics; optional auth)
	r.Post("/api/activity", handlers.RecordActivity)

	// Vent routes
	r.Post("/api/vent", handlers.CreateVent)
	r.Get("/api/vent", handlers.GetVents)
	
	// Feedback routes
	r.Post("/api/feedback", handlers.SubmitFeedback)
	r.Get("/api/admin/feedbacks", handlers.GetFeedbacks)
	r.Delete("/api/admin/feedbacks", handlers.DeleteFeedback)

	// Journaling routes
	r.Post("/api/journals", handlers.CreateJournal)
	r.Get("/api/journals", handlers.GetJournals)
	
	// Contact us routes
	r.Post("/api/contact", handlers.SubmitContact)
	r.Get("/api/admin/contacts", handlers.GetContacts)
	r.Delete("/api/admin/contacts", handlers.DeleteContact)
	
	// Waitlist routes
	r.Post("/api/waitlist/user", handlers.SubmitUserWaitlist)
	r.Post("/api/waitlist/therapist", handlers.SubmitTherapistWaitlist)
	r.Get("/api/admin/waitlist/user", handlers.GetUserWaitlist)
	r.Get("/api/admin/waitlist/therapist", handlers.GetTherapistWaitlist)
	r.Delete("/api/admin/waitlist/user", handlers.DeleteUserWaitlistEntry)
	r.Delete("/api/admin/waitlist/therapist", handlers.DeleteTherapistWaitlistEntry)
	
	// Admin auth routes (signup removed - admin accounts must be created directly in database)
	// r.Post("/api/admin/signup", handlers.AdminSignup) // Disabled - use database directly
	r.Post("/api/admin/signin", handlers.AdminSignin)
	
	// Group community routes (Telegram-style community system)
	r.Post("/api/groups", handlers.CreateGroup)
	r.Get("/api/groups", handlers.GetGroups)
	r.Put("/api/groups", handlers.UpdateGroup)
	r.Delete("/api/groups", handlers.DeleteGroup)
	r.Post("/api/groups/join", handlers.JoinGroup)
	r.Delete("/api/groups/member", handlers.RemoveMember)
	r.Get("/api/groups/members", handlers.GetGroupMembers)

	// Realtime chat API (MongoDB history + Redis Pub/Sub)
	r.Get("/api/chat/history", handlers.LoadChatHistory)

	// Abuse & crisis report governed disclosure routes
	r.Get("/api/reports/escrow-key", handlers.GetEscrowPublicKey)
	r.Post("/api/reports/submit", handlers.SubmitAbuseReport)
	r.Post("/api/reports/review/{id}", handlers.ReviewAbuseReport)

	// WebSocket endpoint for realtime group chat (Discord-style gateway)
	r.Get("/ws/chat", handlers.ChatWebSocket)
}

