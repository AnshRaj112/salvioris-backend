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
		
		// Groups table (public community groups)
		`CREATE TABLE IF NOT EXISTS groups (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			name VARCHAR(255) NOT NULL,
			description TEXT,
			created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			is_public BOOLEAN NOT NULL DEFAULT TRUE,
			member_count INTEGER NOT NULL DEFAULT 1
		)`,
		
		// Group members table (many-to-many relationship)
		`CREATE TABLE IF NOT EXISTS group_members (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
		`CREATE INDEX IF NOT EXISTS idx_group_members_group_id ON group_members(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_messages_group_id ON group_messages(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_messages_created_at ON group_messages(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_group_messages_user_id ON group_messages(user_id)`,
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

