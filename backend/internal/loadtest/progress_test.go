package loadtest

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProgressRates(t *testing.T) {
	p := makeProgress("running", time.Now(), 3*time.Second, 2*time.Second, Config{Duration: 5 * time.Second, RPS: 30}, 40, 3, Summary{Requests: 30, Successes: 28, Failures: 2}, 500*time.Millisecond, 7)
	if p.ObservedStartRPS != 20 || p.RecentThroughput != 14 || p.CompletedRequests != 30 || p.SampleIntervalSeconds != .5 || p.LoadElapsedSeconds != 2 {
		t.Fatalf("wrong rates: %+v", p)
	}
	p = makeProgress("pending", time.Now(), 0, 0, Config{}, 0, 0, Summary{}, 0, 0)
	if math.IsNaN(p.ObservedStartRPS) || math.IsNaN(p.RecentThroughput) || p.ObservedStartRPS != 0 {
		t.Fatalf("invalid empty rate: %+v", p)
	}
}

func TestSlowProgressConsumerDoesNotBlockFinalResults(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer target.Close()
	updates := make(chan Progress, 1) // Deliberately never read until execution finishes.
	start := time.Now()
	s := RunWithProgress(context.Background(), Config{URL: target.URL, Workers: 2, Duration: 1100 * time.Millisecond, RPS: 20}, updates)
	if time.Since(start) > 3*time.Second || s.Successes == 0 || s.StartedRequests != s.Requests {
		t.Fatalf("blocked or lost results: %+v", s)
	}
	if len(updates) != 1 {
		t.Fatal("progress was not bounded")
	}
}

func TestLiveProgressAccounting(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer target.Close()
	updates := make(chan Progress, 1)
	final := make(chan Summary, 1)
	go func() {
		final <- RunWithProgress(context.Background(), Config{URL: target.URL, Workers: 2, Duration: 1500 * time.Millisecond, RPS: 20}, updates)
	}()
	sawLive := false
	for {
		select {
		case p := <-updates:
			if p.CompletedRequests != p.Successes+p.Failures || p.StartedRequests < p.CompletedRequests || p.LoadElapsedSeconds > 1.5 {
				t.Fatalf("invalid sample: %+v", p)
			}
			if p.Status == "running" && p.LoadElapsedSeconds >= .5 {
				sawLive = true
				if p.ObservedStartRPS != float64(p.StartedRequests)/p.LoadElapsedSeconds {
					t.Fatal("incorrect start rate")
				}
			}
		case s := <-final:
			if !sawLive || s.Requests == 0 {
				t.Fatal("missing live sample or final report")
			}
			return
		case <-time.After(4 * time.Second):
			t.Fatal("engine did not finish")
		}
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("redirect followed") }))
	defer destination.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer target.Close()
	s := Run(context.Background(), Config{URL: target.URL, Workers: 1, Duration: 150 * time.Millisecond, RPS: 20})
	if s.HTTPErrorCount == 0 || s.Successes != 0 {
		t.Fatalf("bad redirect result: %+v", s)
	}
	if conn, err := dialLoopback(context.Background(), "tcp", "192.0.2.1:80"); err == nil {
		conn.Close()
		t.Fatal("non-loopback dial allowed")
	}
}
