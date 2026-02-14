package clientip

import (
	"net"
	"net/http"
	"strings"
)

// RealClientIP returns the client IP from the request.
// Uses r.RemoteAddr only (no proxy headers). Use for rate limiting and logging
// when traffic goes directly to the app (e.g. Vercel → Render, no CDN).
func RealClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return strings.TrimSpace(host)
}
