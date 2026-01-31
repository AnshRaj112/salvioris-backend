# Backend Setup Guide

## Environment Variables

Create a `.env` file in the `serenify-backend` directory with the following variables:

```env
# MongoDB Configuration
MONGODB_URI=mongodb://localhost:27017/serenify

# Server Configuration
PORT=8080
FRONTEND_URL=http://localhost:3000

# JWT Configuration
JWT_SECRET=your-secret-key-change-in-production

# Allowed Origins (comma-separated)
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
```

## Installation

1. Install dependencies:
```bash
cd serenify-backend
go mod tidy
```

2. Make sure MongoDB is running on your system.

3. Run the server:
```bash
go run cmd/server/main.go
```

## API Endpoints

### User Authentication
- `POST /api/auth/user/signup` - User registration
- `POST /api/auth/user/signin` - User login

### Therapist Authentication
- `POST /api/auth/therapist/signup` - Therapist registration
- `POST /api/auth/therapist/signin` - Therapist login

## Features

- ✅ MongoDB integration
- ✅ Argon2 password hashing
- ✅ CORS configuration
- ✅ User and Therapist authentication
- ✅ Input validation
- ✅ Error handling

