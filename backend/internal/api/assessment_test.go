package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAnalysisAPIValidationAndCompatibility(t *testing.T) {
	s := NewServer()
	defer s.Close()
	base := map[string]any{"mode": "stepped", "url": "http://127.0.0.1:1", "method": "GET", "workers": 1, "start_rps": 20, "rps_increment": 10, "max_rps": 20, "stage_duration_seconds": 1, "authorized": true}
	valid := map[string]any{"max_p95_latency_ms": 200, "max_error_rate_percent": 1, "min_start_rate_percent": 95}
	for _, invalid := range []any{map[string]any{}, map[string]any{"max_p95_latency_ms": 200, "max_error_rate_percent": 1}, map[string]any{"max_p95_latency_ms": 0, "max_error_rate_percent": 1, "min_start_rate_percent": 95}, map[string]any{"max_p95_latency_ms": 200, "max_error_rate_percent": 101, "min_start_rate_percent": 95}} {
		base["thresholds"] = invalid
		if w := request(t, s, "POST", "/api/tests", base); w.Code != 400 {
			t.Fatalf("invalid thresholds: %d %s", w.Code, w.Body)
		}
	}
	standard := map[string]any{"url": "http://127.0.0.1:1", "method": "GET", "workers": 1, "rps": 20, "duration_seconds": 1, "authorized": true, "thresholds": valid}
	if w := request(t, s, "POST", "/api/tests", standard); w.Code != 400 {
		t.Fatal("standard accepted analysis thresholds")
	}
	delete(standard, "thresholds")
	w := request(t, s, "POST", "/api/tests", standard)
	if w.Code != 202 || bytes.Contains(w.Body.Bytes(), []byte(`"analysis":`)) {
		t.Fatalf("legacy standard changed: %d %s", w.Code, w.Body)
	}
	id := decodeRun(t, w).ID
	request(t, s, "POST", "/api/tests/"+id+"/stop", nil)
	s.Close()
	// A real completed job with too few outcomes is explicitly inconclusive.
	s = NewServer()
	defer s.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer target.Close()
	base["url"], base["thresholds"] = target.URL, valid
	w = request(t, s, "POST", "/api/tests", base)
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	r := decodeRun(t, w)
	deadline := time.Now().Add(3 * time.Second)
	for r.CompletedAt == nil {
		if time.Now().After(deadline) {
			t.Fatal("run did not finish")
		}
		time.Sleep(5 * time.Millisecond)
		r = decodeRun(t, request(t, s, "GET", "/api/tests/"+r.ID, nil))
	}
	if r.Analysis == nil || r.Analysis.Stages[0].Status != "INCONCLUSIVE" || r.Analysis.Breakpoint.Bounded {
		t.Fatalf("bad analysis JSON: %+v", r.Analysis)
	}
	delete(base, "thresholds")
	w = request(t, s, "POST", "/api/tests", base)
	if w.Code != 202 || bytes.Contains(w.Body.Bytes(), []byte(`"analysis":`)) {
		t.Fatal("legacy stepped unexpectedly assessed")
	}
}
