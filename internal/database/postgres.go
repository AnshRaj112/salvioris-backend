package database

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

var PostgresDB *sql.DB

// ConnectPostgres connects to PostgreSQL database
func ConnectPostgres(postgresURI string) error {
	var err error

	PostgresDB, err = sql.Open("postgres", postgresURI)
	if err != nil {
		return err
	}

	// Set connection pool settings
	PostgresDB.SetMaxOpenConns(25)
	PostgresDB.SetMaxIdleConns(5)
	PostgresDB.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err = PostgresDB.Ping(); err != nil {
		return err
	}

	log.Println("✅ Connected to PostgreSQL")

	// Initialize tables
	if err = InitPostgresTables(); err != nil {
		return err
	}

	return nil
}

// InitPostgresTables creates all necessary tables if they don't exist
func InitPostgresTables() error {
	queries := []string{
		// Users table (PRIVACY-FIRST: Public profile data only)
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(20) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			is_active BOOLEAN NOT NULL DEFAULT TRUE
		)`,

		// User recovery table (PRIVATE: Encrypted recovery data)
		`CREATE TABLE IF NOT EXISTS user_recovery (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			email_encrypted TEXT,
			phone_encrypted TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(user_id)
		)`,

		// User devices table (SECURITY: Device tracking for support)
		`CREATE TABLE IF NOT EXISTS user_devices (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			device_token VARCHAR(255) NOT NULL UNIQUE,
			ip_address VARCHAR(255),
			user_agent TEXT,
			last_used TIMESTAMP NOT NULL DEFAULT NOW(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,

		// Therapists table
		`CREATE TABLE IF NOT EXISTS therapists (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			password VARCHAR(255) NOT NULL,
			license_number VARCHAR(255) NOT NULL,
			license_state VARCHAR(255) NOT NULL,
			years_of_experience INTEGER NOT NULL,
			specialization VARCHAR(255),
			phone VARCHAR(50) NOT NULL,
			college_degree VARCHAR(255) NOT NULL,
			masters_institution VARCHAR(255) NOT NULL,
			psychologist_type VARCHAR(255) NOT NULL,
			successful_cases INTEGER NOT NULL,
			dsm_awareness VARCHAR(255) NOT NULL,
			therapy_types VARCHAR(255) NOT NULL,
			certificate_image_path TEXT,
			degree_image_path TEXT,
			is_approved BOOLEAN NOT NULL DEFAULT FALSE
		)`,

		// Violations table
		`CREATE TABLE IF NOT EXISTS violations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			user_id UUID REFERENCES users(id) ON DELETE SET NULL,
			ip_address VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			message TEXT NOT NULL,
			vent_id VARCHAR(255),
			action_taken VARCHAR(50) NOT NULL
		)`,

		// Blocked IPs table
		`CREATE TABLE IF NOT EXISTS blocked_ips (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL,
			ip_address VARCHAR(255) NOT NULL,
			reason TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE
		)`,

		// Password reset tokens table
		`CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token VARCHAR(255) NOT NULL UNIQUE,
			expires_at TIMESTAMP NOT NULL,
			used BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,

		// Feedbacks table
		`CREATE TABLE IF NOT EXISTS feedbacks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			feedback TEXT NOT NULL,
			ip_address VARCHAR(255)
		)`,

		// Contact us table
		`CREATE TABLE IF NOT EXISTS contact_us (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			message TEXT NOT NULL,
			ip_address VARCHAR(255)
		)`,

		// User waitlist table
		`CREATE TABLE IF NOT EXISTS user_waitlist (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			ip_address VARCHAR(255)
		)`,

		// Therapist waitlist table
		`CREATE TABLE IF NOT EXISTS therapist_waitlist (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			phone VARCHAR(50),
			ip_address VARCHAR(255)
		)`,

		// Admins table
		`CREATE TABLE IF NOT EXISTS admins (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			username VARCHAR(50) NOT NULL UNIQUE,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE
		)`,

		// Staff sessions table for MFA status tracking
		`CREATE TABLE IF NOT EXISTS staff_sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			actor_id UUID NOT NULL,
			mfa_verified BOOLEAN NOT NULL DEFAULT FALSE,
			last_mfa_at TIMESTAMP NOT NULL DEFAULT NOW(),
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,

		// Groups table (public community groups)
		// NOTE: name and slug must both be globally unique (case-insensitive for name).
		// slug is used for shareable URLs: /community/group/<slug>
		`CREATE TABLE IF NOT EXISTS groups (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) NOT NULL UNIQUE,
			description TEXT,
			created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			is_public BOOLEAN NOT NULL DEFAULT TRUE,
			member_count INTEGER NOT NULL DEFAULT 1,
			tags TEXT[] DEFAULT '{}'::text[]
		)`,
		// Group members table (many-to-many relationship)
		// role: "admin" | "member"
		`CREATE TABLE IF NOT EXISTS group_members (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role VARCHAR(20) NOT NULL DEFAULT 'member',
			joined_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(group_id, user_id)
		)`,
		// Group messages table
		`CREATE TABLE IF NOT EXISTS group_messages (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			message TEXT NOT NULL
		)`,

		// Create indexes for better performance
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username_lower ON users(LOWER(username))`,
		`CREATE INDEX IF NOT EXISTS idx_user_recovery_user_id ON user_recovery(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_devices_user_id ON user_devices(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_devices_device_token ON user_devices(device_token)`,
		`CREATE INDEX IF NOT EXISTS idx_therapists_email ON therapists(email)`,
		`CREATE INDEX IF NOT EXISTS idx_violations_ip_address ON violations(ip_address)`,
		`CREATE INDEX IF NOT EXISTS idx_violations_created_at ON violations(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_blocked_ips_ip_address ON blocked_ips(ip_address)`,
		`CREATE INDEX IF NOT EXISTS idx_blocked_ips_is_active ON blocked_ips(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON password_reset_tokens(token)`,
		`CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at ON password_reset_tokens(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_feedbacks_created_at ON feedbacks(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_contact_us_created_at ON contact_us(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_contact_us_email ON contact_us(email)`,
		`CREATE INDEX IF NOT EXISTS idx_user_waitlist_created_at ON user_waitlist(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_user_waitlist_email ON user_waitlist(email)`,
		`CREATE INDEX IF NOT EXISTS idx_therapist_waitlist_created_at ON therapist_waitlist(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_therapist_waitlist_email ON therapist_waitlist(email)`,
		`CREATE INDEX IF NOT EXISTS idx_admins_username ON admins(username)`,
		`CREATE INDEX IF NOT EXISTS idx_admins_email ON admins(email)`,
		`CREATE INDEX IF NOT EXISTS idx_groups_created_by ON groups(created_by)`,
		`CREATE INDEX IF NOT EXISTS idx_groups_is_public ON groups(is_public)`,
		`CREATE INDEX IF NOT EXISTS idx_groups_created_at ON groups(created_at)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_name_lower_unique ON groups(LOWER(name))`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_slug_lower_unique ON groups(LOWER(slug))`,
		`CREATE INDEX IF NOT EXISTS idx_group_members_group_id ON group_members(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_messages_group_id ON group_messages(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_messages_created_at ON group_messages(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_group_messages_user_id ON group_messages(user_id)`,

		// Activity events table (for analytics: page views, recurring users)
		`CREATE TABLE IF NOT EXISTS activity_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE SET NULL,
			path VARCHAR(500) NOT NULL,
			event_type VARCHAR(50) NOT NULL DEFAULT 'page_view',
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_events_created_at ON activity_events(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_events_user_id ON activity_events(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_events_user_created ON activity_events(user_id, created_at)`,

		// Abuse reports ledger
		`CREATE TABLE IF NOT EXISTS abuse_reports (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			reported_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			encrypted_payload TEXT NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,

		// Group blocks table
		`CREATE TABLE IF NOT EXISTS group_blocks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			blocked_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(group_id, user_id)
		)`,

		// Security audit logs ledger (Append-only)
		`CREATE TABLE IF NOT EXISTS security_audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			event_type VARCHAR(100) NOT NULL,
			target_id VARCHAR(255) NOT NULL,
			actor_id VARCHAR(255) NOT NULL,
			actor_role VARCHAR(50) NOT NULL DEFAULT 'unknown',
			reason TEXT NOT NULL,
			ip_address VARCHAR(45) NOT NULL,
			user_agent TEXT NOT NULL DEFAULT 'unknown',
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_security_audit_actor ON security_audit_logs(actor_id)`,

		// 1. Referral Codes Table
		`CREATE TABLE IF NOT EXISTS referral_codes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			therapist_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			code VARCHAR(50) NOT NULL UNIQUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP,
			usage_limit INTEGER,
			usage_count INTEGER NOT NULL DEFAULT 0,
			is_revoked BOOLEAN NOT NULL DEFAULT FALSE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_referral_codes_code ON referral_codes(code)`,
		`CREATE INDEX IF NOT EXISTS idx_referral_codes_therapist ON referral_codes(therapist_id)`,

		// 2. Referral Usages Table
		`CREATE TABLE IF NOT EXISTS referral_usages (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			referral_code_id UUID NOT NULL REFERENCES referral_codes(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			used_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(referral_code_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_referral_usages_user ON referral_usages(user_id)`,

		// 3. Therapist-User Connections Table
		`CREATE TABLE IF NOT EXISTS therapist_user_connections (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			therapist_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			connected_at TIMESTAMP NOT NULL DEFAULT NOW(),
			connection_type VARCHAR(50) NOT NULL,
			referral_code_id UUID REFERENCES referral_codes(id) ON DELETE SET NULL,
			UNIQUE(therapist_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_connections_therapist ON therapist_user_connections(therapist_id)`,
		`CREATE INDEX IF NOT EXISTS idx_connections_user ON therapist_user_connections(user_id)`,

		// 4. Connection Requests Table
		`CREATE TABLE IF NOT EXISTS connection_requests (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			therapist_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			note TEXT,
			UNIQUE(user_id, therapist_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_conn_requests_therapist_status ON connection_requests(therapist_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_conn_requests_user ON connection_requests(user_id)`,

		// 5. Consent History Table
		`CREATE TABLE IF NOT EXISTS consent_history (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			therapist_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			action VARCHAR(50) NOT NULL,
			timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
			details TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_consent_history_user ON consent_history(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_consent_history_therapist ON consent_history(therapist_id)`,

		// 6. Notifications Table
		`CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			recipient_id UUID NOT NULL,
			recipient_role VARCHAR(20) NOT NULL,
			title VARCHAR(255) NOT NULL,
			message TEXT NOT NULL,
			type VARCHAR(50) NOT NULL,
			is_read BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			data JSONB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_recipient ON notifications(recipient_id, is_read)`,

		// Therapist-initiated patient onboarding records
		`CREATE TABLE IF NOT EXISTS patient_onboardings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			therapist_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			patient_name VARCHAR(255) NOT NULL,
			patient_email VARCHAR(255) NOT NULL,
			username VARCHAR(20) NOT NULL,
			referral_code_id UUID NOT NULL REFERENCES referral_codes(id) ON DELETE CASCADE,
			onboarded_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(therapist_id, patient_email)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_patient_onboardings_therapist ON patient_onboardings(therapist_id)`,
		`CREATE INDEX IF NOT EXISTS idx_patient_onboardings_user ON patient_onboardings(user_id)`,
		`ALTER TABLE patient_onboardings ADD COLUMN IF NOT EXISTS initial_password_hash VARCHAR(255)`,

		// V2: Multi-tenant foundation (P0)
		`CREATE TABLE IF NOT EXISTS tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			therapist_id UUID NOT NULL UNIQUE REFERENCES therapists(id) ON DELETE CASCADE,
			display_name VARCHAR(255) NOT NULL,
			timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Kolkata',
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tenants_therapist ON tenants(therapist_id)`,

		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			token_hash VARCHAR(64) NOT NULL UNIQUE,
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			expires_at TIMESTAMP NOT NULL,
			revoked_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id)`,

		`CREATE TABLE IF NOT EXISTS patients (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			user_id UUID REFERENCES users(id) ON DELETE SET NULL,
			full_name VARCHAR(255) NOT NULL,
			date_of_birth DATE,
			gender VARCHAR(50),
			phone VARCHAR(50),
			email VARCHAR(255),
			emergency_contact TEXT,
			address TEXT,
			assigned_therapist_id UUID REFERENCES therapists(id) ON DELETE SET NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_patients_tenant ON patients(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_patients_tenant_status ON patients(tenant_id, status)`,

		// Backfill tenants for existing approved therapists
		`INSERT INTO tenants (therapist_id, display_name)
		 SELECT t.id, t.name FROM therapists t
		 WHERE NOT EXISTS (SELECT 1 FROM tenants tn WHERE tn.therapist_id = t.id)`,

		// V2 P2: Appointments & scheduling
		`CREATE TABLE IF NOT EXISTS appointments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
			therapist_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'scheduled',
			starts_at TIMESTAMP NOT NULL,
			ends_at TIMESTAMP NOT NULL,
			meeting_link TEXT,
			location TEXT,
			notes TEXT,
			reminder_sent BOOLEAN NOT NULL DEFAULT FALSE,
			created_by UUID REFERENCES therapists(id) ON DELETE SET NULL,
			cancelled_at TIMESTAMP,
			cancel_reason TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_tenant_date ON appointments(tenant_id, starts_at)`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_therapist ON appointments(tenant_id, therapist_id, starts_at)`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_patient ON appointments(tenant_id, patient_id)`,

		`CREATE TABLE IF NOT EXISTS availability_slots (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			therapist_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			day_of_week SMALLINT NOT NULL,
			start_time TIME NOT NULL,
			end_time TIME NOT NULL,
			slot_duration_min INT NOT NULL DEFAULT 60,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_availability_tenant ON availability_slots(tenant_id, therapist_id)`,

		`CREATE TABLE IF NOT EXISTS calendar_integrations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			therapist_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			access_token_enc TEXT NOT NULL,
			refresh_token_enc TEXT NOT NULL,
			token_expires_at TIMESTAMP,
			calendar_id VARCHAR(255) DEFAULT 'primary',
			sync_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(tenant_id, therapist_id)
		)`,

		// V2 P3: Prescriptions & tasks
		`CREATE TABLE IF NOT EXISTS prescriptions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
			therapist_id UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			medicine_name VARCHAR(255) NOT NULL,
			dosage VARCHAR(100) NOT NULL,
			frequency VARCHAR(100) NOT NULL,
			duration_days INT,
			notes TEXT,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			prescribed_at TIMESTAMP NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP,
			discontinued_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_prescriptions_patient ON prescriptions(tenant_id, patient_id, status)`,

		`CREATE TABLE IF NOT EXISTS tasks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
			assigned_by UUID NOT NULL REFERENCES therapists(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			category VARCHAR(50),
			due_at TIMESTAMP,
			reminder_at TIMESTAMP,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			completed_at TIMESTAMP,
			patient_notes TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_patient ON tasks(tenant_id, patient_id, status)`,

		// V2 P4: Billing
		`CREATE TABLE IF NOT EXISTS billing_profiles (
			tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
			consultation_fee DECIMAL(10,2) DEFAULT 0,
			session_fee DECIMAL(10,2) DEFAULT 0,
			package_fees JSONB DEFAULT '[]',
			gst_rate DECIMAL(5,2) DEFAULT 18.00,
			invoice_prefix VARCHAR(20) DEFAULT 'INV',
			currency VARCHAR(10) DEFAULT 'INR',
			gst_number VARCHAR(50),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS invoices (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
			invoice_number VARCHAR(50) NOT NULL,
			appointment_id UUID REFERENCES appointments(id) ON DELETE SET NULL,
			subtotal DECIMAL(12,2) NOT NULL,
			gst_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
			total DECIMAL(12,2) NOT NULL,
			currency VARCHAR(10) NOT NULL DEFAULT 'INR',
			status VARCHAR(20) NOT NULL DEFAULT 'draft',
			due_at TIMESTAMP,
			paid_at TIMESTAMP,
			pdf_url TEXT,
			line_items JSONB NOT NULL,
			notes TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(tenant_id, invoice_number)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_tenant_status ON invoices(tenant_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_patient ON invoices(tenant_id, patient_id)`,

		`CREATE TABLE IF NOT EXISTS payments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
			provider VARCHAR(20) NOT NULL,
			external_id TEXT,
			amount DECIMAL(12,2) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			refunded_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_payments_invoice ON payments(tenant_id, invoice_id)`,

		// P6: Row-level security (defense-in-depth; use WithTenantRLS for enforcement)
		`ALTER TABLE patients ENABLE ROW LEVEL SECURITY`,
		`DROP POLICY IF EXISTS tenant_isolation_patients ON patients`,
		`CREATE POLICY tenant_isolation_patients ON patients
			USING (tenant_id::text = current_setting('app.tenant_id', true))`,
		`ALTER TABLE appointments ENABLE ROW LEVEL SECURITY`,
		`DROP POLICY IF EXISTS tenant_isolation_appointments ON appointments`,
		`CREATE POLICY tenant_isolation_appointments ON appointments
			USING (tenant_id::text = current_setting('app.tenant_id', true))`,
		`ALTER TABLE invoices ENABLE ROW LEVEL SECURITY`,
		`DROP POLICY IF EXISTS tenant_isolation_invoices ON invoices`,
		`CREATE POLICY tenant_isolation_invoices ON invoices
			USING (tenant_id::text = current_setting('app.tenant_id', true))`,
		`ALTER TABLE prescriptions ENABLE ROW LEVEL SECURITY`,
		`DROP POLICY IF EXISTS tenant_isolation_prescriptions ON prescriptions`,
		`CREATE POLICY tenant_isolation_prescriptions ON prescriptions
			USING (tenant_id::text = current_setting('app.tenant_id', true))`,
		`ALTER TABLE tasks ENABLE ROW LEVEL SECURITY`,
		`DROP POLICY IF EXISTS tenant_isolation_tasks ON tasks`,
		`CREATE POLICY tenant_isolation_tasks ON tasks
			USING (tenant_id::text = current_setting('app.tenant_id', true))`,

		`CREATE TABLE IF NOT EXISTS calendar_event_mappings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			appointment_id UUID NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
			integration_id UUID NOT NULL REFERENCES calendar_integrations(id) ON DELETE CASCADE,
			external_event_id TEXT NOT NULL,
			sync_status VARCHAR(20) NOT NULL DEFAULT 'synced',
			last_synced_at TIMESTAMP,
			UNIQUE(appointment_id, integration_id)
		)`,

		// Function and trigger to enforce append-only nature
		`CREATE OR REPLACE FUNCTION block_modifications()
		RETURNS TRIGGER AS $$
		BEGIN
			RAISE EXCEPTION 'Database Governance Policy: Modifications to audit logs are strictly prohibited';
		END;
		$$ LANGUAGE plpgsql`,

		`DROP TRIGGER IF EXISTS restrict_audit_mutations ON security_audit_logs`,

		`CREATE TRIGGER restrict_audit_mutations
		BEFORE UPDATE OR DELETE ON security_audit_logs
		FOR EACH ROW EXECUTE FUNCTION block_modifications()`,
	}

	for _, query := range queries {
		if _, err := PostgresDB.Exec(query); err != nil {
			return err
		}
	}

	log.Println("✅ PostgreSQL tables initialized")
	return nil
}

// DisconnectPostgres closes the PostgreSQL connection
func DisconnectPostgres() error {
	if PostgresDB != nil {
		return PostgresDB.Close()
	}
	return nil
}
