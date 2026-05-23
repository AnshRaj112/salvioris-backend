# Serenify Backend Security & HIPAA Compliance Technical Specification

## Document Context & Scope
This specification acts as the definitive architectural design, deployment guide, and regulatory compliance map for the security safeguards implemented in the **Serenify Backend Service**.

This document is engineered to serve security auditors, HIPAA compliance officers, distributed systems engineers, and lead security architects. It details the backend architectures, cryptographic protocols, database engines, and operational boundaries required to support Serenify as a fully **HIPAA-Ready, privacy-first platform**.

---

## 🏗️ 1. Zero-Trust Architecture & Network Segments

The Serenify backend operates under an absolute **Zero-Trust Posture**. The system assumes the transport network, database hosts, administrators, and infrastructure hosts are compromised or untrusted. The backend serves purely as a **blind, cryptographically validated transit courier** for mental health messages.

### Network Boundaries and Topology
```
[ Public Internet: Patients & Therapists ]
               │ (WSS / HTTPS - TLS 1.3 only)
               ▼
   [ Application Load Balancer (ALB) ]
               │ (Terminates Edge TLS, performs DDoS filtering)
               ▼ (mTLS / SPIFFE-SPIRE Enclave)
   [ Private Subnet: Chi API Gateway Instances ]
         ├── (Private VPC Connection) ──> [ Ephemeral Session Cache: Redis Sentinel ]
         ├── (Private VPC Connection) ──> [ Governed Disclosure Enclave (KMS HSM) ]
         └── (Private VPC Connection) ──> [ Persistent Vaults ]
                                                ├── [ MongoDB E2EE Vault ]
                                                └── [ Postgres Audit & Auth Ledger ]
```

### Trust Boundary Rules
1.  **Transport Cryptography:** External connections must mandate TLS 1.3 with secure cipher suites (`TLS_AES_256_GCM_SHA384` and `TLS_CHACHA20_POLY1305_SHA256`).
2.  **Service-to-Service Isolation:** Microservices communicate using Mutual TLS (mTLS) backed by an ephemeral SPIFFE/SPIRE certificate authority. Direct cross-database scanning is prohibited.
3.  **Untrusted Host Principle:** The server has **zero capacity** to decrypt patient chat envelopes under normal operations. Database dumps or server physical memory compromises do not reveal patient communications.

---

## 🔒 2. HIPAA Technical Safeguards & Scoped Middlewares

To satisfy **HIPAA Security Rule §164.312 (Technical Safeguards)**, the backend implements granular, audited interceptors that enforce access control, session validation, and transmission integrity.

### A. Role-Based Access Control (RBAC) Middleware
*   **Target Code:** `internal/middleware/rbac.go`
*   **Regulatory Standard:** HIPAA §164.312(a)(2)(i) (Unique User Identification & Scoped Authorization).
*   **Detailed Workflow:**
    1.  The incoming request is parsed by upstream authentication handlers to validate the JSON Web Token (JWT).
    2.  The user's unique identifier (`user_id`) and administrative role designation (`user_role`) are extracted and injected into the request's Go `context.Context` structure.
    3.  The `RequireRole` middleware interceptor checks the extracted role against a set of allowed roles specified during route initialization (e.g. `admin`, `moderator`, `compliance`, `support`).
    4.  If the role is valid, context execution continues. If the role context is missing or does not match, the middleware generates a high-priority audit log entry:
        *   **Event:** `UNAUTHORIZED_ACCESS_ATTEMPT` or `ACCESS_DENIED_INSUFFICIENT_PRIVILEGES`
        *   **Action:** Immediately returns `403 Forbidden` with a generic JSON error payload, masking backend structures.

### B. Privileged Session MFA Checkpoint Middleware
*   **Target Code:** `internal/middleware/mfa_enforcer.go`
*   **Regulatory Standard:** HIPAA §164.312(a)(2)(iv) (Encryption and Decryption access controls) & §164.308(a)(5)(ii)(C) (Log-in Monitoring).
*   **Detailed Workflow:**
    1.  For endpoints designated as **Privileged Administrative Operations** (e.g. moderation decryptions, audit log reviews), the request pipeline invokes `MFAEnforcer`.
    2.  The middleware queries the `staff_sessions` relational PostgreSQL table using the context-derived `actor_id`.
    3.  It checks the following columns:
        *   `mfa_verified` (BOOLEAN): Must be explicitly set to `TRUE`.
        *   `last_mfa_at` (TIMESTAMP): Must have occurred within the last 12 hours.
        *   `active` (BOOLEAN): Must verify the session has not been administratively revoked.
    4.  If verification fails, the server records a `PRIVILEGED_MFA_CHALLENGE_FAILED` event in the audit ledger, injects `X-MFA-Challenge-Required: true` into the response headers, and returns `412 Precondition Required`.

### C. WebSocket Replay Protection Middleware
*   **Target Code:** `internal/middleware/ws_auth.go`
*   **Regulatory Standard:** Safeguarding against session hijacking, man-in-the-middle (MitM) sniffing, and Cross-Site WebSocket Hijacking (CSWSH).
*   **Detailed Workflow:**
    1.  Unlike REST APIs, WebSockets cannot reliably append custom authorization headers during initial handshakes. Clients must request a **One-Time Password (OTP) WebSocket Token** via a secure HTTPS route.
    2.  The backend generates a cryptographically random 32-byte string (`ws_token`) and saves it in Redis with the user's metadata under the key `ws_otp:<token>` with a strict TTL of 30 seconds.
    3.  During the WS handshake, the client passes this string inside the query parameter: `wss://api.serenify.app/ws?ws_token=<token>`.
    4.  The `AuthenticateWebSocket` middleware extracts the token, queries `RedisClient`, and immediately invokes `DEL` to burn the token, neutralizing replay vectors.
    5.  The parsed session details are bound to the WebSocket context. If the token does not exist, the handshake is aborted with `401 Unauthorized`.

---

## 📝 3. Telemetry Redaction & Log Sanitizer

*   **Target Code:** `pkg/logger/sanitizer.go`
*   **Regulatory Standard:** HIPAA §164.314 (Organizational Requirements) & HIPAA Breach Notification Rule (§164.400-414).
*   **Technical Risks addressed:** accidental transmission of Patient Health Information (PHI) or Personally Identifiable Information (PII) to cloud application performance monitors (APM) like Sentry or Datadog.

### RegEx Redaction Logic
Our core sanitizer runs a stream parser over every log entry using high-performance regular expression structures:

| Target Pattern | Regular Expression | Redacted Label |
| :--- | :--- | :--- |
| **JSON E2EE Attributes** | `(?i)"(message\|ciphertext\|phone\|email\|password\|recovery_secret\|nonce\|signature)"\s*:\s*"[^"]+"` | `"$1":"[REDACTED_PHI_SECURE]"` |
| **E.164 Phone Numbers** | `\+?\d{1,4}?[-.\s]?\(?\d{1,3}?\)?[-.\s]?\d{1,4}[-.\s]?\d{1,4}[-.\s]?\d{1,9}` | `[PHONE_REDACTED]` |
| **RFC 5322 Emails** | `(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}` | `[EMAIL_REDACTED]` |

### Implementation Integration
All logs written by Chi routers, MongoDB drivers, or application exceptions pass through our custom `RedactWriter` implementation, which wraps `os.Stdout` and `os.Stderr`. This guarantees that even unhandled stack traces are fully redacted prior to being shipped to observability agents.

---

## 💾 4. Database Security Ledger (PostgreSQL WORM)

*   **Target Code:** `internal/database/postgres.go`
*   **Regulatory Standard:** HIPAA §164.312(b) (Audit Controls) & §164.312(c)(2) (Mechanism to authenticate EPHI and verify integrity).

### PostgreSQL Audit Schema Specification
The `security_audit_logs` table acts as a **Write-Once Read-Many (WORM)** ledger. The table is structured to document:
*   **Who:** `actor_id` + `actor_role` (identifies the unique administrative account).
*   **What:** `event_type` + `reason` (detailed description of the compliance query).
*   **When:** `created_at` (server-enforced UTC timestamp).
*   **Where:** `ip_address` + `user_agent` (identifies the origin network location).
*   **Why:** `target_id` (links to the active resource like the report UUID).

```sql
CREATE TABLE IF NOT EXISTS security_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    target_id VARCHAR(255) NOT NULL,
    actor_id VARCHAR(255) NOT NULL,
    actor_role VARCHAR(50) NOT NULL DEFAULT 'unknown',
    reason TEXT NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    user_agent TEXT NOT NULL DEFAULT 'unknown',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_security_audit_actor ON security_audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_security_audit_created ON security_audit_logs(created_at);
```

### Immutable Constraints Trigger
To guarantee the integrity of the audit logs against rogue DB administrators or root compromises, a PL/pgSQL database trigger blocks all `UPDATE` and `DELETE` commands:

```sql
CREATE OR REPLACE FUNCTION block_modifications()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Database Governance Policy: Modifications to audit logs are strictly prohibited';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER restrict_audit_mutations
BEFORE UPDATE OR DELETE ON security_audit_logs
FOR EACH ROW EXECUTE FUNCTION block_modifications();
```

---

## 🕵️ 5. Governed Moderation & Asymmetric Decryption

*   **Target Code:** `internal/services/disclosure.go` & `internal/handlers/reports.go`
*   **Regulatory Standard:** HIPAA §164.308(a)(1)(ii)(D) (Information System Activity Review).

### The Governed Disclosure Workflow (Step-by-Step)

```
[ Patient Device ]                            [ Postgres DB ]              [ Moderator Console ]
        │                                            │                               │
        │── 1. Packages & Encrypts ECIES ───────────>│                               │
        │      (Curve25519 Ephemeral Key derived)    │                               │
        │                                            │                               │
        │                                            │── 2. Request Decrypt ────────>│
        │                                            │   (Passes signed reason)      │
        │                                            │                               │
        │                                            │<── 3. Audit Checkpoint Log ───│
        │                                            │                               │
        │<─ 4. Compute Shared Secret ────────────────│                               │
        │      (Curve25519 Private + Ephemeral Pub)  │                               │
        │                                            │                               │
        │<─ 5. AES-256-GCM Decrypt ──────────────────│                               │
        v                                            v                               v
```

1.  **Report Submission:** When an active user files an abuse report, the client fetches the static **Server Moderation Public Key** (Curve25519).
2.  **ECIES Package Compilation:** The client encrypts the target message and immediate context using an ephemeral key derived from the server's public key (using ECDH and AES-GCM wrapping). It submits the encrypted ECIES envelope to the backend `/api/v1/moderation/submit` endpoint.
3.  **Postgres Ledger Insertion:** The backend saves the envelope to the `abuse_reports` table. At this stage, the backend cannot read the content.
4.  **Moderator Decryption Authorization:** A safety moderator initiates a review request. The backend intercepts the request and verifies both RBAC permission levels and active FIDO2 MFA status.
5.  **Strict Audit Logging:** Prior to executing decryption, the database inserts an immutable record into `security_audit_logs`, tracking the moderator's session metadata and their stated clinical justification.
6.  **Enclave Cryptographic Operation:**
    *   The server's **X25519 Private Key** is fetched securely from KMS memory.
    *   The backend extracts the **Ephemeral Public Key** (first 32 bytes) and the **Nonce** (next 12 bytes) from the envelope.
    *   Curve25519 scalar multiplication is executed: `SharedSecret = X25519(PrivateKeyX25519, EphemeralPublicKey)`.
    *   A Key Encryption Key (KEK) is derived: `KEK = SHA-256(SharedSecret)`.
    *   The backend decrypts the remaining ciphertext envelope using **AES-256-GCM** with the derived `KEK` and `Nonce`.
7.  **Response Payload:** The backend returns the decrypted plaintext array containing message records to the moderator's session memory, updating the report status column in PostgreSQL to `reviewed`.
