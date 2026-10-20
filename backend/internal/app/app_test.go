package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/asralov/storm/internal/storage"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func launch(t *testing.T, path string) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- Run(ctx, Options{DatabasePath: path, Ready: func(u string) { ready <- u }}) }()
	select {
	case u := <-ready:
		return u, cancel, done
	case err := <-done:
		cancel()
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("startup timeout")
	}
	return "", nil, nil
}
func finish(t *testing.T, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown timeout")
	}
}
func TestLifecycleAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.sqlite3")
	url, cancel, done := launch(t, path)
	started := make(chan struct{}, 2)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
			w.Write([]byte("ok"))
		}
	}))
	defer target.Close()
	response, err := http.Post(url+"/api/tests", "application/json", strings.NewReader(fmt.Sprintf(`{"url":%q,"method":"GET","workers":2,"rps":10,"duration_seconds":30,"authorized":true}`, target.URL)))
	if err != nil {
		finish(t, cancel, done)
		t.Fatal(err)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(response.Body).Decode(&created)
	response.Body.Close()
	if response.StatusCode != 202 {
		finish(t, cancel, done)
		t.Fatalf("start: %d", response.StatusCode)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		finish(t, cancel, done)
		t.Fatal("target request never started")
	}
	finish(t, cancel, done)
	store, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.Get(context.Background(), created.ID)
	store.Close()
	if err != nil || saved.Status != "canceled" {
		t.Fatalf("final persisted: %+v %v", saved, err)
	}
	_, cancel, done = launch(t, path)
	finish(t, cancel, done)
}
func TestConflictsAndInvalidConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.sqlite3")
	_, cancel, done := launch(t, path)
	defer finish(t, cancel, done)
	if err := Run(context.Background(), Options{DatabasePath: path}); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("lock: %v", err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := Run(context.Background(), Options{Port: listener.Addr().(*net.TCPAddr).Port, DatabasePath: path}); err == nil || !strings.Contains(err.Error(), "cannot listen") {
		t.Fatalf("port: %v", err)
	}
	if err := Run(context.Background(), Options{Port: -1}); err == nil {
		t.Fatal("invalid port accepted")
	}
	if err := Run(context.Background(), Options{DatabasePath: "relative"}); err == nil {
		t.Fatal("relative path accepted")
	}
}
