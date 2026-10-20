package loadtest

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	URL      string
	Workers  int
	Duration time.Duration
	RPS      int
}

type Result struct {
	CompletedAt time.Time
	StatusCode  int
	Latency     time.Duration
	Err         error
}

type SchedulerStats struct {
	Scheduled  int
	Dispatched int
	Missed     int
}

const shutdownGracePeriod = 3 * time.Second

func Run(parent context.Context, cfg Config) Summary {
	return RunWithProgress(parent, cfg, nil)
}

// RunWithProgress never waits for a progress consumer. Final results are returned
// separately, so dropping a periodic snapshot cannot lose the final report.
func RunWithProgress(parent context.Context, cfg Config, updates chan<- Progress) Summary {
	start := time.Now()
	loadWindowEnded := make(chan time.Duration, 1)

	// Keep existing callers working with a safe default.
	if cfg.RPS == 0 {
		cfg.RPS = 20
	}

	// Defensive limits: API validation is not enough.
	if cfg.Workers < 1 || cfg.Workers > 10 ||
		cfg.RPS < 1 || cfg.RPS > 100 ||
		cfg.Duration <= 0 || cfg.Duration > 30*time.Second {
		return Summary{}
	}

	scheduleCtx, stopScheduling := context.WithTimeout(
		parent, cfg.Duration,
	)
	defer stopScheduling()

	requestCtx, cancelRequests := context.WithCancel(parent)
	defer cancelRequests()

	transport := &http.Transport{
		Proxy:               nil,
		DialContext:         dialLoopback,
		MaxIdleConns:        cfg.Workers,
		MaxIdleConnsPerHost: cfg.Workers,
		MaxConnsPerHost:     cfg.Workers,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: transport,
		CheckRedirect: func(
			req *http.Request,
			via []*http.Request,
		) error {
			return http.ErrUseLastResponse
		},
	}

	// Unbuffered: work is accepted only if a worker is free.
	jobs := make(chan struct{})
	results := make(chan Result, cfg.Workers)

	var wg sync.WaitGroup
	var requestsStarted atomic.Int64
	var missed atomic.Int64
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(scheduleCtx, requestCtx, client, cfg.URL, jobs, results, &requestsStarted)
		}()
	}

	schedulerDone := make(chan SchedulerStats, 1)

	go func() {
		schedulerDone <- scheduleRequests(
			scheduleCtx,
			cfg.RPS,
			jobs,
			&missed,
		)
	}()

	workersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(workersDone)
	}()

	// Coordinate graceful shutdown.
	go func() {
		<-scheduleCtx.Done()
		loadWindowEnded <- time.Since(start)

		timer := time.NewTimer(shutdownGracePeriod)
		defer timer.Stop()

		select {
		case <-workersDone:
		case <-parent.Done():
			cancelRequests()
		case <-timer.C:
			cancelRequests()
		}

		<-workersDone
		close(results)
	}()

	var summary Summary
	var intervals intervalCollector
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastSample := start
	lastCompleted := 0
	loadElapsed := time.Duration(0)
	loadEnded := false
	status := "running"
	publish := func() {
		now := time.Now()
		active := loadElapsed
		if !loadEnded {
			active = min(now.Sub(start), cfg.Duration)
		}
		progress := makeProgress(status, now, now.Sub(start), active, cfg,
			int(requestsStarted.Load()), int(missed.Load()), summary,
			now.Sub(lastSample), summary.Requests-lastCompleted)
		progress.Intervals = intervals.snapshot(now.Sub(start), false)
		select {
		case updates <- progress:
		default:
		}
		lastSample, lastCompleted = now, summary.Requests
	}
	publish()

	// Drain every result before returning.
	for results != nil {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
			} else {
				summary.Record(result)
				intervals.record(result, result.CompletedAt.Sub(start))
			}
		case elapsed := <-loadWindowEnded:
			loadElapsed, loadEnded = min(elapsed, cfg.Duration), true
			loadWindowEnded = nil
			status = "draining"
			publish()
		case <-ticker.C:
			publish()
		}
	}

	if !loadEnded {
		loadElapsed = min(<-loadWindowEnded, cfg.Duration)
	}
	summary.LoadWindowSeconds = loadElapsed.Seconds()
	summary.Elapsed = time.Since(start)
	summary.Intervals = intervals.snapshot(summary.Elapsed, true)
	schedulerStats := <-schedulerDone

	summary.ScheduledRequests = schedulerStats.Scheduled
	summary.DispatchedRequests = schedulerStats.Dispatched
	summary.MissedSchedules = schedulerStats.Missed
	summary.StartedRequests = int(requestsStarted.Load())
	summary.Calculate()
	return summary
}

// Pin localhost to loopback instead of trusting DNS resolution; redirects are
// separately disabled on the client. Direct engine callers get this protection too.
func dialLoopback(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("non-loopback target rejected")
	}
	return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, network, net.JoinHostPort(host, port))
}

func scheduleRequests(
	ctx context.Context,
	rps int,
	jobs chan<- struct{},
	missed *atomic.Int64,
) SchedulerStats {
	defer close(jobs)

	stats := SchedulerStats{}

	interval := time.Second / time.Duration(rps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return stats

		case <-ticker.C:
			if ctx.Err() != nil {
				return stats
			}

			stats.Scheduled++

			select {
			case jobs <- struct{}{}:
				stats.Dispatched++

			default:
				stats.Missed++
				missed.Add(1)
			}
		}
	}
}

func worker(
	scheduleCtx context.Context,
	requestCtx context.Context,
	client *http.Client,
	target string,
	jobs <-chan struct{},
	results chan<- Result,
	started *atomic.Int64,
) {
	for {
		select {
		case <-scheduleCtx.Done():
			return

		case _, ok := <-jobs:
			if !ok {
				return
			}
		}

		if scheduleCtx.Err() != nil {
			return
		}
		started.Add(1)

		result := executeRequest(requestCtx, client, target)
		result.CompletedAt = time.Now()

		// Every started request is recorded.
		results <- result
	}
}

func executeRequest(
	ctx context.Context,
	client *http.Client,
	target string,
) Result {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, target, nil,
	)
	if err != nil {
		return Result{Err: err}
	}

	start := time.Now()

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			err = context.Canceled
		}
		return Result{
			Latency: time.Since(start),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	// Include complete body transfer in measured latency.
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil && ctx.Err() != nil {
		err = context.Canceled
	}

	return Result{
		StatusCode: resp.StatusCode,
		Latency:    time.Since(start),
		Err:        err,
	}
}
