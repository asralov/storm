package webui

import (
	"github.com/asralov/storm/internal/api"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	files := fstest.MapFS{"index.html": {Data: []byte("<html>Storm</html>")}, "assets/app-abcdefgh.js": {Data: []byte("console.log('Storm')")}}
	server := api.NewServer()
	defer server.Close()
	h := Handler(files, server, "127.0.0.1:8080")
	for _, tc := range []struct {
		path, host, method, origin string
		code                       int
		content                    string
	}{
		{"/", "127.0.0.1:8080", "GET", "", 200, "text/html"},
		{"/history/example", "localhost:8080", "GET", "", 200, "text/html"},
		{"/assets/app-abcdefgh.js", "127.0.0.1:8080", "GET", "", 200, "javascript"},
		{"/missing.js", "127.0.0.1:8080", "GET", "", 404, "text/plain"},
		{"/assets/missing", "127.0.0.1:8080", "GET", "", 404, "text/plain"},
		{"/../secret", "127.0.0.1:8080", "GET", "", 404, "text/plain"},
		{"/api/missing", "127.0.0.1:8080", "GET", "", 404, "application/json"},
		{"/api", "127.0.0.1:8080", "GET", "", 404, "application/json"},
		{"/api/health", "127.0.0.1:8080", "GET", "", 200, "application/json"},
		{"/api/tests", "127.0.0.1:8080", "POST", "https://evil.example", 403, "application/json"},
		{"/api/health", "evil.example:8080", "GET", "", 403, "application/json"},
		{"/api/health", "localhost:9999", "GET", "", 403, "application/json"},
		{"/", "127.0.0.1:8080", "POST", "", 405, "text/plain"},
	} {
		t.Run(tc.path+tc.host+tc.method+tc.origin, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, nil)
			r.Header.Set("Accept", "text/html")
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.code || !strings.Contains(w.Header().Get("Content-Type"), tc.content) {
				t.Fatalf("%d %s %s", w.Code, w.Header(), w.Body.String())
			}
			if strings.Contains(tc.path, "abcdefgh") && !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
				t.Fatal("asset cache missing")
			}
		})
	}
}
