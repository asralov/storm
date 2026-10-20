package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asralov/storm/internal/runs"
	"github.com/asralov/storm/internal/storage"
)

func TestHistoryAPI(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "history.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewServer(db)
	defer s.Close()
	now := time.Now()
	id := fmt.Sprintf("run_%032x", 1)
	r := runs.Run{ID: id, Status: "completed", StartedAt: &now, CompletedAt: &now, Config: runs.Config{Mode: "standard", URL: "http://127.0.0.1:8081/fast", Method: "GET"}}
	if err = db.Save(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/history?limit=1&offset=0", "/api/history/" + id} {
		w := request(t, s, "GET", path, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), id) {
			t.Fatalf("history read: %d %s", w.Code, w.Body)
		}
	}
	for _, path := range []string{"/api/history?limit=101", "/api/history?offset=-1", "/api/history?limit=x", "/api/history?limit=1&limit=2", "/api/history?unknown=1", "/api/history/bad"} {
		if w := request(t, s, "GET", path, nil); w.Code != 400 {
			t.Fatalf("validation: %s %d", path, w.Code)
		}
	}
	bad := httptest.NewRequest("DELETE", "http://localhost:8080/api/history/"+id, nil)
	bad.Header.Set("Origin", "https://evil.example")
	bad.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, bad)
	if w.Code != 403 {
		t.Fatal("cross-origin delete accepted")
	}
	bad = httptest.NewRequest("DELETE", "http://localhost:8080/api/history/"+id, nil)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, bad)
	if w.Code != 415 {
		t.Fatal("non-JSON mutation accepted")
	}
	if w = request(t, s, "DELETE", "/api/history/"+id, nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request(t, s, "GET", "/api/history/"+id, nil); w.Code != 404 {
		t.Fatal("deleted run not missing")
	}
	db.Close()
	if w = request(t, s, "GET", "/api/history", nil); w.Code != 503 {
		t.Fatal("database failure hidden")
	}
}

func TestExecutedRunIsPersisted(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "history.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewServer(db)
	defer s.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer target.Close()
	w := request(t, s, "POST", "/api/tests", runs.Config{URL: target.URL, Method: "GET", Workers: 1, RPS: 20, DurationSeconds: 1, Authorized: true})
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	r := decodeRun(t, w)
	deadline := time.Now().Add(4 * time.Second)
	for {
		r = decodeRun(t, request(t, s, "GET", "/api/tests/"+r.ID, nil))
		if r.Persistence != nil && r.Persistence.Status == "saved" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("not saved: %+v", r.Persistence)
		}
		time.Sleep(10 * time.Millisecond)
	}
	w = request(t, s, "GET", "/api/history/"+r.ID, nil)
	saved := decodeRun(t, w)
	if w.Code != 200 || saved.Summary == nil || saved.Summary.Requests != r.Summary.Requests || len(saved.Intervals) == 0 {
		t.Fatalf("report not persisted: %+v", saved)
	}
}
