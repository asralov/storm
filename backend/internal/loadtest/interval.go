package loadtest

import (
	"slices"
	"time"
)

const MaxIntervals = 300

// Interval covers [start_seconds, elapsed_seconds). Outcomes are assigned by
// worker completion time, even if the collector receives them after a boundary.
type Interval struct {
	Sequence        int      `json:"sequence"`
	StartSeconds    float64  `json:"start_seconds"`
	ElapsedSeconds  float64  `json:"elapsed_seconds"`
	DurationSeconds float64  `json:"duration_seconds"`
	Completed       int      `json:"completed"`
	Successes       int      `json:"successes"`
	Failures        int      `json:"failures"`
	HTTPErrors      int      `json:"http_errors"`
	NetworkErrors   int      `json:"network_errors"`
	Canceled        int      `json:"canceled"`
	Throughput      float64  `json:"throughput"`
	Average         *float64 `json:"avg_latency_ms"`
	P50             *float64 `json:"p50_latency_ms"`
	P95             *float64 `json:"p95_latency_ms"`
	P99             *float64 `json:"p99_latency_ms"`
}

func CloneIntervals(values []Interval) []Interval {
	result := slices.Clone(values)
	copyValue := func(p *float64) *float64 {
		if p == nil {
			return nil
		}
		v := *p
		return &v
	}
	for i := range result {
		result[i].Average = copyValue(result[i].Average)
		result[i].P50 = copyValue(result[i].P50)
		result[i].P95 = copyValue(result[i].P95)
		result[i].P99 = copyValue(result[i].P99)
	}
	return result
}

type intervalBucket struct {
	summary Summary
	dirty   bool
}
type intervalCollector struct{ buckets map[int]*intervalBucket }

func (c *intervalCollector) record(result Result, elapsed time.Duration) {
	if c.buckets == nil {
		c.buckets = make(map[int]*intervalBucket)
	}
	index := max(0, int(elapsed/time.Second))
	b := c.buckets[index]
	if b == nil {
		b = &intervalBucket{}
		c.buckets[index] = b
	}
	b.summary.Record(result)
	b.dirty = true
}

func (c *intervalCollector) snapshot(elapsed time.Duration, final bool) []Interval {
	count := int(elapsed / time.Second)
	if final && elapsed%time.Second > 0 {
		count++
	}
	first := max(0, count-MaxIntervals)
	for index := range c.buckets {
		if index < first {
			delete(c.buckets, index)
		}
	}
	result := make([]Interval, 0, min(count, MaxIntervals))
	for index := first; index < count; index++ {
		end := min(time.Duration(index+1)*time.Second, elapsed)
		duration := end - time.Duration(index)*time.Second
		point := Interval{Sequence: index + 1, StartSeconds: float64(index), ElapsedSeconds: end.Seconds(), DurationSeconds: duration.Seconds()}
		if b := c.buckets[index]; b != nil {
			if b.dirty {
				b.summary.Calculate()
				b.dirty = false
			}
			s := b.summary
			point.Completed, point.Successes, point.Failures = s.Requests, s.Successes, s.Failures
			point.HTTPErrors, point.NetworkErrors, point.Canceled = s.HTTPErrorCount, s.NetworkErrors, s.Canceled
			point.Throughput = float64(s.Requests) / duration.Seconds()
			if s.Successes > 0 {
				point.Average, point.P50, point.P95, point.P99 = &s.AvgLatencyMs, &s.P50LatencyMs, &s.P95LatencyMs, &s.P99LatencyMs
			}
		}
		result = append(result, point)
	}
	return result
}
