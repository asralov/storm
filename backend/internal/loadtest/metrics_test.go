package loadtest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestLatencyPercentiles(t *testing.T) {
	hundred := make([]time.Duration, 100)
	for i := range hundred {
		hundred[i] = time.Duration(i+1) * time.Millisecond
	}
	tests := []struct {
		name    string
		samples []time.Duration
		want    [3]float64
	}{
		{"1 to 100 ms", hundred, [3]float64{50, 95, 99}},
		{"single fractional sample", []time.Duration{1250 * time.Microsecond}, [3]float64{1.25, 1.25, 1.25}},
		{"empty", nil, [3]float64{}},
		{"identical", []time.Duration{7 * time.Millisecond, 7 * time.Millisecond, 7 * time.Millisecond}, [3]float64{7, 7, 7}},
		{"unsorted odd count", []time.Duration{9 * time.Millisecond, time.Millisecond, 5 * time.Millisecond, 3 * time.Millisecond, 7 * time.Millisecond}, [3]float64{5, 9, 9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var summary Summary
			for _, latency := range tt.samples {
				summary.Record(Result{StatusCode: 200, Latency: latency})
			}
			// Repeated finalization must preserve the distribution and counts.
			for range 2 {
				summary.Calculate()
				got := [3]float64{summary.P50LatencyMs, summary.P95LatencyMs, summary.P99LatencyMs}
				if got != tt.want {
					t.Fatalf("percentiles = %v, want %v", got, tt.want)
				}
				if summary.Requests != len(tt.samples) || summary.Successes != len(tt.samples) || summary.Failures != 0 {
					t.Fatalf("unexpected accounting: %+v", summary)
				}
			}
		})
	}
}

func TestLatencyPercentilesExcludeFailures(t *testing.T) {
	failures := []Result{
		{StatusCode: 302, Latency: time.Second},
		{StatusCode: 404, Latency: time.Second},
		{StatusCode: 500, Latency: time.Second},
		{Err: errors.New("connection refused"), Latency: time.Second},
		// A 2xx response with an incomplete body is still a failure.
		{StatusCode: 200, Err: errors.New("body read failed"), Latency: time.Second},
		{Err: context.Canceled, Latency: time.Second},
		{Err: context.DeadlineExceeded, Latency: time.Second},
	}
	for _, withSuccess := range []bool{false, true} {
		summary := Summary{Elapsed: 2 * time.Second, LoadWindowSeconds: 1}
		for _, result := range failures {
			summary.Record(result)
		}
		wantSuccesses := 0
		wantPercentile := float64(0)
		if withSuccess {
			summary.Record(Result{StatusCode: 200, Latency: 2 * time.Millisecond})
			summary.Record(Result{StatusCode: 299, Latency: 2 * time.Millisecond})
			wantSuccesses = 2
			wantPercentile = 2
		}
		summary.Calculate()
		if summary.P50LatencyMs != wantPercentile || summary.P95LatencyMs != wantPercentile || summary.P99LatencyMs != wantPercentile {
			t.Fatalf("failures entered latency distribution: %+v", summary)
		}
		if len(summary.latencies) != wantSuccesses || summary.Successes != wantSuccesses || summary.Failures != len(failures) || summary.HTTPErrorCount != 3 || summary.NetworkErrors != 3 || summary.Canceled != 1 {
			t.Fatalf("unexpected classification: %+v", summary)
		}
		if summary.Requests != summary.Successes+summary.Failures || summary.Failures != summary.HTTPErrorCount+summary.NetworkErrors+summary.Canceled {
			t.Fatalf("request accounting mismatch: %+v", summary)
		}
		if summary.AvgLatencyMs != wantPercentile || summary.Throughput != float64(summary.Requests)/2 || summary.AttemptedRPS != float64(summary.Requests) || summary.ElapsedSeconds != 2 {
			t.Fatalf("existing calculations changed: %+v", summary)
		}
	}
}

func TestLatencyPercentileJSON(t *testing.T) {
	var summary Summary
	summary.Record(Result{StatusCode: 200, Latency: 1250 * time.Microsecond})
	summary.Calculate()
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]float64
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"p50_latency_ms", "p95_latency_ms", "p99_latency_ms"} {
		if value, ok := fields[key]; !ok || value != 1.25 {
			t.Errorf("JSON field %s = %v (present: %v), want 1.25", key, value, ok)
		}
	}
}
