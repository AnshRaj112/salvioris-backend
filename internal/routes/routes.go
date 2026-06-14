package routes

import (
	"github.com/AnshRaj112/serenify-backend/internal/handlers"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
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
	r.Delete("/api/therapist/referrals/{id}", handlers.DeleteReferralCode)
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
	r.Delete("/api/admin/therapists", handlers.DeleteTherapist)
	r.Delete("/api/admin/therapists/{id}", handlers.DeleteTherapist)
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

	// V2 P0: JWT refresh + tenant-scoped patient APIs
	r.Post("/api/v1/auth/refresh", handlers.RefreshTokenV2)
	r.Route("/api/v1/tenant/{tenantId}", func(r chi.Router) {
		r.Use(middleware.TenantAuth)
		r.Get("/patients", handlers.ListPatientsV2)
		r.Post("/patients", handlers.CreatePatientV2)
		r.Get("/patients/{patientId}", handlers.GetPatientV2)
		r.Patch("/patients/{patientId}", handlers.UpdatePatientV2)
		r.Delete("/patients/{patientId}", handlers.DeletePatientV2)

		// P1: Session notes
		r.Get("/patients/{patientId}/notes", handlers.ListSessionNotesV2)
		r.Post("/patients/{patientId}/notes", handlers.CreateSessionNoteV2)
		r.Get("/patients/{patientId}/notes/{noteId}", handlers.GetSessionNoteV2)
		r.Patch("/patients/{patientId}/notes/{noteId}", handlers.UpdateSessionNoteV2)
		r.Post("/patients/{patientId}/notes/{noteId}/publish", handlers.PublishSessionNoteV2)
		r.Get("/patients/{patientId}/notes/{noteId}/versions", handlers.ListSessionNoteVersionsV2)
		r.Get("/notes/search", handlers.SearchSessionNotesV2)

		// P1: Wellness (therapist view)
		r.Get("/patients/{patientId}/wellness", handlers.ListPatientWellnessV2)
		r.Get("/patients/{patientId}/wellness/trends", handlers.WellnessTrendsV2)

		// P1: Journals (therapist view + comments)
		r.Get("/patients/{patientId}/journals", handlers.ListPatientJournalsV2)
		r.Post("/patients/{patientId}/journals/{journalId}/comments", handlers.CommentOnJournalV2)

		// P2: Appointments
		r.Get("/appointments", handlers.ListAppointmentsV2)
		r.Post("/appointments", handlers.CreateAppointmentV2)
		r.Get("/appointments/{appointmentId}", handlers.GetAppointmentV2)
		r.Patch("/appointments/{appointmentId}", handlers.UpdateAppointmentV2)
		r.Post("/appointments/{appointmentId}/cancel", handlers.CancelAppointmentV2)

		// P2: Availability
		r.Get("/availability", handlers.ListAvailabilityV2)
		r.Post("/availability", handlers.CreateAvailabilityV2)
		r.Delete("/availability/{slotId}", handlers.DeleteAvailabilityV2)
		r.Get("/availability/open-slots", handlers.GetOpenSlotsV2)

		// P2: Reception
		r.Post("/reception/walk-in", handlers.ReceptionWalkInV2)
		r.Post("/reception/quick-register", handlers.ReceptionQuickRegisterV2)

		// P2: Google Calendar
		r.Get("/calendar/status", handlers.CalendarStatusV2)
		r.Get("/calendar/connect/google", handlers.ConnectGoogleCalendarV2)
		r.Delete("/calendar/disconnect", handlers.DisconnectGoogleCalendarV2)

		// P3: Prescriptions
		r.Get("/patients/{patientId}/prescriptions", handlers.ListPrescriptionsV2)
		r.Post("/patients/{patientId}/prescriptions", handlers.CreatePrescriptionV2)
		r.Patch("/prescriptions/{rxId}", handlers.UpdatePrescriptionV2)

		// P3: Tasks
		r.Get("/patients/{patientId}/tasks", handlers.ListPatientTasksV2)
		r.Post("/patients/{patientId}/tasks", handlers.CreateTaskV2)
		r.Patch("/tasks/{taskId}", handlers.UpdateTaskV2)

		// P3: Messaging
		r.Get("/conversations", handlers.ListConversationsV2)
		r.Get("/patients/{patientId}/conversation", handlers.GetOrCreatePatientConversationV2)
		r.Get("/conversations/{conversationId}/messages", handlers.ListConversationMessagesV2)
		r.Post("/conversations/{conversationId}/messages", handlers.SendConversationMessageV2)
		r.Patch("/conversations/{conversationId}/read", handlers.MarkConversationReadV2)

		// P4: Billing
		r.Get("/billing/profile", handlers.GetBillingProfileV2)
		r.Patch("/billing/profile", handlers.UpdateBillingProfileV2)
		r.Get("/invoices", handlers.ListInvoicesV2)
		r.Post("/invoices", handlers.CreateInvoiceV2)
		r.Get("/invoices/{invoiceId}", handlers.GetInvoiceV2)
		r.Post("/invoices/{invoiceId}/send", handlers.SendInvoiceV2)
		r.Get("/invoices/{invoiceId}/pdf", handlers.GenerateInvoicePDFV2)
		r.Post("/payments/initiate", handlers.InitiatePaymentV2)
		r.Post("/payments/verify", handlers.VerifyPaymentV2)
		r.Get("/payments", handlers.ListPaymentsV2)
		r.Post("/reception/collect-payment", handlers.ReceptionCollectPaymentV2)

		// P5: Analytics
		r.Get("/analytics/overview", handlers.AnalyticsOverviewV2)
		r.Get("/analytics/revenue", handlers.AnalyticsRevenueV2)
		r.Get("/analytics/appointments", handlers.AnalyticsAppointmentsV2)
		r.Get("/analytics/wellness-trends", handlers.AnalyticsWellnessTrendsV2)

		// P5: AI Copilot (rule-based insights)
		r.Post("/ai/summarize-session", handlers.AISummarizeSessionV2)
		r.Get("/patients/{patientId}/ai/progress", handlers.AIPatientProgressV2)
		r.Get("/patients/{patientId}/ai/mood-analysis", handlers.AIMoodAnalysisV2)
		r.Get("/ai/risk-alerts", handlers.AIRiskAlertsV2)
	})

	// P2: Google OAuth callback (no tenant prefix)
	r.Get("/api/v1/calendar/oauth/callback", handlers.GoogleCalendarCallback)

	// P3: 1:1 DM WebSocket
	r.Get("/ws/v1/tenant/{tenantId}/dm", handlers.DMWebSocket)

	// P4: Razorpay webhook (no auth)
	r.Post("/api/v1/webhooks/razorpay", handlers.RazorpayWebhookV2)

	// P1: Patient self-service
	r.Route("/api/v1/patient/me", func(r chi.Router) {
		r.Use(middleware.PatientAuth)
		r.Post("/wellness", handlers.CreateWellnessV2)
		r.Get("/wellness", handlers.ListMyWellnessV2)
		r.Post("/journals", handlers.CreateJournalV2)
		r.Get("/journals", handlers.ListMyJournalsV2)
		r.Get("/appointments", handlers.ListMyAppointmentsV2)
		r.Get("/prescriptions", handlers.ListMyPrescriptionsV2)
		r.Get("/tasks", handlers.ListMyTasksV2)
		r.Post("/tasks/{taskId}/complete", handlers.CompleteTaskV2)
		r.Get("/conversation", handlers.GetMyConversationV2)
		r.Get("/conversation/messages", handlers.ListMyMessagesV2)
		r.Post("/conversation/messages", handlers.SendMyMessageV2)
		r.Patch("/conversation/read", handlers.MarkMyConversationReadV2)
		r.Get("/invoices", handlers.ListMyInvoicesV2)
		r.Post("/invoices/{invoiceId}/pay", handlers.PayMyInvoiceV2)
		r.Post("/payments/verify", handlers.VerifyPatientPaymentV2)

		// Direct Booking & Availability check
		r.Get("/therapists/{therapistId}/availability", handlers.GetTherapistAvailabilityForPatientV2)
		r.Post("/booking/initiate", handlers.InitiateBookingV2)
		r.Post("/booking/verify", handlers.VerifyBookingPaymentV2)
	})
}
