# Fix production sign-in (e.g. `/api/auth/signin`)

If sign-in (or other API calls) fail in production with the request URL `https://backend.salvioris.com/api/auth/signin`, apply the following.

## 1. Set CORS origins on the backend (Render)

The browser sends an `Origin` header (your frontend URL). The backend must allow that origin or the response is blocked (CORS error or failed request).

**On Render (or your host), set one of:**

- **Option A – list all allowed origins (recommended)**  
  `ALLOWED_ORIGINS=https://salvioris.com,https://www.salvioris.com,http://localhost:3000`  
  (Add every origin your frontend is served from, comma-separated, no spaces or with spaces—both are trimmed.)

- **Option B – use frontend URL vars**  
  Set `FRONTEND_URL=https://salvioris.com` (and optionally `FRONTEND_URL_2`, `FRONTEND_URL_3`).  
  If `ALLOWED_ORIGINS` is not set, the server uses `FRONTEND_URL` (+ `_2`, `_3` if set) as allowed origins.

Redeploy the backend after changing env vars.

## 2. Confirm production env vars (backend)

- `ENV=production`
- `ALLOWED_ORIGINS` (or `FRONTEND_URL`) includes the exact origin the user sees in the address bar (e.g. `https://salvioris.com` or `https://www.salvioris.com`).

## 3. Frontend API URL (frontend host, e.g. Vercel)

Set the backend base URL for the deployed frontend:

- `NEXT_PUBLIC_API_URL=https://backend.salvioris.com`  
  (No trailing slash. The frontend uses this for all API calls.)

Redeploy the frontend after changing this.

## 4. What the backend does

- **CORS:** Backend uses allowed origins from `ALLOWED_ORIGINS` or `FRONTEND_URL` / `FRONTEND_URL_2` / `FRONTEND_URL_3`, so the production frontend origin is allowed. OPTIONS (preflight) receives 200 and CORS headers.

After setting the env vars and redeploying both backend and frontend, sign-in to `https://backend.salvioris.com/api/auth/signin` from your production site should work.
