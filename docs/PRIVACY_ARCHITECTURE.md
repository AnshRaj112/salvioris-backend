# Privacy-First Architecture Documentation

## Overview

Serenify is designed with **privacy-first principles** where users remain publicly anonymous at all times. The authentication and data architecture strictly separates public profile data from private recovery/security data.

## Database Schema

### Public Tables (Anonymous Identity)

#### `users` Table
- **Purpose**: Stores only public, anonymous identity
- **Fields**:
  - `id` (UUID) - Primary key
  - `username` (VARCHAR(20)) - Unique, lowercase, indexed
  - `password_hash` (VARCHAR(255)) - Argon2id hash
  - `created_at` (TIMESTAMP)
  - `is_active` (BOOLEAN)

**Privacy Rule**: This table contains NO real identity data (no names, emails, addresses).

### Private Tables (Encrypted Recovery Data)

#### `user_recovery` Table
- **Purpose**: Stores encrypted recovery information (email/phone)
- **Fields**:
  - `id` (UUID) - Primary key
  - `user_id` (UUID) - Foreign key to users
  - `email_encrypted` (TEXT) - AES-256-GCM encrypted
  - `phone_encrypted` (TEXT) - AES-256-GCM encrypted (optional)
  - `created_at`, `updated_at` (TIMESTAMP)

**Privacy Rule**: All data is encrypted at rest. Only decrypted when needed for recovery.

### Security Tables (Device Tracking)

#### `user_devices` Table
- **Purpose**: Device tracking for support and abuse prevention
- **Fields**:
  - `id` (UUID) - Primary key
  - `user_id` (UUID) - Foreign key to users
  - `device_token` (VARCHAR(255)) - Unique, generated token
  - `ip_address` (VARCHAR(255))
  - `user_agent` (TEXT)
  - `last_used`, `created_at` (TIMESTAMP)

**Privacy Rule**: Used only for support purposes, never for authentication.

## Authentication Flow

### Signup Process

1. User selects username (3-20 chars, alphanumeric + underscore)
2. System validates format
3. System checks availability (case-insensitive)
4. User sets password (min 8 chars)
5. Optional: User provides recovery email (encrypted and stored)
6. Account created with anonymous identity only

### Login Process

1. User provides username + password
2. System normalizes username (lowercase)
3. Password verified against hash
4. Device token generated and tracked
5. Returns only anonymous user data (id, username, created_at)

## API Endpoints

### Privacy-First Endpoints

- `POST /api/auth/signup` - Create anonymous account
- `POST /api/auth/signin` - Login with username
- `POST /api/auth/check-username` - Check username availability
- `POST /api/auth/forgot-username` - Recover username via email
- `POST /api/auth/forgot-password` - Reset password via email

### Response Format

All responses return **only anonymous data**:
```json
{
  "success": true,
  "user": {
    "id": "uuid",
    "username": "anonymous_user",
    "created_at": "timestamp"
  }
}
```

**Never returned**: email, phone, real names, addresses, or any PII.

## Username System

### Validation Rules
- Length: 3-20 characters
- Characters: Letters, numbers, underscores only
- Must start with letter or number
- Case-insensitive (stored lowercase)

### Uniqueness
- Database-level UNIQUE constraint on `username`
- Indexed for fast lookup: `idx_users_username_lower`
- Checked before account creation

## Encryption

### Email/Phone Encryption
- Algorithm: AES-256-GCM
- Key: 32-byte key from `ENCRYPTION_KEY` environment variable
- Storage: Base64-encoded ciphertext in database

### Key Generation
```bash
# Generate a 32-byte encryption key
openssl rand -base64 32
```

Set in `.env`:
```
ENCRYPTION_KEY=<32-byte-base64-key>
```

## Recovery System

### Forgot Username Flow
1. User provides recovery email
2. System searches encrypted emails (decrypts to compare)
3. If found, username sent via email
4. Always returns generic success message (privacy)

### Forgot Password Flow
1. User provides recovery email
2. System generates reset token
3. Reset link sent via email
4. User resets password with token

**Note**: Email search requires decrypting all emails. For production, consider:
- Email hash index for faster lookup
- Rate limiting on recovery endpoints
- Email verification before account creation

## Device Tracking

### Purpose
- Support assistance (identify user devices)
- Abuse prevention (track suspicious activity)
- Account recovery assistance (suggest known devices)

### Implementation
- Device token generated on each login
- Stored with IP address and user agent
- Never used for authentication
- Always requires email confirmation for recovery

## Anonymity Rules

### Content Display
- Vents/posts show only `username` (never user_id or real names)
- All user-generated content references anonymous username
- No PII in logs or error messages

### Admin Access
- Admin can access recovery data for support
- All admin access should be logged
- Recovery data decrypted only when needed

## Security Measures

1. **Password Hashing**: Argon2id with salt
2. **Encryption**: AES-256-GCM for sensitive data
3. **UUIDs**: No sequential IDs that reveal user count
4. **Rate Limiting**: Should be implemented on auth endpoints
5. **Input Validation**: Username format strictly enforced
6. **SQL Injection**: Parameterized queries only

## Environment Variables

```env
# Database
POSTGRES_URI=postgres://user:pass@localhost:5432/serenify?sslmode=disable
MONGODB_URI=mongodb://localhost:27017/serenify

# Security
JWT_SECRET=your-secret-key
ENCRYPTION_KEY=<32-byte-base64-key>  # Required for email/phone encryption

# Server
PORT=8080
FRONTEND_URL=http://localhost:3000
```

## Future Expansion

The architecture supports:
- Therapist accounts (separate table, same privacy principles)
- Chat systems (anonymous usernames only)
- Additional user types without redesigning identity model

## Migration Notes

- Legacy `users` table with email/name can coexist during migration
- New privacy-first endpoints are separate from legacy endpoints
- Gradual migration path: new users → privacy-first, existing users → migrate over time

