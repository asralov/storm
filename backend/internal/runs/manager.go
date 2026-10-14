package runs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/yourusername/storm/internal/loadtest"
	"sync"
	"time"
)

var (
	ErrBusy     = errors.New("a test is already active")
	ErrNotFound = errors.New("test not found or expired")
	ErrFinished = errors.New("test has already finished")
	ErrClosed   = errors.New("test manager is shutting down")
)

const retainedRuns = 20

type Run struct {
	ID                    string            `json:"id"`
	Config                Config            `json:"config"`
	Status                string            `json:"status"`
	StartedAt             *time.Time        `json:"started_at"`
	CompletedAt           *time.Time        `json:"completed_at"`
	Progress              loadtest.Progress `json:"progress"`
	Summary               *loadtest.Summary `json:"summary,omitempty"`
	CancellationRequested bool              `json:"cancellation_requested"`
	Error                 string            `json:"error,omitempty"`
}

type entry struct {
	run    Run
	cancel context.CancelFunc
}

// Manager owns run lifetimes independently of the HTTP request that starts them.
// All published snapshots are copied while holding mu; no caller gets mutable state.
type Manager struct {
	mu       sync.Mutex
	runs     map[string]*entry
	finished []string
	active   string
	closed   bool
	wg       sync.WaitGroup
}

func NewManager() *Manager { return &Manager{runs: make(map[string]*entry)} }

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
	e := &entry{run: Run{ID: "run_" + hex.EncodeToString(id[:]), Config: cfg, Status: "pending", Progress: loadtest.Progress{Status: "pending", DurationSeconds: float64(cfg.DurationSeconds), TargetRPS: cfg.RPS}}, cancel: cancel}
	m.runs[e.run.ID], m.active = e, e.run.ID
	m.wg.Add(1)
	go m.execute(ctx, e)
	return clone(e.run), nil
}

func (m *Manager) execute(ctx context.Context, e *entry) {
	defer m.wg.Done()
	defer e.cancel()
	m.mu.Lock()
	now := time.Now()
	e.run.StartedAt, e.run.Status = &now, "running"
	m.mu.Unlock()
	updates := make(chan loadtest.Progress, 1)
	final := make(chan loadtest.Summary, 1)
	go func() { final <- loadtest.RunWithProgress(ctx, e.run.Config.engineConfig(), updates) }()
	for {
		select {
		case p := <-updates:
			m.mu.Lock()
			e.run.Progress, e.run.Status = p, p.Status
			m.mu.Unlock()
		case summary := <-final:
			m.mu.Lock()
			status := "completed"
			if e.run.CancellationRequested {
				status = "canceled"
			}
			if summary.StartedRequests != summary.Requests || summary.Requests != summary.Successes+summary.Failures {
				status, e.run.Error = "failed", "engine request accounting mismatch"
			}
			end := time.Now()
			e.run.Status, e.run.CompletedAt, e.run.Summary = status, &end, &summary
			p := e.run.Progress
			p.Status, p.SampledAt = status, end
			p.ElapsedSeconds, p.LoadElapsedSeconds = summary.ElapsedSeconds, summary.LoadWindowSeconds
			p.StartedRequests, p.CompletedRequests = summary.StartedRequests, summary.Requests
			p.Successes, p.Failures, p.MissedSchedules = summary.Successes, summary.Failures, summary.MissedSchedules
			if p.LoadElapsedSeconds > 0 {
				p.ObservedStartRPS = float64(p.StartedRequests) / p.LoadElapsedSeconds
			}
			// Recent rate is only meaningful for live snapshots; the final report has throughput.
			p.RecentThroughput, p.SampleIntervalSeconds = 0, 0
			e.run.Progress = p
			m.active = ""
			m.finished = append(m.finished, e.run.ID)
			if len(m.finished) > retainedRuns {
				delete(m.runs, m.finished[0])
				m.finished = m.finished[1:]
			}
			m.mu.Unlock()
			return
		}
	}
}

func clone(r Run) Run {
	if r.StartedAt != nil {
		v := *r.StartedAt
		r.StartedAt = &v
	}
	if r.CompletedAt != nil {
		v := *r.CompletedAt
		r.CompletedAt = &v
	}
	if r.Summary != nil {
		v := r.Summary.Clone()
		r.Summary = &v
	}
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

// Close rejects new work, cancels active traffic, and joins execution goroutines.
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
