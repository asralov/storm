package loadtest

import (
	"context"
	"errors"
	"math"
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
	} else {
		// Includes redirects and HTTP 4xx/5xx.
		s.Failures++
		s.HTTPErrorCount++
	}
}

func (s *Summary) Calculate() {
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
