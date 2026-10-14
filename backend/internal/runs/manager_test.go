package runs

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func config(url string) Config {
	return Config{URL: url, Method: "GET", Workers: 2, DurationSeconds: 1, RPS: 20, Authorized: true}
}
func manager(t *testing.T) *Manager { t.Helper(); m := NewManager(); t.Cleanup(m.Close); return m }
func waitRun(t *testing.T, m *Manager, id string, predicate func(Run) bool) Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, err := m.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if predicate(r) {
			return r
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("run did not reach expected state")
	return Run{}
}
func terminal(r Run) bool { return r.CompletedAt != nil }
func assertAccounting(t *testing.T, r Run) {
	t.Helper()
	if r.Summary == nil {
		t.Fatal("missing final summary")
	}
	s := r.Summary
	var completed, successes, failures, httpErrors, networkErrors, canceled int
	for _, window := range r.Intervals {
		completed += window.Completed
		successes += window.Successes
		failures += window.Failures
		httpErrors += window.HTTPErrors
		networkErrors += window.NetworkErrors
		canceled += window.Canceled
	}
	if completed != s.Requests || successes != s.Successes || failures != s.Failures || httpErrors != s.HTTPErrorCount || networkErrors != s.NetworkErrors || canceled != s.Canceled {
		t.Fatalf("interval totals differ from final summary: %+v", r)
	}
	if s.StartedRequests != s.Requests || s.Requests != s.Successes+s.Failures || s.Failures != s.HTTPErrorCount+s.NetworkErrors+s.Canceled || s.ScheduledRequests != s.DispatchedRequests+s.MissedSchedules {
		t.Fatalf("invalid final accounting: %+v", s)
	}
	if r.Progress.CompletedRequests != s.Requests || r.Progress.StartedRequests != s.StartedRequests {
		t.Fatal("final progress lost results")
	}
}

func TestCompletionAndConcurrentReads(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer target.Close()
	m := manager(t)
	r, err := m.Start(config(target.URL))
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == "" || r.Status != "pending" {
		t.Fatalf("unexpected accepted run: %+v", r)
	}
	if active := m.Active(); active == nil || active.ID != r.ID {
		t.Fatal("active run missing")
	}
	if _, err = m.Start(config(target.URL)); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected conflict: %v", err)
	}
	var readers sync.WaitGroup
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				got, err := m.Get(r.ID)
				if err != nil {
					t.Error(err)
					return
				}
				if terminal(got) {
					return
				}
				_ = m.Active()
				time.Sleep(time.Millisecond)
			}
		}()
	}
	final := waitRun(t, m, r.ID, terminal)
	readers.Wait()
	if final.Status != "completed" || final.StartedAt == nil || final.CompletedAt.Before(*final.StartedAt) || final.Summary.Successes == 0 {
		t.Fatalf("bad completion: %+v", final)
	}
	assertAccounting(t, final)
	if len(final.Intervals) == 0 {
		t.Fatal("missing retained history")
	}
	before := final.Intervals[0].Completed
	final.Intervals[0].Completed = -1
	historyCopy, _ := m.Get(r.ID)
	if historyCopy.Intervals[0].Completed != before {
		t.Fatal("history snapshot aliases manager")
	}
	if m.Active() != nil {
		t.Fatal("completed run still active")
	}
	for range 10 {
		again, _ := m.Get(r.ID)
		if again.Summary.Requests != final.Summary.Requests || again.Status != final.Status {
			t.Fatal("final result changed")
		}
	}
	final.Summary.Requests = -1
	*final.StartedAt = time.Time{}
	again, _ := m.Get(r.ID)
	if again.Summary.Requests < 0 || again.StartedAt.IsZero() {
		t.Fatal("returned snapshot mutates manager")
	}
	if _, err = m.Stop(r.ID); !errors.Is(err, ErrFinished) {
		t.Fatalf("stop finished run: %v", err)
	}
}

func TestCancellationStopsSlowRequests(t *testing.T) {
	for _, phase := range []string{"running", "draining"} {
		t.Run(phase, func(t *testing.T) {
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
			cfg.Workers = 1
			if phase == "running" {
				cfg.DurationSeconds = 5
			}
			r, err := m.Start(cfg)
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("request did not start")
			}
			if phase == "draining" {
				waitRun(t, m, r.ID, func(r Run) bool { return r.Status == "draining" })
			}
			start := time.Now()
			ack, err := m.Stop(r.ID)
			if err != nil || !ack.CancellationRequested || terminal(ack) {
				t.Fatalf("bad stop acknowledgment: %+v %v", ack, err)
			}
			final := waitRun(t, m, r.ID, terminal)
			if time.Since(start) > 1500*time.Millisecond {
				t.Fatal("cancellation exceeded bound")
			}
			if final.Status != "canceled" || final.Summary.Canceled == 0 {
				t.Fatalf("not canceled: %+v", final)
			}
			assertAccounting(t, final)
		})
	}
}

func TestNormalDeadlineDrainsWithoutCancelingRun(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(1200 * time.Millisecond); w.WriteHeader(200) }))
	defer target.Close()
	m := manager(t)
	cfg := config(target.URL)
	cfg.Workers = 1
	r, err := m.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	draining := waitRun(t, m, r.ID, func(r Run) bool { return r.Status == "draining" })
	if draining.Progress.LoadElapsedSeconds != 1 {
		t.Fatalf("load clock should freeze at deadline: %+v", draining.Progress)
	}
	final := waitRun(t, m, r.ID, terminal)
	if final.Status != "completed" || final.CancellationRequested || final.Summary.Successes != 1 {
		t.Fatalf("normal drain confused with cancellation: %+v", final)
	}
	assertAccounting(t, final)
}

func TestCloseAndRetention(t *testing.T) {
	m := manager(t)
	var first string
	for i := 0; i < retainedRuns+2; i++ {
		r, err := m.Start(config("http://127.0.0.1:1"))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = r.ID
		}
		if _, err = m.Stop(r.ID); err != nil {
			t.Fatal(err)
		}
		waitRun(t, m, r.ID, terminal)
	}
	if _, err := m.Get(first); !errors.Is(err, ErrNotFound) {
		t.Fatal("old run was not evicted")
	}
	if len(m.runs) != retainedRuns {
		t.Fatalf("retention: %d", len(m.runs))
	}
	r, err := m.Start(config("http://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	m.Close()
	final, err := m.Get(r.ID)
	if err != nil || final.Status != "canceled" || final.Summary == nil {
		t.Fatalf("shutdown lost run: %+v %v", final, err)
	}
	if _, err = m.Start(config("http://127.0.0.1:1")); !errors.Is(err, ErrClosed) {
		t.Fatalf("start after close: %v", err)
	}
}
