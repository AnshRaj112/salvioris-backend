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

To start the server in development mode:

```bash
go run cmd/server/main.go
```

The server will initialize and listen on `http://localhost:8080`. You should see logs indicating successful connections to PostgreSQL, Redis, and MongoDB.

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
