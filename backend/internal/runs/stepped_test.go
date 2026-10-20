package runs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asralov/storm/internal/loadtest"
)

func steppedConfig(url string) Config {
	return Config{Mode: "stepped", URL: url, Method: "GET", Workers: 2, Authorized: true, StartRPS: 10, RPSIncrement: 10, MaxRPS: 30, StageDurationSeconds: 1}
}

func TestStageGenerationAndValidation(t *testing.T) {
	cfg := steppedConfig("http://127.0.0.1:8081/fast")
	cfg.MaxRPS = 35
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	var rates []int
	for _, stage := range cfg.stageConfigs() {
		rates = append(rates, stage.RPS)
		if stage.Duration != time.Second {
			t.Fatal("bad stage duration")
		}
	}
	if !reflect.DeepEqual(rates, []int{10, 20, 30}) {
		t.Fatalf("rates: %v", rates)
	}
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"unknown mode", func(c *Config) { c.Mode = "auto" }}, {"zero start", func(c *Config) { c.StartRPS = 0 }},
		{"zero step", func(c *Config) { c.RPSIncrement = 0 }}, {"negative step", func(c *Config) { c.RPSIncrement = -1 }},
		{"reversed range", func(c *Config) { c.MaxRPS = 5 }}, {"high max", func(c *Config) { c.MaxRPS = 101 }},
		{"many stages", func(c *Config) { c.StartRPS = 1; c.RPSIncrement = 1; c.MaxRPS = 20 }},
		{"long total", func(c *Config) { c.MaxRPS = 50; c.StageDurationSeconds = 30 }},
		{"long stage", func(c *Config) { c.StageDurationSeconds = 31 }}, {"zero duration", func(c *Config) { c.StageDurationSeconds = 0 }},
		{"workers", func(c *Config) { c.Workers = 11 }}, {"unauthorized", func(c *Config) { c.Authorized = false }},
		{"external", func(c *Config) { c.URL = "http://example.com/" }}, {"method", func(c *Config) { c.Method = "POST" }},
		{"mixed fields", func(c *Config) { c.RPS = 10 }}, {"stepped fields in standard", func(c *Config) { c.Mode = "standard"; c.RPS = 20; c.DurationSeconds = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := steppedConfig("http://127.0.0.1:8081/fast")
			tc.change(&c)
			if err := c.validate(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	standard := config("http://127.0.0.1:8081/fast")
	if err := standard.validate(); err != nil || standard.Mode != "standard" {
		t.Fatalf("legacy standard rejected: %v", err)
	}
}

func fakeSummary(cfg loadtest.Config) loadtest.Summary {
	s := loadtest.Summary{Elapsed: cfg.Duration, LoadWindowSeconds: cfg.Duration.Seconds()}
	for range cfg.RPS {
		s.Record(loadtest.Result{StatusCode: 200, Latency: time.Duration(cfg.RPS) * time.Millisecond})
	}
	s.StartedRequests, s.DispatchedRequests, s.ScheduledRequests = s.Requests, s.Requests, s.Requests
	s.Calculate()
	return s
}

func TestSteppedSequentialIsolationAndSnapshots(t *testing.T) {
	m := manager(t)
	var inEngine, calls atomic.Int32
	m.engine = func(ctx context.Context, cfg loadtest.Config, updates chan<- loadtest.Progress) loadtest.Summary {
		if inEngine.Add(1) != 1 {
			t.Error("stages overlap")
		}
		defer inEngine.Add(-1)
		calls.Add(1)
		return fakeSummary(cfg)
	}
	r, err := m.Start(steppedConfig("http://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	final := waitRun(t, m, r.ID, terminal)
	if calls.Load() != 3 || inEngine.Load() != 0 || final.Status != "completed" || final.Summary != nil {
		t.Fatalf("bad stepped completion: %+v", final)
	}
	for i, stage := range final.Stages {
		if stage.Number != i+1 || stage.Status != "completed" || stage.Summary == nil || stage.StartedAt == nil || stage.CompletedAt == nil {
			t.Fatalf("bad stage: %+v", stage)
		}
		if stage.Summary.P95LatencyMs != float64(stage.TargetRPS) || stage.Summary.Requests != stage.TargetRPS || *stage.ActualStartRPS != float64(stage.TargetRPS) {
			t.Fatalf("stage samples contaminated: %+v", stage)
		}
		if i > 0 && stage.StartedAt.Before(*final.Stages[i-1].CompletedAt) {
			t.Fatal("overlapping timestamps")
		}
	}
	if final.OverallLoadSeconds != 3 || final.PlannedLoadSeconds != 3 || final.CurrentStage != 3 {
		t.Fatal("incorrect overall progress")
	}
	final.Stages[0].Summary.Requests = -1
	*final.Stages[0].ActualStartRPS = -1
	again, _ := m.Get(r.ID)
	if again.Stages[0].Summary.Requests < 0 || *again.Stages[0].ActualStartRPS < 0 {
		t.Fatal("stage snapshot not isolated")
	}
}

func TestCancelAtStageBoundaryPreventsNextStage(t *testing.T) {
	m := manager(t)
	var calls atomic.Int32
	m.engine = func(ctx context.Context, cfg loadtest.Config, updates chan<- loadtest.Progress) loadtest.Summary {
		calls.Add(1)
		// Stop after stage work finishes but before the orchestrator can admit the next.
		s := fakeSummary(cfg)
		active := m.Active()
		if active == nil {
			t.Error("active slot released between stages")
		} else {
			if _, err := m.Stop(active.ID); err != nil {
				t.Error(err)
			}
		}
		return s
	}
	r, err := m.Start(steppedConfig("http://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	final := waitRun(t, m, r.ID, terminal)
	if calls.Load() != 1 || final.Status != "canceled" || final.Stages[1].Status != "skipped" || final.Stages[1].StartedAt != nil || final.Stages[1].Summary != nil {
		t.Fatalf("future stage launched after Stop: %+v", final)
	}
}

func TestSharedActiveSlotBothModes(t *testing.T) {
	for _, firstStepped := range []bool{false, true} {
		m := manager(t)
		m.engine = func(ctx context.Context, cfg loadtest.Config, updates chan<- loadtest.Progress) loadtest.Summary {
			<-ctx.Done()
			return loadtest.Summary{}
		}
		first, second := config("http://127.0.0.1:1"), steppedConfig("http://127.0.0.1:1")
		if firstStepped {
			first, second = second, first
		}
		r, err := m.Start(first)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = m.Start(second); !errors.Is(err, ErrBusy) {
			t.Fatalf("cross-mode conflict missing: %v", err)
		}
		if _, err = m.Stop(r.ID); err != nil {
			t.Fatal(err)
		}
		waitRun(t, m, r.ID, terminal)
		m.Close()
	}
}

func TestRealSteppedDrainAndAccounting(t *testing.T) {
	var active atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		active.Add(1)
		defer active.Add(-1)
		select {
		case <-r.Context().Done():
			return
		case <-time.After(150 * time.Millisecond):
			w.WriteHeader(200)
		}
	}))
	defer target.Close()
	m := manager(t)
	m.engine = func(ctx context.Context, cfg loadtest.Config, p chan<- loadtest.Progress) loadtest.Summary {
		if active.Load() != 0 {
			t.Error("next stage started while previous target handler active")
		}
		return loadtest.RunWithProgress(ctx, cfg, p)
	}
	cfg := steppedConfig(target.URL)
	cfg.MaxRPS = 20
	r, err := m.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	final := waitRun(t, m, r.ID, terminal)
	if final.Status != "completed" || len(final.Stages) != 2 || active.Load() != 0 {
		t.Fatal("stepped drain incomplete")
	}
	for _, stage := range final.Stages {
		assertAccounting(t, Run{Summary: stage.Summary, Intervals: stage.Intervals, Progress: loadtest.Progress{CompletedRequests: stage.Summary.Requests, StartedRequests: stage.Summary.StartedRequests}})
	}
}

func TestSteppedStopAndShutdownDuringSlowResponse(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		entered := make(chan struct{}, 1)
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-r.Context().Done()
		}))
		m := manager(t)
		cfg := steppedConfig(target.URL)
		cfg.StageDurationSeconds = 5
		r, err := m.Start(cfg)
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-entered:
		case <-time.After(time.Second):
			m.Close()
			target.Close()
			t.Fatal("request not started")
		}
		start := time.Now()
		if shutdown {
			m.Close()
		} else {
			_, err = m.Stop(r.ID)
			if err != nil {
				t.Fatal(err)
			}
		}
		final := waitRun(t, m, r.ID, terminal)
		if time.Since(start) > 1500*time.Millisecond || final.Status != "canceled" || final.Stages[0].Summary == nil || final.Stages[0].Summary.Canceled == 0 || final.Stages[1].Status != "skipped" {
			t.Fatalf("bad cancellation: %+v", final)
		}
		m.Close()
		target.Close()
	}
}
