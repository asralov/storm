package runs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/yourusername/storm/internal/loadtest"
)

var (
	ErrBusy     = errors.New("a test is already active")
	ErrNotFound = errors.New("test not found or expired")
	ErrFinished = errors.New("test has already finished")
	ErrClosed   = errors.New("test manager is shutting down")
)

const retainedRuns = 20

type Stage struct {
	Number         int                 `json:"number"`
	TargetRPS      int                 `json:"target_rps"`
	Status         string              `json:"status"`
	StartedAt      *time.Time          `json:"started_at"`
	CompletedAt    *time.Time          `json:"completed_at"`
	ActualStartRPS *float64            `json:"actual_start_rps"`
	Summary        *loadtest.Summary   `json:"summary,omitempty"`
	Intervals      []loadtest.Interval `json:"intervals"`
}

type Run struct {
	Intervals             []loadtest.Interval `json:"intervals"`
	ID                    string              `json:"id"`
	Config                Config              `json:"config"`
	Status                string              `json:"status"`
	StartedAt             *time.Time          `json:"started_at"`
	CompletedAt           *time.Time          `json:"completed_at"`
	Progress              loadtest.Progress   `json:"progress"`
	Summary               *loadtest.Summary   `json:"summary,omitempty"`
	CancellationRequested bool                `json:"cancellation_requested"`
	Error                 string              `json:"error,omitempty"`
	Stages                []Stage             `json:"stages,omitempty"`
	CurrentStage          int                 `json:"current_stage,omitempty"`
	TotalStages           int                 `json:"total_stages,omitempty"`
	OverallElapsedSeconds float64             `json:"overall_elapsed_seconds"`
	OverallLoadSeconds    float64             `json:"overall_load_seconds"`
	PlannedLoadSeconds    float64             `json:"planned_load_seconds"`
}

type entry struct {
	run    Run
	cancel context.CancelFunc
}
type engineFunc func(context.Context, loadtest.Config, chan<- loadtest.Progress) loadtest.Summary

// The single active slot covers the whole job, including stage drains and gaps.
// Engine calls return only after worker cleanup. Snapshots are copied under mu.
type Manager struct {
	mu       sync.Mutex
	runs     map[string]*entry
	finished []string
	active   string
	closed   bool
	wg       sync.WaitGroup
	engine   engineFunc
}

func NewManager() *Manager {
	return &Manager{runs: make(map[string]*entry), engine: loadtest.RunWithProgress}
}

func (m *Manager) Start(cfg Config) (Run, error) {
	if err := cfg.validate(); err != nil {
		return Run{}, err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Run{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Run{}, ErrClosed
	}
	if m.active != "" {
		return Run{}, ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	plan := cfg.stageConfigs()
	e := &entry{run: Run{ID: "run_" + hex.EncodeToString(id[:]), Config: cfg, Status: "pending", Progress: loadtest.Progress{Status: "pending", DurationSeconds: plan[0].Duration.Seconds(), TargetRPS: plan[0].RPS}}, cancel: cancel}
	for i, stage := range plan {
		e.run.PlannedLoadSeconds += stage.Duration.Seconds()
		if cfg.Mode == "stepped" {
			e.run.Stages = append(e.run.Stages, Stage{Number: i + 1, TargetRPS: stage.RPS, Status: "pending"})
		}
	}
	if cfg.Mode == "stepped" {
		e.run.TotalStages = len(plan)
	}
	m.runs[e.run.ID], m.active = e, e.run.ID
	m.wg.Add(1)
	go m.execute(ctx, e, plan)
	return clone(e.run), nil
}

func (m *Manager) execute(ctx context.Context, e *entry, plan []loadtest.Config) {
	defer m.wg.Done()
	defer e.cancel()
	m.mu.Lock()
	start := time.Now()
	e.run.StartedAt = &start
	m.mu.Unlock()
	loadDone := 0.0
	for index, cfg := range plan {
		// Stop and stage admission share mu. Once Stop has been acknowledged no
		// future stage is admitted; a concurrently admitted stage sees canceled ctx.
		m.mu.Lock()
		if ctx.Err() != nil || e.run.CancellationRequested {
			m.mu.Unlock()
			break
		}
		now := time.Now()
		e.run.Status = "running"
		e.run.Progress = loadtest.Progress{Status: "running", DurationSeconds: cfg.Duration.Seconds(), TargetRPS: cfg.RPS}
		e.run.Intervals = nil
		e.run.OverallElapsedSeconds = now.Sub(start).Seconds()
		if e.run.Config.Mode == "stepped" {
			e.run.CurrentStage = index + 1
			e.run.Stages[index].Status = "running"
			e.run.Stages[index].StartedAt = &now
		}
		m.mu.Unlock()
		summary := m.executeStage(ctx, e, cfg, index, start, loadDone)
		m.mu.Lock()
		status := "completed"
		if e.run.CancellationRequested {
			status = "canceled"
		}
		if summary.StartedRequests != summary.Requests || summary.Requests != summary.Successes+summary.Failures {
			status, e.run.Error = "failed", "engine request accounting mismatch"
		}
		end := time.Now()
		loadDone += summary.LoadWindowSeconds
		e.run.OverallLoadSeconds = loadDone
		e.run.OverallElapsedSeconds = end.Sub(start).Seconds()
		e.run.Intervals = summary.Intervals
		e.run.Progress = finalProgress(e.run.Progress, summary, status, end)
		if e.run.Config.Mode == "stepped" {
			stage := &e.run.Stages[index]
			stage.Status, stage.CompletedAt, stage.Summary, stage.Intervals = status, &end, &summary, summary.Intervals
			if summary.LoadWindowSeconds > 0 {
				rate := float64(summary.StartedRequests) / summary.LoadWindowSeconds
				stage.ActualStartRPS = &rate
			}
		} else {
			e.run.Summary = &summary
		}
		if status == "completed" && index < len(plan)-1 {
			e.run.Status = "running"
		}
		m.mu.Unlock()
		if status != "completed" {
			break
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	status := "completed"
	if e.run.CancellationRequested {
		status = "canceled"
	}
	if e.run.Error != "" {
		status = "failed"
	}
	end := time.Now()
	e.run.Status, e.run.CompletedAt = status, &end
	e.run.OverallElapsedSeconds = end.Sub(start).Seconds()
	// A standard run canceled before admission still exposes an empty report.
	if e.run.Config.Mode != "stepped" && e.run.Summary == nil {
		e.run.Summary = &loadtest.Summary{}
	}
	for i := range e.run.Stages {
		if e.run.Stages[i].Status == "pending" {
			e.run.Stages[i].Status = "skipped"
		}
	}
	e.run.Progress.Status = status
	m.active = ""
	m.finished = append(m.finished, e.run.ID)
	if len(m.finished) > retainedRuns {
		delete(m.runs, m.finished[0])
		m.finished = m.finished[1:]
	}
}

// Both standard tests and individual stages use this same execution adapter.
func (m *Manager) executeStage(ctx context.Context, e *entry, cfg loadtest.Config, index int, start time.Time, loadDone float64) loadtest.Summary {
	updates := make(chan loadtest.Progress, 1)
	final := make(chan loadtest.Summary, 1)
	done := make(chan struct{})
	go func() { defer close(done); final <- m.engine(ctx, cfg, updates) }()
	for {
		select {
		case p := <-updates:
			m.mu.Lock()
			e.run.Intervals, e.run.Progress, e.run.Status = p.Intervals, p, p.Status
			e.run.OverallElapsedSeconds = time.Since(start).Seconds()
			e.run.OverallLoadSeconds = loadDone + p.LoadElapsedSeconds
			if e.run.Config.Mode == "stepped" {
				e.run.Stages[index].Status = p.Status
				e.run.Stages[index].Intervals = p.Intervals
			}
			m.mu.Unlock()
		case summary := <-final:
			<-done
			return summary
		}
	}
}

func finalProgress(p loadtest.Progress, s loadtest.Summary, status string, now time.Time) loadtest.Progress {
	p.Status, p.SampledAt = status, now
	p.ElapsedSeconds, p.LoadElapsedSeconds = s.ElapsedSeconds, s.LoadWindowSeconds
	p.StartedRequests, p.CompletedRequests = s.StartedRequests, s.Requests
	p.Successes, p.Failures, p.MissedSchedules = s.Successes, s.Failures, s.MissedSchedules
	p.AvgLatencyMs = s.AverageLatency()
	if s.LoadWindowSeconds > 0 {
		p.ObservedStartRPS = float64(s.StartedRequests) / s.LoadWindowSeconds
	}
	p.RecentThroughput, p.SampleIntervalSeconds = 0, 0
	p.Intervals = nil
	return p
}

func copyTime(p *time.Time) *time.Time {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func copyNumber(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func clone(r Run) Run {
	r.Intervals = loadtest.CloneIntervals(r.Intervals)
	r.Progress.Intervals = nil
	r.Progress.AvgLatencyMs = copyNumber(r.Progress.AvgLatencyMs)
	r.StartedAt, r.CompletedAt = copyTime(r.StartedAt), copyTime(r.CompletedAt)
	if r.Summary != nil {
		v := r.Summary.Clone()
		r.Summary = &v
	}
	stages := make([]Stage, len(r.Stages))
	for i, stage := range r.Stages {
		stage.StartedAt, stage.CompletedAt = copyTime(stage.StartedAt), copyTime(stage.CompletedAt)
		stage.ActualStartRPS = copyNumber(stage.ActualStartRPS)
		stage.Intervals = loadtest.CloneIntervals(stage.Intervals)
		if stage.Summary != nil {
			v := stage.Summary.Clone()
			stage.Summary = &v
		}
		stages[i] = stage
	}
	r.Stages = stages
	return r
}

func (m *Manager) Get(id string) (Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.runs[id]
	if !ok {
		return Run{}, ErrNotFound
	}
	return clone(e.run), nil
}
func (m *Manager) Active() *Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == "" {
		return nil
	}
	r := clone(m.runs[m.active].run)
	return &r
}
func (m *Manager) Stop(id string) (Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.runs[id]
	if !ok {
		return Run{}, ErrNotFound
	}
	if e.run.CompletedAt != nil {
		return clone(e.run), ErrFinished
	}
	e.run.CancellationRequested = true
	e.cancel()
	return clone(e.run), nil
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	if m.active != "" {
		e := m.runs[m.active]
		e.run.CancellationRequested = true
		e.cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
}
