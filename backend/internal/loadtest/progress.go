package loadtest

import "time"

// Progress is an immutable sample, not an estimate between samples. Completed
// counts include all recorded outcomes, including errors and cancellation.
type Progress struct {
	Status             string    `json:"status"`
	SampledAt          time.Time `json:"sampled_at"`
	ElapsedSeconds     float64   `json:"elapsed_seconds"`
	LoadElapsedSeconds float64   `json:"load_elapsed_seconds"`
	DurationSeconds    float64   `json:"duration_seconds"`
	TargetRPS          int       `json:"target_rps"`
	StartedRequests    int       `json:"started_requests"`
	CompletedRequests  int       `json:"completed_requests"`
	Successes          int       `json:"successes"`
	Failures           int       `json:"failures"`
	MissedSchedules    int       `json:"missed_schedules"`
	// StartedRequests / LoadElapsedSeconds, frozen when scheduling ends.
	ObservedStartRPS float64 `json:"observed_start_rps"`
	// Outcomes recorded since the previous engine sample / sample interval.
	RecentThroughput      float64 `json:"recent_throughput"`
	SampleIntervalSeconds float64 `json:"sample_interval_seconds"`
}

func makeProgress(status string, now time.Time, elapsed, active time.Duration, cfg Config, started, missed int, summary Summary, interval time.Duration, completed int) Progress {
	p := Progress{Status: status, SampledAt: now, ElapsedSeconds: elapsed.Seconds(), LoadElapsedSeconds: active.Seconds(), DurationSeconds: cfg.Duration.Seconds(), TargetRPS: cfg.RPS, StartedRequests: started, CompletedRequests: summary.Requests, Successes: summary.Successes, Failures: summary.Failures, MissedSchedules: missed, SampleIntervalSeconds: interval.Seconds()}
	if active > 0 {
		p.ObservedStartRPS = float64(started) / active.Seconds()
	}
	if interval > 0 {
		p.RecentThroughput = float64(completed) / interval.Seconds()
	}
	return p
}
