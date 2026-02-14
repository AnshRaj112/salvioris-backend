# Production Security: Rate Limiting & Hardening

This document describes the **production-only** security stack for the Go backend when running on Render. Traffic flows **directly** from the internet to Render (no CDN/proxy layer). CORS is configured for the production frontend (e.g. `https://www.salvioris.com`).

---

## Table of contents

1. [Overview](#overview)
2. [When it runs](#when-it-runs)
3. [Files involved](#files-involved)
4. [Environment & config](#environment--config)
5. [Middleware flow](#middleware-flow)
6. [Client IP](#client-ip)
7. [Global rate limiting](#global-rate-limiting)
8. [Login route rate limiting](#login-route-rate-limiting)
9. [Security headers](#security-headers)
10. [Production checklist](#production-checklist)

---

## Overview

| Feature | Description |
|--------|--------------|
| **Client IP** | From `r.RemoteAddr` only (no proxy headers). |
| **Global rate limit** | Per-IP: 1 req/s, burst 10. HTTP 429 when exceeded. In-memory, thread-safe. |
| **Login rate limit** | Sign-in routes only: 1 req / 5s, burst 2. HTTP 429 when exceeded. |
| **Security headers** | `X-Content-Type-Options`, `X-Frame-Options`, `X-XSS-Protection`, CSP, HSTS. |

All of this runs **only when `ENV=production`**. Otherwise the app uses the existing Redis-based rate limiting only.

---

## When it runs

- **`ENV=production`** → Production security stack is applied (security headers, global + login rate limits).
- **Any other `ENV`** (e.g. `development`) → Only the existing Redis-based `RateLimitMiddleware` is used.

---

## Files involved

| File | Purpose |
|------|--------|
| **`pkg/clientip/clientip.go`** | `RealClientIP(r *http.Request) string` — parses `r.RemoteAddr` with `net.SplitHostPort`. |
| **`internal/config/config.go`** | `Environment`, `IsProduction()`, `AllowedOrigins` (CORS). No host validation. |
| **`internal/middleware/security.go`** | `SecurityHeaders`, `GlobalRateLimit`, `LoginRateLimit`, `ProductionSecurity()`. |
| **`cmd/server/main.go`** | If `cfg.IsProduction()`: applies `ProductionSecurity()`; else `RateLimitMiddleware`. CORS first. |
| **`internal/services/moderation.go`** | `GetIPAddress(r)` calls `clientip.RealClientIP(r)`. |
| **`internal/middleware/ratelimit.go`** | Redis-based rate limit when **not** production. Uses `services.GetIPAddress(r)`. |

---

## Environment & config

### Environment variables

| Variable | Example | Purpose |
|----------|---------|--------|
| **`ENV`** | `production` | Enables production security stack. |
| **`HOST`** | `https://backend.salvioris.com` | Optional; for URL generation if needed. Not used for host validation. |
| **`ALLOWED_ORIGINS`** | `https://www.salvioris.com,http://localhost:3000` | CORS allowed origins (or use `FRONTEND_URL` / `FRONTEND_URL_2` / `FRONTEND_URL_3`). |

### Config fields

- **`Environment`** – from `ENV`, default `development`.
- **`IsProduction() bool`** – true when `Environment` is `production`.
- **`AllowedOrigins`** – from `ALLOWED_ORIGINS` or `FRONTEND_URL`(s). Must include production frontend origin.

---

## Middleware flow

Order:

1. **CORS** (first).
2. **Production only:** **SecurityHeaders** → **GlobalRateLimit** → **LoginRateLimit** (`middleware.ProductionSecurity()`).
3. **Non-production only:** **RateLimitMiddleware** (Redis-based).
4. Routes (e.g. `GET /health`, then `routes.SetupRoutes(r)`).

---

## Client IP

- **Source:** `pkg/clientip/clientip.go` → `RealClientIP(r *http.Request) string`.
- **Logic:** Parse `r.RemoteAddr` with `net.SplitHostPort`; return host part. On error, return `r.RemoteAddr` trimmed.
- **No proxy headers:** No `CF-Connecting-IP`, `X-Forwarded-For`, or `X-Real-IP` — traffic is assumed to hit Render (or the app) directly.

---

## Global rate limiting

- **Middleware:** `GlobalRateLimit` in `internal/middleware/security.go`.
- **Limits:** 1 request per second per IP, burst 10.
- **On exceed:** HTTP 429, JSON body `{"success":false,"message":"Too many requests. Please slow down."}`.
- **Storage:** In-memory map with mutex; cleanup every 5 minutes (entries unused for 30 minutes removed).

---

## Login route rate limiting

- **Middleware:** `LoginRateLimit` in `internal/middleware/security.go`.
- **Paths:** `/api/auth/signin`, `/api/auth/user/signin`, `/api/auth/therapist/signin`, `/api/admin/signin`.
- **Limits:** 1 request every 5 seconds per IP, burst 2.
- **On exceed:** HTTP 429, JSON body `{"success":false,"message":"Too many login attempts. Please try again later."}`.

---

## Security headers

- **Middleware:** `SecurityHeaders` in `internal/middleware/security.go`.
- **Headers:** `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `X-XSS-Protection: 1; mode=block`, `Content-Security-Policy: default-src 'self'`, `Strict-Transport-Security: max-age=31536000; includeSubDomains`.

---

## Production checklist

1. **Backend (e.g. Render)**
   - `ENV=production`
   - **`ALLOWED_ORIGINS`** or **`FRONTEND_URL`** must include your production frontend origin (e.g. `https://www.salvioris.com`). Example: `ALLOWED_ORIGINS=https://www.salvioris.com,https://salvioris.com,http://localhost:3000`

2. **DNS**
   - Point `backend.salvioris.com` (CNAME) to your Render service (e.g. `serenify-backend-25s5.onrender.com`). No proxy/CDN in between.
   - See [DNS_SETUP.md](./DNS_SETUP.md) for full DNS at registrar.

3. **Frontend**
   - `NEXT_PUBLIC_API_URL=https://backend.salvioris.com` (no trailing slash).

4. **Behavior**
   - CORS allows only configured origins. Security headers and per-IP + login rate limits apply in production. No host validation.
