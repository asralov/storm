package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMalformedConfigurationsNeverStart(t *testing.T) {
	s := NewServer()
	defer s.Close()
	for _, body := range []string{`null`, `[]`, `{}`, `{"url":`, `{} {}`, `{"unknown":true}`, `{"url":"` + strings.Repeat("a", 5000) + `"}`} {
		r := httptest.NewRequest("POST", "http://localhost:8080/api/tests", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 400 || !strings.Contains(w.Header().Get("Content-Type"), "application/json") || s.manager.Active() != nil {
			t.Fatalf("malformed input: %d %s", w.Code, w.Body)
		}
	}
}
func TestCrossOriginMutationsRejected(t *testing.T) {
	s := NewServer()
	defer s.Close()
	for _, method := range []string{"POST", "DELETE"} {
		for _, origin := range []string{"https://evil.example", "null", "http://localhost:9999"} {
			r := httptest.NewRequest(method, "http://localhost:8080/api/history/run_00000000000000000000000000000000", strings.NewReader(`{}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("%s %s: %d", method, origin, w.Code)
			}
		}
	}
}
