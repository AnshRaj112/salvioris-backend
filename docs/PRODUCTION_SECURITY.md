# Production Security: DDoS Mitigation, Rate Limiting & Hardening

This document describes the **production-only** security stack for the Go backend when running on Render behind Cloudflare (`HOST`). It covers every file, config, middleware order, and behavior.

---

## Table of contents

1. [Overview](#overview)
2. [When it runs](#when-it-runs)
3. [Files involved](#files-involved)
4. [Environment & config](#environment--config)
5. [Middleware flow](#middleware-flow)
6. [Real client IP (Cloudflare)](#real-client-ip-cloudflare)
7. [Global rate limiting](#global-rate-limiting)
8. [Login route rate limiting](#login-route-rate-limiting)
9. [Strict host validation](#strict-host-validation)
10. [Security headers](#security-headers)
11. [Dependencies](#dependencies)
12. [Call sites using client IP](#call-sites-using-client-ip)
13. [Production checklist](#production-checklist)

---

## Overview

| Feature | Description |
|--------|--------------|
| **Trust Cloudflare** | Real client IP from `CF-Connecting-IP`, fallback `RemoteAddr` via `net.SplitHostPort`. |
| **Global rate limit** | Per-IP: 1 req/s, burst 10. HTTP 429 when exceeded. In-memory, thread-safe. |
| **Login rate limit** | Sign-in routes only: 1 req / 5s, burst 2. HTTP 429 when exceeded. |
| **Host check** | Reject requests when `Host != backend.salvioris.com` (HTTP 403). |
| **Security headers** | `X-Content-Type-Options`, `X-Frame-Options`, `X-XSS-Protection`, CSP, HSTS. |

All of this runs **only when `ENV=production`**. Otherwise the app uses the existing Redis-based rate limiting only.

---

## When it runs

- **`ENV=production`** → Production security stack is applied (security headers, host check, global + login rate limits).
- **Any other `ENV`** (e.g. `development`) → Only the existing Redis-based `RateLimitMiddleware` is used; no host check, no extra security headers from this stack.

---

## Files involved

| File | Purpose |
|------|--------|
| **`pkg/clientip/clientip.go`** | Single helper: `RealClientIP(r *http.Request) string`. Uses `CF-Connecting-IP`, then `net.SplitHostPort(r.RemoteAddr)`. |
| **`internal/config/config.go`** | Adds `Environment`, `AllowedHost` (from `HOST`), and `IsProduction()`. Parses `HOST` to a bare hostname (no scheme/port). |
| **`internal/middleware/security.go`** | All production middlewares: `SecurityHeaders`, `HostCheck(allowedHost)`, `GlobalRateLimit`, `LoginRateLimit`, plus `ProductionSecurity(allowedHost)` and per-IP limiter maps with cleanup. |
| **`cmd/server/main.go`** | If `cfg.IsProduction()`: applies `ProductionSecurity(cfg.AllowedHost)`; else applies `RateLimitMiddleware`. CORS stays first. |
| **`internal/services/moderation.go`** | `GetIPAddress(r)` updated to call `clientip.RealClientIP(r)` so all logging/rate-limiting use the same IP. |
| **`internal/middleware/ratelimit.go`** | Existing Redis-based rate limit; used only when **not** production. Uses `services.GetIPAddress(r)`. |
| **`go.mod`** | Adds `golang.org/x/time/rate`. |

No other files are required for this feature. Handlers that already use `services.GetIPAddress(r)` (feedback, contact, waitlist, journal, vent, etc.) automatically use the real client IP once `GetIPAddress` is implemented via `clientip.RealClientIP`.

---

## Environment & config

### Environment variables

| Variable | Example | Purpose |
|----------|---------|--------|
| **`ENV`** | `production` | Enables production security stack when set to `production` (case-insensitive). |
| **`HOST`** | `https://backend.salvioris.com` | Used to derive `AllowedHost` for strict host check. Scheme and port are stripped; comparison is hostname-only. |

### Config fields (`internal/config/config.go`)

- **`Environment`** – from `ENV`, default `development`.
- **`AllowedHost`** – from `HOST`: strip `https://` or `http://`, then strip path and port. Example: `https://backend.salvioris.com` → `backend.salvioris.com`.
- **`IsProduction() bool`** – returns true when `Environment` is `production` (trimmed, lowercased).

---

## Middleware flow

Order is fixed:

1. **CORS** (unchanged, first).
2. **Production only:**  
   **SecurityHeaders** → **HostCheck** → **GlobalRateLimit** → **LoginRateLimit**  
   (exposed as `middleware.ProductionSecurity(cfg.AllowedHost)`).
3. **Non-production only:**  
   **RateLimitMiddleware** (existing Redis-based).
4. Routes (e.g. `GET /health`, then `routes.SetupRoutes(r)`).

So:

- **SecurityHeaders** – set headers on every response.
- **HostCheck** – reject wrong host before any rate limiting.
- **GlobalRateLimit** – per-IP token bucket (1/s, burst 10).
- **LoginRateLimit** – stricter limit on sign-in paths only (1/5s, burst 2).

---

## Real client IP (Cloudflare)

- **Source of truth:** `pkg/clientip/clientip.go` → `RealClientIP(r *http.Request) string`.
- **Logic:**
  1. If `CF-Connecting-IP` is set and non-empty (after trim), return it.
  2. Else parse `r.RemoteAddr` with `net.SplitHostPort` and return the host part; on parse error, return `r.RemoteAddr` as-is.
- **Usage:** All rate limiting and logging must use this IP: production middlewares and `services.GetIPAddress(r)` both rely on it (with `GetIPAddress` delegating to `RealClientIP`).

---

## Global rate limiting

- **Middleware:** `GlobalRateLimit` in `internal/middleware/security.go`.
- **Library:** `golang.org/x/time/rate` (token bucket).
- **Limits:** 1 request per second per IP, burst 10.
- **On exceed:** HTTP 429, JSON body e.g. `{"success":false,"message":"Too many requests. Please slow down."}`.
- **Storage:** In-memory map `globalEntries` from client IP → `*limiterEntry` (limiter + `lastUse`). Access guarded by `globalEntriesMu` (`sync.Mutex`).
- **Cleanup:** Background goroutine every 5 minutes removes entries not used in the last 30 minutes to avoid unbounded growth.

---

## Login route rate limiting

- **Middleware:** `LoginRateLimit` in `internal/middleware/security.go`.
- **Paths (exact):**  
  `/api/auth/signin`, `/api/auth/user/signin`, `/api/auth/therapist/signin`, `/api/admin/signin`.
- **Limits:** 1 request every 5 seconds per IP, burst 2.
- **On exceed:** HTTP 429, JSON body e.g. `{"success":false,"message":"Too many login attempts. Please try again later."}`.
- **Storage:** Separate in-memory map `loginEntries` with its own mutex and cleanup (same pattern: 5 min interval, 30 min TTL).

---

## Strict host validation

- **Middleware:** `HostCheck(allowedHost)` in `internal/middleware/security.go`.
- **Behavior:**  
  - Parse `r.Host` with `net.SplitHostPort`; if that fails, use `r.Host` as the host.  
  - If the host part is not equal (case-insensitive, trimmed) to `allowedHost`, respond with **HTTP 403** and body `Forbidden`, and do not call next handler.
- **Purpose:** Block direct access via the Render host (e.g. `.onrender.com`) so traffic must go through Cloudflare at `backend.salvioris.com`.

---

## Security headers

- **Middleware:** `SecurityHeaders` in `internal/middleware/security.go`.
- **Headers set on every response:**

| Header | Value |
|--------|--------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `X-XSS-Protection` | `1; mode=block` |
| `Content-Security-Policy` | `default-src 'self'` |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` |

---

## Dependencies

- **`golang.org/x/time/rate`** – used only in `internal/middleware/security.go` for both global and login limiters.
- No other new external dependencies. CORS and JWT auth are unchanged.

---

## Call sites using client IP

These use `services.GetIPAddress(r)`, which (after the change) uses `clientip.RealClientIP(r)`:

- `internal/middleware/ratelimit.go` – Redis rate limit (non-production).
- `internal/handlers/feedback.go`
- `internal/handlers/contact.go`
- `internal/handlers/waitlist.go` (two places)
- `internal/handlers/journal.go`
- `internal/handlers/vent.go`

Production middlewares use `clientip.RealClientIP(r)` directly in `security.go`. So “every file” that needs the real client IP goes through one implementation (Cloudflare + `net.SplitHostPort` fallback).

---

## Production checklist

1. **Render (or host) env**
   - `ENV=production`
   - `HOST = host`

2. **DNS / proxy**
   - Traffic to the backend goes through Cloudflare so `CF-Connecting-IP` is set.
   - Public hostname is `backend.salvioris.com` so `Host` matches `AllowedHost`.

3. **No bypass**
   - Users should hit `HOST`; direct `.onrender.com` (or other host) will get 403 when host check is enabled.

4. **Behavior**
   - First middleware after CORS: security headers.
   - Then host check, then global rate limit, then login rate limit, then routes.
   - No change to CORS or JWT; optional cleanup goroutines prevent limiter map growth.

This doc and the listed files together define the full production security setup (every file and everything).
