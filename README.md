# Serenify Backend Service

![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go)
![Chi Router](https://img.shields.io/badge/Router-Chi_v5-black?style=flat)
![Database](https://img.shields.io/badge/Database-MongoDB%20%7C%20PostgreSQL-47A248?style=flat&logo=mongodb)
![License](https://img.shields.io/badge/License-MIT-blue.svg)

## 📖 Overview

The **Serenify Backend** is a robust, high-performance RESTful API designed to power the Serenify mental health platform. Built with **Go** (Golang), it leverages a layered architecture to ensure scalability, maintainability, and security. This service handles user and therapist authentication, manages real-time chat functionality, processes secure file uploads, and orchestrates data persistence across both relational (PostgreSQL) and document-oriented (MongoDB) databases.

Key features include:
- **Dual-Database Strategy:** Utilizes PostgreSQL for structured relational data (users, therapists) and MongoDB for flexible document storage (chat history, therapy logs).
- **Secure Authentication:** Implements industry-standard JWT authentication with Argon2 password hashing.
- **High Performance:** Powered by the Chi router for lightweight and fast HTTP routing.
- **Caching & Rate Limiting:** Integrated Redis for efficient caching and API rate limiting to protect against abuse.
- **Media Management:** Seamless integration with Cloudinary for secure handling of media assets.

## 🏗️ Architecture

The project follows a clean, layered architecture (Clean Architecture principles) to separate concerns and improve testability:

```
serenify-backend/
├── cmd/
│   └── server/       # Application entry point
├── internal/
│   ├── config/       # Configuration management
│   ├── handlers/     # HTTP request handlers (Controllers)
│   ├── middleware/   # HTTP middleware (Auth, CORS, Rate Limiting)
│   ├── models/       # Data structures and domain models
│   ├── routes/       # API route definitions
│   └── services/     # Business logic layer
└── pkg/
    └── utils/        # Shared utility functions
```

## 🛠️ Technology Stack

| Category | Technology | Description |
|----------|------------|-------------|
| **Language** | [Go 1.25](https://go.dev/) | Core programming language. |
| **Router** | [Chi v5](https://github.com/go-chi/chi) | Lightweight, idiomatic, and composable router. |
| **Databases** | [PostgreSQL](https://www.postgresql.org/) | Primary relational database for user data. |
| | [MongoDB](https://www.mongodb.com/) | NoSQL database for chat logs and unstructured data. |
| **Caching** | [Redis](https://redis.io/) | In-memory data structure store for caching & sessions. |
| **Authentication** | [JWT](https://jwt.io/) | JSON Web Tokens for stateless authentication. |
| **Security** | [Argon2](https://github.com/golang/crypto) | Secure password hashing algorithm. |
| **Storage** | [Cloudinary](https://cloudinary.com/) | Cloud-based image and video management. |
| **Drivers** | `lib/pq`, `mongo-driver`, `go-redis` | Official database drivers. |

## 🔐 Security & Regulatory Compliance (E2EE & HIPAA-Ready)

Serenify is engineered from the ground up as a **zero-trust, privacy-first mental health communication platform**. By incorporating client-side End-to-End Encryption (E2EE) and administrative/technical safeguards, the platform is designed to be fully **HIPAA-Ready**.

### 🔒 1. Zero-Knowledge End-to-End Encryption (E2EE)
All clinical and peer messaging data is encrypted on-device before it reaches the backend, ensuring that only the sender and the designated recipients can read the message.

*   **Asymmetric Cryptography:** Uses **X25519** for ephemeral Diffie-Hellman (ECDH) key exchanges and **Ed25519** for cryptographic device identity signatures.
*   **Symmetric Encryption:** All messages are encrypted locally using **AES-256-GCM** with 96-bit random nonces.
*   **Cryptographic Binding:** Messages bind the ciphertext to the sender's device and the target group via a binding hash signed with the sender’s non-extractable Ed25519 key. This prevents message spoofing or network injection.
*   **Secure Storage:** Device keypairs are loaded into the browser's IndexedDB as non-extractable CryptoKeys via the native Web Crypto API, preventing malicious cross-site scripting (XSS) extraction.

### 🛡️ 2. HIPAA Technical & Administrative Safeguards
To comply with HIPAA Security Rule requirements (§164.312), Serenify implements robust access governance, auditing, and telemetry boundaries:

```mermaid
graph TD
    subgraph Client [Web Client - Web Crypto API]
        E2EE[E2EE Message Payload] -->|Secure Transit| Gateway[Chi API Gateway]
        LocalScan[Local Crisis scanner] -->|On-Device Escalation| UI[Local Crisis UI]
    end

    subgraph Backend [Go API Gateway & Enclave]
        Gateway -->|Verify WS Token| RedisOTP[Redis Replay Cache]
        Gateway -->|Route Context| RBAC[RBAC & MFA Middleware]
        RBAC -->|Save Ciphertext| MongoDB[(MongoDB E2EE Vault)]
        RBAC -->|Audited Decrypt| Enclave[Governed Disclosure Service]
    end

    subgraph Security Ledger [WORM Vault]
        RBAC -->|Audit Write| Postgres[(PostgreSQL Audit Ledger)]
        Postgres -.->|Trigger Exception| Lock[Append-Only Constraint]
    end
```

*   **Immutable Write-Once Audit Ledger (§164.312(b)):** Security events, privileged logins, and disclosure reviews are written to the `security_audit_logs` database ledger. A PostgreSQL engine trigger blocks all `UPDATE` and `DELETE` queries, forming a hardware-resilient Write-Once Read-Many (WORM) store.
*   **Role-Based Access Control (RBAC):** Access permission scopes restrict administrative, moderator, engineer, and support roles based on least-privilege principles.
*   **Hardware Multi-Factor Authentication (MFA):** Access to privileged portals mandates WebAuthn FIDO2 hardware keys, enforced by server-side active session checks.
*   **WebSocket Replay Prevention:** WebSocket upgrades require single-use tokens backed by Redis cache buffers that expire instantly upon connection.
*   **Structured Telemetry Sanitization:** A custom regular expression parser sanitizes application output, scrubbing E2EE keys, PII (emails, phone numbers), and plaintext traces before logs flow to external providers.
*   **Governed Moderation (No Backdoors):** Moderators cannot browse user messages. If abuse occurs, clients encrypt a WhatsApp-style message disclosure package under the Moderation Team’s Curve25519 public key. Decryption triggers high-priority compliance audit events.
*   **On-Device Psychiatric Safety Scanning:** Self-harm and acute crisis detection are executed in a browser thread locally to maintain privacy, offering local counselor routing without notifying the server.

## 🚀 Getting Started

### Prerequisites

Ensure you have the following installed on your local machine:
- **Go** (version 1.25 or higher)
- **PostgreSQL** (running on port 5432)
- **MongoDB** (running on port 27017)
- **Redis** (running on port 6379)
- **Git**

### Installation

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/AnshRaj112/serenify-backend.git
    cd serenify-backend
    ```

2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Environment Configuration:**
    Create a `.env` file in the root directory. You can copy the structure below:

    ```env
    # --- Server Configuration ---
    PORT=8080
    ENV=development
    HOST=http://localhost:8080

    # --- Database Connection URIs ---
    MONGODB_URI=mongodb://localhost:27017/serenify
    POSTGRES_URI=postgres://user:password@localhost:5432/serenify?sslmode=disable
    REDIS_URI=redis://localhost:6379/0

    # --- Security & Authentication ---
    JWT_SECRET=replace_with_a_secure_random_string
    # Generate a 32-byte base64 key: openssl rand -base64 32
    ENCRYPTION_KEY=replace_with_generated_key

    # --- CORS & Frontend Integration ---
    FRONTEND_URL=http://localhost:3000
    ALLOWED_ORIGINS=http://localhost:3000

    # --- Cloudinary ---
    CLOUDINARY_CLOUD_NAME=your_cloud_name
    CLOUDINARY_API_KEY=your_api_key
    CLOUDINARY_API_SECRET=your_api_secret
    ```

4.  **Database Setup:**
    - Ensure your PostgreSQL database `serenify` is created.
    - Unlike SQL, MongoDB will create the database and collections lazily upon the first write.

### Running the Application

#### Option 1: Running Locally (Development)

To start the server in development mode:

```bash
go run cmd/server/main.go
```

The server will initialize and listen on `http://localhost:8080`. You should see logs indicating successful connections to PostgreSQL, Redis, and MongoDB.

#### Option 2: Running via Docker

You can build and run the application locally as a Docker container. Ensure your database services are accessible from within the container:

```bash
# Build the Docker image
docker build -t serenify-backend .

# Run the container (Ensure .env variables match your Docker setup)
docker run -p 8080:8080 --env-file .env serenify-backend
```

#### Option 3: Google Cloud Run Deployment

This project includes a `Dockerfile` optimized for Google Cloud Run:

1. Connect your repository to a new Google Cloud Run service.
2. In the setup, set the Build type to **Dockerfile**.
3. **Important:** Leave the **Container command** and **Container arguments** completely blank.
4. Expand the "Containers, Volumes, Networking, Security" section, go to the **Variables & Secrets** tab, and add all required environment variables from your `.env` file.

## 📡 API Documentation

Usage examples for key endpoints.

### Authentication

#### User Sign Up
- **Endpoint:** `POST /api/auth/user/signup`
- **Body:**
  ```json
  {
    "username": "jdoe",
    "email": "jdoe@example.com",
    "password": "securePassword123"
  }
  ```

#### user Sign In
- **Endpoint:** `POST /api/auth/user/signin`
- **Body:**
  ```json
  {
    "email": "jdoe@example.com",
    "password": "securePassword123"
  }
  ```

### Therapist Management

#### Therapist Sign Up
- **Endpoint:** `POST /api/auth/therapist/signup`
- **Details:** Requires additional professional details (license number, specialization, etc.).

### Admin Controls

- `GET /api/admin/therapists/pending`: Retrieve list of therapists awaiting approval.
- `PUT /api/admin/therapists/approve`: Approve a therapist account.
- `DELETE /api/admin/therapists/reject`: Reject a therapist application.

*(For a complete list of endpoints, please refer to the `routes` package or the Postman collection provided in the docs.)*


## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

*Documentation maintained by the SALVIORIS Development Team.*
