package loadtest

import (
	"context"
	"errors"
	"math"
	"slices"
	"time"
)

type Summary struct {
	Requests       int `json:"requests"`
	Successes      int `json:"successes"`
	Failures       int `json:"failures"`
	HTTPErrorCount int `json:"http_error_count"`
	NetworkErrors  int `json:"network_errors"`
	Canceled       int `json:"canceled"`

	ElapsedSeconds float64 `json:"elapsed_seconds"`
	Throughput     float64 `json:"throughput"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	P50LatencyMs   float64 `json:"p50_latency_ms"`
	P95LatencyMs   float64 `json:"p95_latency_ms"`
	P99LatencyMs   float64 `json:"p99_latency_ms"`

	// Only the collector records and finalizes these successful-response samples.
	// Keeping every sample is acceptable for the bounded MVP; high-volume tests
	// will need a histogram or another memory-efficient distribution.
	latencies []time.Duration

	Elapsed           time.Duration `json:"-"`
	totalLatency      time.Duration
	LoadWindowSeconds float64 `json:"load_window_seconds"`
	AttemptedRPS      float64 `json:"attempted_rps"`

	ScheduledRequests  int `json:"scheduled_requests"`
	DispatchedRequests int `json:"dispatched_requests"`
	StartedRequests    int `json:"started_requests"`
	MissedSchedules    int `json:"missed_schedules"`
}

func (s *Summary) Record(result Result) {
	s.Requests++

	if result.Err != nil {
		s.Failures++

		if errors.Is(result.Err, context.Canceled) ||
			errors.Is(result.Err, context.DeadlineExceeded) {
			s.Canceled++
		} else {
			s.NetworkErrors++
		}

		return
	}

	if result.StatusCode >= 200 && result.StatusCode < 300 {
		s.Successes++
		s.totalLatency += result.Latency
		s.latencies = append(s.latencies, result.Latency)
	} else {
		// Includes redirects and HTTP 4xx/5xx.
		s.Failures++
		s.HTTPErrorCount++
	}
}

func (s *Summary) Calculate() {
	// Sort once for all three nearest-rank percentiles.
	slices.Sort(s.latencies)
	s.P50LatencyMs = nearestRankLatencyMs(s.latencies, 0.50)
	s.P95LatencyMs = nearestRankLatencyMs(s.latencies, 0.95)
	s.P99LatencyMs = nearestRankLatencyMs(s.latencies, 0.99)

	s.ElapsedSeconds = s.Elapsed.Seconds()

	if s.ElapsedSeconds > 0 {
		s.Throughput = float64(s.Requests) /
			s.ElapsedSeconds
	}

	if s.Successes > 0 {
		avgMs := float64(s.totalLatency) /
			float64(s.Successes) /
			float64(time.Millisecond)

		s.AvgLatencyMs = math.Round(avgMs*100) / 100
	}

	if s.LoadWindowSeconds > 0 {
		s.AttemptedRPS = float64(s.Requests) /
			s.LoadWindowSeconds
	}
}

// nearestRankLatencyMs expects sorted samples and 0 < p <= 1.
// Zero denotes no samples when Successes is zero.
func nearestRankLatencyMs(samples []time.Duration, p float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	rank := int(math.Ceil(p * float64(len(samples))))
	return float64(samples[rank-1]) / float64(time.Millisecond)
}
