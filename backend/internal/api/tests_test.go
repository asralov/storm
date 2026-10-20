package api

import (
	"bytes"
	"encoding/json"
	"github.com/asralov/storm/internal/runs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func request(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, "http://localhost:8080"+path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func decodeRun(t *testing.T, w *httptest.ResponseRecorder) runs.Run {
	t.Helper()
	var r runs.Run
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAsyncEndpoints(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer target.Close()
	s := NewServer()
	defer s.Close()
	cfg := runs.Config{URL: target.URL, Method: "GET", Workers: 2, DurationSeconds: 1, RPS: 20, Authorized: true}
	start := time.Now()
	w := request(t, s, "POST", "/api/tests", cfg)
	if w.Code != 202 || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("start must return immediately: %d %s", w.Code, w.Body)
	}
	r := decodeRun(t, w)
	if r.ID == "" || r.Summary != nil {
		t.Fatal("bad accepted response")
	}
	if w = request(t, s, "POST", "/api/tests", cfg); w.Code != 409 {
		t.Fatalf("second run: %d", w.Code)
	}
	if w = request(t, s, "GET", "/api/tests/active", nil); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(r.ID)) {
		t.Fatal("active route does not return active run")
	}
	deadline := time.Now().Add(4 * time.Second)
	for {
		w = request(t, s, "GET", "/api/tests/"+r.ID, nil)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		r = decodeRun(t, w)
		if r.CompletedAt != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("run did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r.Status != "completed" || r.Summary == nil || r.Summary.Successes == 0 || r.Summary.Requests != r.Summary.StartedRequests {
		t.Fatalf("invalid report: %+v", r)
	}
	if w = request(t, s, "GET", "/api/tests/active", nil); w.Body.String() != "{\"run\":null}\n" {
		t.Fatalf("unexpected active result: %s", w.Body)
	}
	if w = request(t, s, "POST", "/api/tests/"+r.ID+"/stop", nil); w.Code != 409 {
		t.Fatal("terminal stop must conflict")
	}
	if w = request(t, s, "GET", "/api/tests/missing", nil); w.Code != 404 {
		t.Fatal("missing run must be 404")
	}
	// Acknowledgment is separate from termination, and results remain retrievable.
	cfg.DurationSeconds = 5
	r = decodeRun(t, request(t, s, "POST", "/api/tests", cfg))
	w = request(t, s, "POST", "/api/tests/"+r.ID+"/stop", nil)
	if w.Code != 202 || !bytes.Contains(w.Body.Bytes(), []byte(`"cancellation_requested":true`)) {
		t.Fatalf("bad stop acknowledgment: %s", w.Body)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		final := decodeRun(t, request(t, s, "GET", "/api/tests/"+r.ID, nil))
		if final.CompletedAt != nil {
			if final.Status != "canceled" || final.Summary == nil {
				t.Fatalf("lost canceled report: %+v", final)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stop never reached terminal state")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestBrowserSecurity(t *testing.T) {
	s := NewServer()
	defer s.Close()
	for _, tc := range []struct {
		name, host, origin, site, content string
		want                              int
	}{
		{"same origin", "localhost:5173", "http://localhost:5173", "same-origin", "application/json", 404},
		{"CLI", "127.0.0.1:8080", "", "", "application/json", 404},
		{"evil origin", "localhost:8080", "https://evil.example", "cross-site", "application/json", 403},
		{"null origin", "localhost:8080", "null", "", "application/json", 403},
		{"other local port", "localhost:8080", "http://localhost:9999", "same-site", "application/json", 403},
		{"rebinding", "evil.example:8080", "http://evil.example:8080", "same-origin", "application/json", 403},
		{"fetch metadata", "localhost:8080", "", "cross-site", "application/json", 403},
		{"form", "localhost:8080", "", "", "application/x-www-form-urlencoded", 415},
		{"text", "localhost:8080", "", "", "text/plain", 415},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://localhost:8080/api/tests/missing/stop", bytes.NewBufferString("{}"))
			r.Host = tc.host
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if tc.site != "" {
				r.Header.Set("Sec-Fetch-Site", tc.site)
			}
			r.Header.Set("Content-Type", tc.content)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d want %d: %s", w.Code, tc.want, w.Body)
			}
			var body map[string]any
			if json.Unmarshal(w.Body.Bytes(), &body) != nil || body["error"] == nil || w.Header().Get("Content-Type") != "application/json" {
				t.Fatal("error must be JSON")
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unexpected CORS permission")
			}
		})
	}
}

func TestValidation(t *testing.T) {
	s := NewServer()
	defer s.Close()
	for _, change := range []func(*runs.Config){
		func(c *runs.Config) { c.URL = "http://example.com" }, func(c *runs.Config) { c.URL = "https://127.0.0.1" },
		func(c *runs.Config) { c.Authorized = false }, func(c *runs.Config) { c.Workers = 11 },
		func(c *runs.Config) { c.RPS = 101 }, func(c *runs.Config) { c.DurationSeconds = 31 }, func(c *runs.Config) { c.Method = "POST" },
	} {
		cfg := runs.Config{URL: "http://127.0.0.1:1", Method: "GET", Workers: 1, DurationSeconds: 1, RPS: 20, Authorized: true}
		change(&cfg)
		w := request(t, s, "POST", "/api/tests", cfg)
		if w.Code != 400 {
			t.Fatalf("invalid config accepted: %+v %d", cfg, w.Code)
		}
	}
	if s.manager.Active() != nil {
		t.Fatal("validation launched a run")
	}
}
