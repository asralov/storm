package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yourusername/storm/internal/runs"
)

func TestSteppedAPICompatibility(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer target.Close()
	s := NewServer()
	defer s.Close()
	cfg := runs.Config{Mode: "stepped", URL: target.URL, Method: "GET", Workers: 2, Authorized: true, StartRPS: 10, RPSIncrement: 10, MaxRPS: 20, StageDurationSeconds: 1}
	w := request(t, s, "POST", "/api/tests", cfg)
	if w.Code != 202 {
		t.Fatalf("start: %d %s", w.Code, w.Body)
	}
	r := decodeRun(t, w)
	if r.Config.Mode != "stepped" || r.TotalStages != 2 || len(r.Stages) != 2 || r.PlannedLoadSeconds != 2 {
		t.Fatalf("bad plan: %+v", r)
	}
	standard := runs.Config{URL: target.URL, Method: "GET", Workers: 1, DurationSeconds: 1, RPS: 10, Authorized: true}
	if w = request(t, s, "POST", "/api/tests", standard); w.Code != 409 {
		t.Fatalf("standard run overlapped stepped job: %d", w.Code)
	}
	deadline := time.Now().Add(4 * time.Second)
	for {
		r = decodeRun(t, request(t, s, "GET", "/api/tests/"+r.ID, nil))
		if r.CompletedAt != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stepped job did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if r.Status != "completed" || r.Summary != nil || r.CurrentStage != 2 {
		t.Fatalf("bad final job: %+v", r)
	}
	for _, stage := range r.Stages {
		if stage.Summary == nil || stage.Summary.Requests == 0 || stage.Summary.StartedRequests != stage.Summary.Requests || stage.ActualStartRPS == nil {
			t.Fatalf("bad final stage: %+v", stage)
		}
	}
	cfg.RPSIncrement = 0
	if w = request(t, s, "POST", "/api/tests", cfg); w.Code != 400 {
		t.Fatalf("invalid stepped plan: %d", w.Code)
	}
}
