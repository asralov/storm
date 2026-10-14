package lab

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/yourusername/storm/internal/loadtest"
)

func call(s *Server, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

func TestBasicEndpoints(t *testing.T) {
	s := NewServer()
	for _, path := range []string{"/health", "/fast", "/slow?ms=0"} {
		w := call(s, path)
		if w.Code != 200 || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, path := range []string{"/slow?ms=-1", "/slow?ms=1501", "/slow?ms=no", "/errors?percent=101", "/variable?seq=-1"} {
		if w := call(s, path); w.Code != 400 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	start := time.Now()
	w := call(s, "/slow?ms=50")
	elapsed := time.Since(start)
	if w.Code != 200 || elapsed < 45*time.Millisecond || elapsed > time.Second {
		t.Fatalf("fixed delay: %s %d", elapsed, w.Code)
	}
}

func TestVariableDistribution(t *testing.T) {
	delays := make([]int, 100)
	for i := range delays {
		delays[i] = variableMilliseconds(uint64(i))
		if delays[i] != variableMilliseconds(uint64(i+100)) {
			t.Fatal("cycle not reproducible")
		}
	}
	slices.Sort(delays)
	if delays[49] != 10 || delays[94] != 100 || delays[98] != 400 || delays[99] != 900 {
		t.Fatalf("unexpected distribution: %v", delays)
	}
	s := NewServer()
	for _, seq := range []int{0, 8, 27} {
		w := call(s, "/variable?seq="+strconv.Itoa(seq))
		var body struct {
			Delay int `json:"delay_ms"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Delay != variableMilliseconds(uint64(seq)) {
			t.Fatal("endpoint differs from distribution")
		}
	}
}

func TestErrorMix(t *testing.T) {
	s := NewServer()
	failed := 0
	for i := 0; i < 100; i++ {
		if call(s, "/errors?percent=25&seq="+strconv.Itoa(i)).Code == 500 {
			failed++
		}
	}
	if failed != 25 {
		t.Fatalf("got %d failures", failed)
	}
	if call(s, "/errors?percent=0").Code != 200 || call(s, "/errors?percent=100").Code != 500 {
		t.Fatal("error percentage endpoints incorrect")
	}
}

func TestCancellationAndCapacity(t *testing.T) {
	s := NewServer()
	for _, path := range []string{"/slow?ms=1500", "/variable?seq=27", "/degrade"} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		r := httptest.NewRequest("GET", path, nil).WithContext(ctx)
		w := httptest.NewRecorder()
		start := time.Now()
		s.ServeHTTP(w, r)
		cancel()
		if time.Since(start) > 500*time.Millisecond || w.Body.Len() != 0 {
			t.Fatalf("%s ignored cancellation", path)
		}
	}
	// Occupy both service slots to deterministically test bounded queue timeout.
	s.degrade <- struct{}{}
	s.degrade <- struct{}{}
	start := time.Now()
	if w := call(s, "/degrade"); w.Code != 503 {
		t.Fatalf("queue overflow status: %d", w.Code)
	}
	if time.Since(start) < 450*time.Millisecond || time.Since(start) > time.Second {
		t.Fatal("queue timeout out of bounds")
	}
	<-s.degrade
	<-s.degrade
	if call(s, "/degrade").Code != 200 {
		t.Fatal("service slot not released")
	}
	for i := 0; i < cap(s.admitted); i++ {
		s.admitted <- struct{}{}
	}
	if call(s, "/fast").Code != 503 {
		t.Fatal("admission limit not enforced")
	}
}

func TestRealVariableWorkload(t *testing.T) {
	target := httptest.NewServer(NewServer())
	defer target.Close()
	s := loadtest.Run(context.Background(), loadtest.Config{URL: target.URL + "/variable", Workers: 10, RPS: 100, Duration: 2 * time.Second})
	if s.Successes < 80 || !(s.P50LatencyMs < s.P95LatencyMs && s.P95LatencyMs < s.P99LatencyMs) {
		t.Fatalf("variable workload percentiles: %+v", s)
	}
	if s.P50LatencyMs < 8 || s.P95LatencyMs < 80 || s.P99LatencyMs < 350 {
		t.Fatalf("unexpected real delays: %+v", s)
	}
	completed := 0
	for _, window := range s.Intervals {
		completed += window.Completed
	}
	if completed != s.Requests {
		t.Fatal("lab results lost at interval boundary")
	}
}

func TestRealHTTPErrorClassification(t *testing.T) {
	target := httptest.NewServer(NewServer())
	defer target.Close()
	s := loadtest.Run(context.Background(), loadtest.Config{URL: target.URL + "/errors?percent=100", Workers: 2, RPS: 20, Duration: 200 * time.Millisecond})
	if s.Requests == 0 || s.HTTPErrorCount != s.Requests || s.Successes != 0 || s.NetworkErrors != 0 {
		t.Fatalf("incorrect HTTP classification: %+v", s)
	}
	for _, window := range s.Intervals {
		if window.Average != nil || window.P99 != nil {
			t.Fatal("failed requests entered latency distribution")
		}
	}
}
