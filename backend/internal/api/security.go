package api

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Reject DNS-rebinding Hosts and cross-origin browser access. Vite preserves the
// original Host/Origin on its same-origin /api proxy (changeOrigin must stay false).
// CLI clients without browser headers remain supported; this is not authentication.
func safeRequest(w http.ResponseWriter, r *http.Request) bool {
	u, err := url.Parse("http://" + r.Host)
	if err != nil || u.Host != r.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		writeError(w, 403, "invalid Host")
		return false
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		writeError(w, 403, "local Host required")
		return false
	}
	// Compare the whole origin, including the port: another localhost application
	// is a different origin and must not be allowed to control this engine.
	origins := r.Header.Values("Origin")
	if len(origins) > 1 || (len(origins) == 1 && origins[0] != "http://"+r.Host) {
		writeError(w, 403, "cross-origin requests are not allowed")
		return false
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		writeError(w, 403, "cross-site requests are not allowed")
		return false
	}
	if r.Method == http.MethodPost || r.Method == http.MethodDelete {
		kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || kind != "application/json" {
			writeError(w, 415, "Content-Type must be application/json")
			return false
		}
	}
	return true
}
