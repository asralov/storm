package webui

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

var hashedAsset = regexp.MustCompile(`^assets/[^/]+-[A-Za-z0-9_-]{8,}\.[a-zA-Z0-9]+$`)

func Handler(assets fs.FS, api http.Handler, address string) http.Handler {
	_, port, _ := net.SplitHostPort(address)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		if r.Host != address && r.Host != "localhost:"+port {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(403)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid application Host"})
			return
		}
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", 405)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if strings.Contains(name, "\\") || (name != "" && (!fs.ValidPath(name) || path.Clean(name) != name)) {
			http.NotFound(w, r)
			return
		}
		if name == "" {
			name = "index.html"
		}
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			if path.Ext(name) != "" || strings.HasPrefix(name, "assets/") || !strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
			data, err = fs.ReadFile(assets, name)
			if err != nil {
				http.Error(w, "dashboard unavailable", 503)
				return
			}
		}
		if hashedAsset.MatchString(name) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}
