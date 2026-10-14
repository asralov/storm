package lab

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// Server has no per-client state or background workers. Sequence counters are
// global per endpoint. seq= selects a reproducible position without advancing them.
type Server struct {
	mux      *http.ServeMux
	variable atomic.Uint64
	errors   atomic.Uint64
	admitted chan struct{}
	degrade  chan struct{}
}

func NewServer() *Server {
	s := &Server{mux: http.NewServeMux(), admitted: make(chan struct{}, 64), degrade: make(chan struct{}, 2)}
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"status": "ok", "service": "storm-lab"})
	})
	s.mux.HandleFunc("GET /fast", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]any{"ok": true}) })
	s.mux.HandleFunc("GET /slow", s.slow)
	s.mux.HandleFunc("GET /variable", s.variableDelay)
	s.mux.HandleFunc("GET /errors", s.errorMix)
	s.mux.HandleFunc("GET /degrade", s.degraded)
	return s
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Bound the number of admitted handlers, including waiting degraded requests.
	select {
	case s.admitted <- struct{}{}:
		defer func() { <-s.admitted }()
	default:
		respond(w, 503, map[string]string{"error": "lab capacity reached"})
		return
	}
	s.mux.ServeHTTP(w, r)
}

func number(w http.ResponseWriter, r *http.Request, key string, fallback, maximum int) (int, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 || v > maximum {
		respond(w, 400, map[string]string{"error": key + " must be an integer from 0 to " + strconv.Itoa(maximum)})
		return 0, false
	}
	return v, true
}

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}

func (s *Server) slow(w http.ResponseWriter, r *http.Request) {
	ms, ok := number(w, r, "ms", 100, 1500)
	if !ok {
		return
	}
	if wait(r.Context(), time.Duration(ms)*time.Millisecond) {
		respond(w, 200, map[string]any{"delay_ms": ms})
	}
}

func sequence(w http.ResponseWriter, r *http.Request, counter *atomic.Uint64) (uint64, bool) {
	if r.URL.Query().Has("seq") {
		v, ok := number(w, r, "seq", 0, 1000000)
		return uint64(v), ok
	}
	return counter.Add(1) - 1, true
}

// Every 100 arrivals: 80 at 10ms, 15 at 100ms, 4 at 400ms, 1 at 900ms.
// Multiplication permutes positions so slow samples are spread across the cycle.
func variableMilliseconds(seq uint64) int {
	position := (seq % 100 * 37) % 100
	switch {
	case position < 80:
		return 10
	case position < 95:
		return 100
	case position < 99:
		return 400
	default:
		return 900
	}
}

func (s *Server) variableDelay(w http.ResponseWriter, r *http.Request) {
	seq, ok := sequence(w, r, &s.variable)
	if !ok {
		return
	}
	ms := variableMilliseconds(seq)
	if wait(r.Context(), time.Duration(ms)*time.Millisecond) {
		respond(w, 200, map[string]any{"sequence": seq, "delay_ms": ms})
	}
}

func (s *Server) errorMix(w http.ResponseWriter, r *http.Request) {
	percent, ok := number(w, r, "percent", 20, 100)
	if !ok {
		return
	}
	seq, ok := sequence(w, r, &s.errors)
	if !ok {
		return
	}
	status := 200
	if int((seq%100*37)%100) < percent {
		status = 500
	}
	respond(w, status, map[string]any{"sequence": seq, "ok": status == 200, "error_percent": percent})
}

func (s *Server) degraded(w http.ResponseWriter, r *http.Request) {
	// Two service slots, each taking 100ms (~20 requests/s capacity). Admitted
	// excess work queues for at most 500ms; overload then returns 503.
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()
	select {
	case s.degrade <- struct{}{}:
		defer func() { <-s.degrade }()
	case <-ctx.Done():
		if r.Context().Err() == nil {
			respond(w, 503, map[string]string{"error": "degrade queue timeout"})
		}
		return
	}
	if wait(r.Context(), 100*time.Millisecond) {
		respond(w, 200, map[string]any{"service_ms": 100, "capacity": 2})
	}
}
