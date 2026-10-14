package loadtest

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestProgressAverageLatency(t *testing.T) {
	var summary Summary
	sample := func() Progress {
		return makeProgress("running", time.Now(), time.Second, time.Second, Config{}, summary.Requests, 0, summary, time.Second, summary.Requests)
	}
	empty := sample()
	if empty.AvgLatencyMs != nil {
		t.Fatal("empty latency must be unavailable")
	}
	data, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if value, ok := fields["avg_latency_ms"]; !ok || value != nil {
		t.Fatal("empty latency must serialize as null")
	}
	summary.Record(Result{StatusCode: 500, Latency: time.Second})
	summary.Record(Result{StatusCode: 200, Err: errors.New("incomplete body"), Latency: time.Second})
	if sample().AvgLatencyMs != nil {
		t.Fatal("failures polluted latency")
	}
	summary.Record(Result{StatusCode: 200, Latency: 1250 * time.Microsecond})
	first := sample()
	summary.Record(Result{StatusCode: 204, Latency: 2750 * time.Microsecond})
	second := sample()
	if *first.AvgLatencyMs != 1.25 || *second.AvgLatencyMs != 2 {
		t.Fatalf("incorrect cumulative averages: %v %v", *first.AvgLatencyMs, *second.AvgLatencyMs)
	}
	summary.Calculate()
	if *second.AvgLatencyMs != summary.AvgLatencyMs {
		t.Fatal("live and final average disagree")
	}
}
