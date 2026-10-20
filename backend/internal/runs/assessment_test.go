package runs

import (
	"context"
	"testing"

	"github.com/asralov/storm/internal/assessment"
	"github.com/asralov/storm/internal/loadtest"
)

func TestAnalysisOptInAndSnapshotIsolation(t *testing.T) {
	m := manager(t)
	m.engine = func(ctx context.Context, cfg loadtest.Config, updates chan<- loadtest.Progress) loadtest.Summary {
		return fakeSummary(cfg)
	}
	cfg := steppedConfig("http://127.0.0.1:1")
	cfg.StartRPS = 100
	cfg.MaxRPS = 100
	p95, errors, starts := 200.0, 1.0, 95.0
	cfg.Thresholds = &assessment.Thresholds{MaxP95LatencyMs: &p95, MaxErrorRatePercent: &errors, MinStartRatePercent: &starts}
	r, err := m.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	p95 = 0 // Caller changes its own configuration after Start.
	final := waitRun(t, m, r.ID, terminal)
	if final.Analysis == nil || final.Analysis.Stages[0].Status != "PASS" || *final.Analysis.Thresholds.MaxP95LatencyMs != 200 {
		t.Fatalf("bad assessment: %+v", final.Analysis)
	}
	*final.Config.Thresholds.MaxP95LatencyMs = 999
	*final.Analysis.Thresholds.MaxP95LatencyMs = 999
	final.Analysis.Stages[0].Reasons[0] = "changed"
	again, _ := m.Get(r.ID)
	if *again.Config.Thresholds.MaxP95LatencyMs != 200 || *again.Analysis.Thresholds.MaxP95LatencyMs != 200 || again.Analysis.Stages[0].Reasons[0] == "changed" {
		t.Fatal("analysis snapshot leaked mutable state")
	}
	cfg.Thresholds = nil
	r, err = m.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if waitRun(t, m, r.ID, terminal).Analysis != nil {
		t.Fatal("analysis unexpectedly enabled")
	}
}

func TestCanceledJobAnalysisIsPartial(t *testing.T) {
	m := manager(t)
	m.engine = func(ctx context.Context, cfg loadtest.Config, updates chan<- loadtest.Progress) loadtest.Summary {
		<-ctx.Done()
		return loadtest.Summary{}
	}
	cfg := steppedConfig("http://127.0.0.1:1")
	p95, errors, starts := 200.0, 1.0, 95.0
	cfg.Thresholds = &assessment.Thresholds{MaxP95LatencyMs: &p95, MaxErrorRatePercent: &errors, MinStartRatePercent: &starts}
	r, err := m.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Stop(r.ID); err != nil {
		t.Fatal(err)
	}
	final := waitRun(t, m, r.ID, terminal)
	if final.Analysis == nil || final.Analysis.Breakpoint.Status != "partial" || final.Analysis.Breakpoint.Bounded || final.Analysis.Breakpoint.Range != nil {
		t.Fatalf("canceled job got a definitive conclusion: %+v", final.Analysis)
	}
}
