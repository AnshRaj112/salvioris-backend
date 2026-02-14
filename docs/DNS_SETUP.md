# DNS Setup (No Cloudflare)

This document describes how to serve **salvioris.com** and **backend.salvioris.com** using **only** your domain registrar’s DNS. No Cloudflare, no proxy, no CDN in between.

---

## Architecture

```
User
  ↓
Vercel (frontend edge)     ← www.salvioris.com / salvioris.com
  ↓
Render (backend edge)      ← backend.salvioris.com
  ↓
Go application
```

- **Frontend:** Vercel (e.g. `your-vercel-project.vercel.app`).
- **Backend:** Render (e.g. `serenify-backend-25s5.onrender.com`).
- **Custom domain:** salvioris.com. Subdomain: backend.salvioris.com.

---

## 1. Remove Cloudflare

1. **Restore nameservers at registrar**
   - In your domain registrar (where you bought salvioris.com), change nameservers **from** Cloudflare **back to** the registrar’s default (e.g. “Parking” or the NS they gave you when you registered the domain).
2. **Remove domain from Cloudflare**
   - In Cloudflare dashboard: remove salvioris.com (or delete the site). This stops any traffic going through Cloudflare.
3. **Wait for propagation**
   - DNS can take up to 24–48 hours to fully propagate. Until then, some users might still see old Cloudflare behavior.

---

## 2. Add DNS records at registrar

Add these at your **registrar’s DNS management** (where you set nameservers). Use **DNS only** — no proxy, no “orange cloud”, no CDN.

### Frontend (Vercel)

| Type  | Name   | Value                              | Proxy / CDN |
|-------|--------|------------------------------------|-------------|
| CNAME | `www`  | `your-vercel-project.vercel.app`   | **Off**     |

If Vercel requires an **A record** for the root domain (`@`):

| Type | Name | Value (Vercel-provided IP) | Proxy / CDN |
|------|------|----------------------------|-------------|
| A    | `@`  | (IP from Vercel dashboard) | **Off**     |

- Replace `your-vercel-project` with your actual Vercel project hostname.
- In Vercel: Domain Settings → add `salvioris.com` and `www.salvioris.com`; follow their instructions for the exact CNAME/A values.

### Backend (Render)

| Type  | Name     | Value                                | Proxy / CDN |
|-------|----------|--------------------------------------|-------------|
| CNAME | `backend`| `serenify-backend-25s5.onrender.com` | **Off**     |

- Replace with your real Render service hostname if different.
- Ensure “Proxy” or “CDN” is **disabled** for this record at the registrar (if they offer it).

---

## 3. Checklist

- [ ] Nameservers at registrar point to **registrar default** (not Cloudflare).
- [ ] Domain removed from Cloudflare (or Cloudflare no longer in use).
- [ ] CNAME `www` → `your-vercel-project.vercel.app` (no proxy).
- [ ] CNAME `backend` → `serenify-backend-25s5.onrender.com` (no proxy).
- [ ] If required by Vercel: A record `@` → Vercel’s IP (no proxy).
- [ ] No redirect rules or proxy layers in between user and Vercel/Render.

---

## 4. Validate after propagation

1. **Frontend**
   - `https://www.salvioris.com` (or `https://salvioris.com` if you set root) loads the Vercel app.
2. **Backend**
   - `https://backend.salvioris.com/health` returns **200** and body `OK`.
3. **Login**
   - Sign-in from the frontend works; no 403 from proxy layers, no CORS errors, no Cloudflare “Error 1000”.

---

## 5. Troubleshooting

- **Frontend not loading:** Confirm CNAME `www` (and A `@` if used) point to Vercel and that Vercel shows the domain as verified.
- **Backend 5xx or not reachable:** Confirm CNAME `backend` points to the correct Render hostname and that the Render service is running and has the custom domain `backend.salvioris.com` added.
- **CORS errors:** Ensure backend `ALLOWED_ORIGINS` (or `FRONTEND_URL`) includes the origin the user sees (e.g. `https://www.salvioris.com`). See [PRODUCTION_SECURITY.md](./PRODUCTION_SECURITY.md) and [PRODUCTION_FIX_SIGNIN.md](./PRODUCTION_FIX_SIGNIN.md).
- **Still seeing Cloudflare:** Clear browser cache and wait for DNS propagation; confirm nameservers are no longer Cloudflare’s.
