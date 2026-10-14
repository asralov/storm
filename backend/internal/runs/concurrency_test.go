package runs

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSimultaneousStartsReserveOneSlot(t *testing.T) {
	m := manager(t)
	cfg := config("http://127.0.0.1:1")
	cfg.DurationSeconds = 5
	var wg sync.WaitGroup
	var accepted atomic.Int64
	gate := make(chan struct{})
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			_, err := m.Start(cfg)
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrBusy) {
				t.Error(err)
			}
		}()
	}
	close(gate)
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted %d simultaneous starts", accepted.Load())
	}
}

func TestShutdownCancelsOutstandingRequest(t *testing.T) {
	entered := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer target.Close()
	m := manager(t)
	cfg := config(target.URL)
	cfg.DurationSeconds = 5
	run, err := m.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request never started")
	}
	start := time.Now()
	m.Close()
	if time.Since(start) > 1500*time.Millisecond {
		t.Fatal("shutdown did not cancel promptly")
	}
	run, err = m.Get(run.ID)
	if err != nil || run.Status != "canceled" {
		t.Fatalf("shutdown run: %+v %v", run, err)
	}
	assertAccounting(t, run)
}
