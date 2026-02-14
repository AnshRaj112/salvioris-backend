package clientip

import (
	"net"
	"net/http"
	"strings"
)

// RealClientIP returns the real client IP, trusting Cloudflare proxy.
// Uses CF-Connecting-IP when present, otherwise falls back to RemoteAddr
// parsed with net.SplitHostPort. Use for rate limiting and logging behind Cloudflare.
func RealClientIP(r *http.Request) string {
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
		return cf
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return strings.TrimSpace(host)
}
