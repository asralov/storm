package runs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/asralov/storm/internal/loadtest"
)

type saveFunc func(context.Context, Run) error

func (f saveFunc) Save(ctx context.Context, r Run) error { return f(ctx, r) }

func TestPersistenceFailureKeepsFinalMetrics(t *testing.T) {
	m := NewManager(saveFunc(func(context.Context, Run) error { return errors.New("disk full") }))
	defer m.Close()
	m.engine = func(ctx context.Context, cfg loadtest.Config, p chan<- loadtest.Progress) loadtest.Summary {
		return fakeSummary(cfg)
	}
	r, err := m.Start(config("http://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	final := waitRun(t, m, r.ID, func(r Run) bool { return r.Persistence != nil && r.Persistence.Status == "failed" })
	if final.Status != "completed" || final.Summary.Requests != 20 || !strings.Contains(final.Persistence.Error, "disk full") {
		t.Fatalf("lost in-memory report: %+v", final)
	}
}

func TestPersistenceDoesNotHoldManagerLockAndShutdownJoins(t *testing.T) {
	started := make(chan Run, 1)
	saving := make(chan Run, 1)
	release := make(chan struct{})
	m := NewManager(saveFunc(func(ctx context.Context, r Run) error {
		if r.CompletedAt == nil {
			started <- r
			return nil
		}
		saving <- r
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}))
	defer m.Close()
	m.engine = func(ctx context.Context, cfg loadtest.Config, p chan<- loadtest.Progress) loadtest.Summary {
		select {
		case <-started:
		default:
			t.Error("traffic started before marker write")
		}
		return fakeSummary(cfg)
	}
	r, err := m.Start(config("http://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case snapshot := <-saving:
		if snapshot.Status != "completed" || snapshot.Persistence.Status != "saved" {
			t.Fatal("bad durable snapshot")
		}
	case <-time.After(time.Second):
		t.Fatal("save not reached")
	}
	read := make(chan Run, 1)
	go func() { got, _ := m.Get(r.ID); read <- got }()
	select {
	case got := <-read:
		if got.Persistence.Status != "saving" {
			t.Fatal("missing saving state")
		}
	case <-time.After(time.Second):
		t.Fatal("storage held manager lock")
	}
	if _, err = m.Stop(r.ID); !errors.Is(err, ErrFinished) {
		t.Fatal("late stop should not mutate saved result")
	}
	done := make(chan struct{})
	go func() { m.Close(); close(done) }()
	select {
	case <-done:
		t.Fatal("shutdown did not wait for final write")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish")
	}
	final, _ := m.Get(r.ID)
	if final.Persistence.Status != "saved" || final.Status != "completed" {
		t.Fatal("shutdown changed finalized report")
	}
}
