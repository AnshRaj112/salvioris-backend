# Database & API Security Summary

## Can anyone get into the database?

**Direct database access:** No. The database is not exposed to the internet. Only the backend server connects to PostgreSQL, MongoDB, and Redis using credentials from environment variables (e.g. `.env`). Users and the frontend never get a direct connection string.

**Indirect access via APIs:** Previously, several **admin-only** API endpoints did not check admin authentication. That meant anyone who knew the URLs could:

- List all feedback and contact submissions
- List pending/approved therapists (including PII)
- Approve or reject therapist applications
- List violations and blocked IPs, and unblock IPs
- List user and therapist waitlist entries

**Fix applied:** All admin endpoints now require a valid admin session token (`Authorization: Bearer <token>`). The frontend sends the token for all admin API calls. Unauthenticated requests to these endpoints receive `401 Unauthorized`.

---

## What is secure

1. **Credentials**
   - Database and Redis URIs are read from environment variables, not hardcoded.
   - `.env` is in `.gitignore` so it should not be committed.

2. **SQL**
   - Queries use parameterized statements (e.g. `$1`, `$2`), which helps prevent SQL injection.

3. **Rate limiting**
   - Global rate limiting middleware is applied to reduce abuse.

4. **CORS**
   - Backend restricts allowed origins to `FrontendURL`, reducing cross-site misuse.

5. **Admin routes (after fix)**
   - All admin handlers now call `requireAdminAuth` and reject requests without a valid admin session.

---

## What you should do

1. **Never commit `.env`**
   - Ensure `.env` stays in `.gitignore`. If it was ever committed or shared, rotate all secrets (Postgres, MongoDB, Redis, JWT_SECRET, ENCRYPTION_KEY) and revoke/regenerate any API keys.

2. **Production secrets**
   - Set strong, unique values for `JWT_SECRET`, `ENCRYPTION_KEY`, and database URIs in production. The default `JWT_SECRET` in config is only for development.

3. **Database and Redis**
   - Use managed services (e.g. Render Postgres, MongoDB Atlas, Redis Cloud) with TLS and restrict network access (IP allowlist or VPC) where possible.
   - Use a dedicated DB user with minimal required privileges, not a superuser.

4. **Admin accounts**
   - Admin signup is disabled; admins are created directly in the database. Use strong passwords and limit who has admin access.

---

## Summary

- **Database:** Not directly reachable by users; only the backend connects with env-based credentials.
- **Admin APIs:** Were previously unprotected; they now require admin authentication. The frontend sends the admin token for all admin calls.
- **Best practice:** Keep `.env` out of version control, use strong production secrets, and limit DB and admin access.
