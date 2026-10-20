package loadtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSuccessfulRequests(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Storm test response"))
		}),
	)
	defer server.Close()

	summary := Run(context.Background(), Config{
		URL:      server.URL,
		Workers:  2,
		Duration: 500 * time.Millisecond,
	})

	if summary.Successes == 0 {
		t.Fatal("Expected successful requests")
	}

	if summary.HTTPErrorCount != 0 {
		t.Fatalf(
			"Expected zero HTTP errors, got %d",
			summary.HTTPErrorCount,
		)
	}

	if summary.NetworkErrors != 0 {
		t.Fatalf(
			"Expected zero network errors, got %d",
			summary.NetworkErrors,
		)
	}

	if summary.Requests != summary.Successes+summary.Failures {
		t.Fatalf(
			"Request accounting mismatch: %+v",
			summary,
		)
	}

	if summary.Failures != summary.Canceled {
		t.Fatalf(
			"Expected only cancellation-related failures: %+v",
			summary,
		)
	}
}

func TestInFlightRequestTimeoutDuringDrain(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(4 * time.Second):
				w.WriteHeader(http.StatusOK)
			}
		}),
	)
	defer server.Close()

	summary := Run(context.Background(), Config{
		URL:      server.URL,
		Workers:  2,
		Duration: 350 * time.Millisecond,
	})

	if summary.Requests == 0 {
		t.Fatal("Expected requests to be attempted")
	}

	if summary.NetworkErrors == 0 || summary.Canceled != 0 {
		t.Fatalf(
			"Expected transport timeouts, not cancellation, got %+v",
			summary,
		)
	}

	if summary.Requests != summary.Successes+summary.Failures {
		t.Fatalf(
			"Request accounting mismatch: %+v",
			summary,
		)
	}
}

func TestHTTPFailures(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	)
	defer server.Close()

	summary := Run(context.Background(), Config{
		URL:      server.URL,
		Workers:  1,
		Duration: 400 * time.Millisecond,
	})

	if summary.HTTPErrorCount == 0 {
		t.Fatal("Expected HTTP errors")
	}

	if summary.Successes != 0 {
		t.Fatal("Expected zero successful requests")
	}
}

func TestRequestAccounting(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			time.Sleep(20 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	summary := Run(context.Background(), Config{
		URL:      server.URL,
		Workers:  5,
		Duration: 500 * time.Millisecond,
	})

	if summary.Requests != summary.Successes+summary.Failures {
		t.Fatalf("Request accounting mismatch: %+v", summary)
	}
}

func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	summary := Run(ctx, Config{
		URL:      "http://127.0.0.1:8081",
		Workers:  2,
		Duration: 10 * time.Second,
	})

	if time.Since(start) > 2*time.Second {
		t.Fatal("Canceled test did not stop promptly")
	}

	if summary.Requests != 0 {
		t.Fatalf(
			"Expected no started requests, got %d",
			summary.Requests,
		)
	}
}

func TestGracefulShutdown(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			// Deliberately take longer than the load window.
			time.Sleep(350 * time.Millisecond)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("completed"))
		}),
	)
	defer server.Close()

	start := time.Now()

	summary := Run(context.Background(), Config{
		URL:      server.URL,
		Workers:  1,
		Duration: 200 * time.Millisecond,
	})

	elapsed := time.Since(start)

	if summary.Successes != 1 {
		t.Fatalf(
			"Expected 1 completed request, got %+v",
			summary,
		)
	}

	if summary.Failures != 0 {
		t.Fatalf(
			"Expected no failures, got %+v",
			summary,
		)
	}

	if elapsed < 400*time.Millisecond {
		t.Fatalf(
			"Expected graceful draining, finished in %v",
			elapsed,
		)
	}

	if summary.Requests != summary.Successes+summary.Failures {
		t.Fatal("Request accounting mismatch")
	}
}

func TestGlobalRPS(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	summary := Run(context.Background(), Config{
		URL:      server.URL,
		Workers:  5,
		Duration: 1 * time.Second,
		RPS:      20,
	})

	t.Logf(
		"Requests: %d, Successes: %d, Throughput: %.2f",
		summary.Requests,
		summary.Successes,
		summary.Throughput,
	)

	// Allow timing variation under system load.
	if summary.Requests < 10 || summary.Requests > 22 {
		t.Fatalf(
			"Expected approximately 20 requests, got %d",
			summary.Requests,
		)
	}

	if summary.Requests != summary.Successes+summary.Failures {
		t.Fatal("Request accounting mismatch")
	}

	if summary.HTTPErrorCount != 0 ||
		summary.NetworkErrors != 0 {
		t.Fatalf("Unexpected HTTP or network errors: %+v", summary)
	}
}

func TestSchedulerAccounting(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	summary := Run(context.Background(), Config{
		URL:      server.URL,
		Workers:  3,
		RPS:      40,
		Duration: time.Second,
	})

	if summary.ScheduledRequests !=
		summary.DispatchedRequests+summary.MissedSchedules {
		t.Fatalf(
			"Scheduler accounting mismatch: %+v",
			summary,
		)
	}

	if summary.StartedRequests > summary.DispatchedRequests {
		t.Fatalf(
			"Started more requests than dispatched: %+v",
			summary,
		)
	}

	if summary.StartedRequests != summary.Requests {
		t.Fatalf(
			"Request accounting mismatch: %+v",
			summary,
		)
	}

	if summary.Requests != summary.Successes+summary.Failures {
		t.Fatalf(
			"Result classification mismatch: %+v",
			summary,
		)
	}
}
