package loadtest

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type Config struct {
	URL      string
	Workers  int
	Duration time.Duration
}

type Result struct {
	StatusCode int
	Latency    time.Duration
	Err        error
}

const shutdownGracePeriod = 3 * time.Second

func Run(parent context.Context, cfg Config) Summary {
	start := time.Now()

	// Controls when NEW requests may begin.
	scheduleCtx, stopScheduling := context.WithTimeout(
		parent, cfg.Duration,
	)
	defer stopScheduling()

	// Controls requests that are already in progress.
	requestCtx, cancelRequests := context.WithCancel(parent)
	defer cancelRequests()

	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 2 * time.Second,
		}).DialContext,
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

	results := make(chan Result, cfg.Workers)
	var wg sync.WaitGroup

	// Launch workers.
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			worker(
				scheduleCtx,
				requestCtx,
				client,
				cfg.URL,
				results,
			)
		}()
	}

	workersDone := make(chan struct{})

	go func() {
		wg.Wait()
		close(workersDone)
	}()

	// Coordinate shutdown independently from metric collection.
	go func() {
		// Wait until the load generation window closes.
		<-scheduleCtx.Done()

		// Allow outstanding requests to finish.
		select {
		case <-workersDone:
			// All workers finished naturally.

		case <-time.After(shutdownGracePeriod):
			// Grace period expired.
			cancelRequests()
		}

		// Always wait before closing results.
		<-workersDone
		close(results)
	}()

	var summary Summary

	// Keep collecting until every worker has finished.
	for result := range results {
		summary.Record(result)
	}

	summary.Elapsed = time.Since(start)
	summary.Calculate()

	return summary
}

func worker(
	scheduleCtx context.Context,
	requestCtx context.Context,
	client *http.Client,
	target string,
	results chan<- Result,
) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-scheduleCtx.Done():
			return

		case <-ticker.C:
		}

		// Do not schedule requests after the load window ends.
		if scheduleCtx.Err() != nil {
			return
		}

		result := executeRequest(requestCtx, client, target)

		// Always account for an attempted request.
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
		return Result{
			Latency: time.Since(start),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	// Include complete body transfer in measured latency.
	_, err = io.Copy(io.Discard, resp.Body)

	return Result{
		StatusCode: resp.StatusCode,
		Latency:    time.Since(start),
		Err:        err,
	}
}
