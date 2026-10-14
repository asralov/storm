package loadtest

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIntervalBoundariesAndLateDelivery(t *testing.T) {
	var c intervalCollector
	c.record(Result{StatusCode: 200, Latency: 10 * time.Millisecond}, 500*time.Millisecond)
	c.record(Result{StatusCode: 200, Latency: 100 * time.Millisecond}, time.Second) // Exactly at boundary belongs to next window.
	c.record(Result{StatusCode: 500}, 1500*time.Millisecond)
	first := c.snapshot(2*time.Second, false)
	if len(first) != 2 || first[0].Completed != 1 || first[1].Completed != 2 {
		t.Fatalf("bad boundaries: %+v", first)
	}
	c.record(Result{StatusCode: 200, Latency: 30 * time.Millisecond}, 900*time.Millisecond) // Delayed channel delivery.
	c.record(Result{Err: errors.New("network")}, 2100*time.Millisecond)
	c.record(Result{Err: context.Canceled}, 2200*time.Millisecond)
	final := c.snapshot(2500*time.Millisecond, true)
	if len(final) != 3 || final[0].Completed != 2 || *final[0].Average != 20 || *final[0].P50 != 10 || *final[0].P95 != 30 || *final[0].P99 != 30 {
		t.Fatalf("bad late update: %+v", final)
	}
	if first[0].Completed != 1 || *first[0].Average != 10 {
		t.Fatal("published snapshot was mutated")
	}
	if final[2].DurationSeconds != .5 || final[2].Throughput != 4 || final[2].NetworkErrors != 1 || final[2].Canceled != 1 || final[2].Average != nil {
		t.Fatalf("bad partial: %+v", final[2])
	}
	if final[1].HTTPErrors != 1 || final[1].Successes != 1 {
		t.Fatal("incorrect split failures")
	}
}

func TestIntervalDistributionAndEmptyWindow(t *testing.T) {
	var c intervalCollector
	for i := 100; i >= 1; i-- {
		c.record(Result{StatusCode: 200, Latency: time.Duration(i) * time.Millisecond}, 200*time.Millisecond)
	}
	points := c.snapshot(2*time.Second, false)
	if *points[0].P50 != 50 || *points[0].P95 != 95 || *points[0].P99 != 99 || *points[0].Average != 50.5 {
		t.Fatalf("bad nearest ranks: %+v", points[0])
	}
	if points[1].Completed != 0 || points[1].P50 != nil || points[1].Throughput != 0 {
		t.Fatal("empty window is not explicit")
	}
	copy := CloneIntervals(points)
	*copy[0].P50 = 999
	if *points[0].P50 != 50 {
		t.Fatal("snapshot clone aliases values")
	}
}

func TestIntervalRetention(t *testing.T) {
	var c intervalCollector
	for i := 0; i < MaxIntervals+10; i++ {
		c.record(Result{StatusCode: 200, Latency: time.Millisecond}, time.Duration(i)*time.Second)
		c.snapshot(time.Duration(i+1)*time.Second, false)
	}
	points := c.snapshot(time.Duration(MaxIntervals+10)*time.Second, false)
	if len(points) != MaxIntervals || len(c.buckets) > MaxIntervals || points[0].Sequence != 11 {
		t.Fatal("history retention is not bounded")
	}
}
