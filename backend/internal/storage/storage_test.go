package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/asralov/storm/internal/assessment"
	"github.com/asralov/storm/internal/loadtest"
	"github.com/asralov/storm/internal/runs"
)

func fixture(n int, stepped bool) runs.Run {
	start := time.Date(2026, 1, 1, 0, 0, n, 0, time.UTC)
	end := start.Add(2 * time.Second)
	p50, p95, p99, avg := 10.0, 20.0, 30.0, 15.0
	window := loadtest.Interval{Sequence: 1, StartSeconds: 0, ElapsedSeconds: 2, DurationSeconds: 2, Completed: 100, Successes: 100, Throughput: 50, Average: &avg, P50: &p50, P95: &p95, P99: &p99}
	summary := loadtest.Summary{Requests: 100, StartedRequests: 100, Successes: 100, ElapsedSeconds: 2, LoadWindowSeconds: 2, Throughput: 50, AvgLatencyMs: 15, P50LatencyMs: 10, P95LatencyMs: 20, P99LatencyMs: 30}
	r := runs.Run{ID: fmt.Sprintf("run_%032x", n), Config: runs.Config{Mode: "standard", URL: "http://127.0.0.1:8081/variable", Method: "GET", Workers: 4, RPS: 50, DurationSeconds: 2, Authorized: true}, Status: "completed", StartedAt: &start, CompletedAt: &end, Summary: &summary, Intervals: []loadtest.Interval{window}, Persistence: &runs.PersistenceState{Status: "saved"}}
	if stepped {
		r.Config.Mode = "stepped"
		r.Config.StartRPS = 10
		r.Config.RPSIncrement = 10
		r.Config.MaxRPS = 20
		r.Config.StageDurationSeconds = 2
		r.Config.RPS = 0
		r.Config.DurationSeconds = 0
		r.Stages = []runs.Stage{{Number: 1, TargetRPS: 10, Status: "completed", StartedAt: &start, CompletedAt: &end, Summary: &summary, Intervals: []loadtest.Interval{window}}}
		second := summary
		second.P95LatencyMs = 300
		r.Stages = append(r.Stages, runs.Stage{Number: 2, TargetRPS: 20, Status: "completed", StartedAt: &start, CompletedAt: &end, Summary: &second, Intervals: []loadtest.Interval{window}})
		r.Summary = nil
		r.CurrentStage = 2
		r.TotalStages = 2
		max, errRate, min := 200.0, 1.0, 95.0
		r.Config.Thresholds = &assessment.Thresholds{MaxP95LatencyMs: &max, MaxErrorRatePercent: &errRate, MinStartRatePercent: &min}
		a, _ := assessment.Analyze(r.Config.Thresholds, []assessment.StageInput{{Number: 1, TargetRPS: 10, Status: "completed", Summary: &summary}, {Number: 2, TargetRPS: 20, Status: "completed", Summary: &second}}, "completed")
		r.Analysis = &a
	}
	return r
}
func openTest(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "space in directory", "history.sqlite3")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestRoundTripAndReopen(t *testing.T) {
	s, path := openTest(t)
	ctx := context.Background()
	for i, stepped := range []bool{false, true} {
		r := fixture(i+1, stepped)
		if err := s.Save(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i, stepped := range []bool{false, true} {
		want := fixture(i+1, stepped)
		got, err := s.Get(ctx, want.ID)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := json.Marshal(want)
		b, _ := json.Marshal(got)
		if string(a) != string(b) {
			t.Fatalf("report round trip changed data:\n%s\n%s", a, b)
		}
	}
	p, err := s.List(ctx, 20, 0)
	if err != nil || len(p.Items) != 2 || p.Items[0].BreakpointStatus != "bounded" || *p.Items[0].P95LatencyMs != 300 || p.Items[0].P95Label != "Max stage P95" {
		t.Fatalf("overview incorrect: %+v %v", p, err)
	}
}

func TestPaginationDeleteAndValidation(t *testing.T) {
	s, _ := openTest(t)
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		if err := s.Save(ctx, fixture(i, false)); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.List(ctx, 2, 1)
	if err != nil || p.Total != 5 || len(p.Items) != 2 || p.Items[0].ID != fixture(4, false).ID || p.Items[1].ID != fixture(3, false).ID {
		t.Fatalf("bad pagination: %+v %v", p, err)
	}
	if err = s.Delete(ctx, p.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, p.Items[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted record still readable")
	}
	if err = s.Delete(ctx, p.Items[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing delete should return not found")
	}
	if _, err = s.List(ctx, 101, 0); !errors.Is(err, ErrInvalid) {
		t.Fatal("unbounded list allowed")
	}
	if _, err = s.List(ctx, 20, -1); !errors.Is(err, ErrInvalid) {
		t.Fatal("negative offset allowed")
	}
	if _, err = s.Get(ctx, "bad' OR 1=1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid id allowed")
	}
}

func TestLifecycleAndCrashRecovery(t *testing.T) {
	s, path := openTest(t)
	ctx := context.Background()
	r := fixture(1, true)
	r.Status = "running"
	r.CompletedAt = nil
	if err := s.Save(ctx, r); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.List(ctx, 20, 0); p.Total != 0 {
		t.Fatal("unfinished marker listed as report")
	}
	if _, err := s.Get(ctx, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("unfinished report exposed")
	}
	s.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.RecoverInterrupted(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, r.ID)
	if err != nil || got.Status != "interrupted" || got.Summary != nil || got.Analysis != nil || got.CompletedAt != nil || got.Stages[0].Summary != nil || got.Stages[0].StartedAt != nil {
		t.Fatalf("invented recovered results: %+v %v", got, err)
	}
	if err = s.RecoverInterrupted(ctx); err != nil {
		t.Fatal(err)
	}
	for i, status := range []string{"canceled", "failed"} {
		f := fixture(i+2, false)
		f.Status = status
		if err = s.Save(ctx, f); err != nil {
			t.Fatal(err)
		}
		got, err = s.Get(ctx, f.ID)
		if err != nil || got.Status != status {
			t.Fatal("terminal status lost")
		}
	}
	// Late start-marker writes cannot overwrite a finalized run.
	f := fixture(2, false)
	f.Status = "pending"
	if err = s.Save(ctx, f); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(ctx, f.ID)
	if got.Status != "canceled" {
		t.Fatal("final report overwritten by marker")
	}
}

func TestConcurrentAccessAndStorageFailure(t *testing.T) {
	s, _ := openTest(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 1; i <= 12; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r := fixture(n, n%2 == 0)
			if err := s.Save(ctx, r); err != nil {
				t.Error(err)
				return
			}
			if _, err := s.Get(ctx, r.ID); err != nil {
				t.Error(err)
			}
			if _, err := s.List(ctx, 5, 0); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	p, err := s.List(ctx, 20, 0)
	if err != nil || p.Total != 12 {
		t.Fatal("concurrent writes lost data")
	}
	if _, err = s.db.Exec("PRAGMA query_only=ON"); err != nil {
		t.Fatal(err)
	}
	if err = s.Save(ctx, fixture(99, false)); err == nil {
		t.Fatal("read-only failure not returned")
	}
	if p, _ = s.List(ctx, 20, 0); p.Total != 12 {
		t.Fatal("failed write corrupted records")
	}
	s.Close()
	if _, err = s.Get(ctx, fixture(1, false).ID); err == nil {
		t.Fatal("closed DB error hidden")
	}
}

func TestVersioningAndMigrationRollback(t *testing.T) {
	s, path := openTest(t)
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatalf("schema not initialized: %d %v", version, err)
	}
	if _, err := s.db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if other, err := Open(path); err == nil {
		other.Close()
		t.Fatal("future schema silently accepted")
	}
	broken := filepath.Join(t.TempDir(), "broken.sqlite3")
	db, err := sql.Open("sqlite", broken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE TABLE history_runs (x INTEGER)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if other, err := Open(broken); err == nil {
		other.Close()
		t.Fatal("invalid legacy schema accepted")
	}
	db, _ = sql.Open("sqlite", broken)
	defer db.Close()
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 0 {
		t.Fatal("failed migration was not rolled back")
	}
	s2, _ := openTest(t)
	r := fixture(1, false)
	if err = s2.Save(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if _, err = s2.db.Exec("UPDATE history_runs SET snapshot_version=99 WHERE id=?", r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s2.Get(context.Background(), r.ID); err == nil {
		t.Fatal("unknown snapshot format accepted")
	}
}

func TestDataDirectorySelection(t *testing.T) {
	home := t.TempDir()
	local := filepath.Join(home, "Local")
	xdg := filepath.Join(home, "data")
	for _, tc := range []struct{ platform, local, xdg, want string }{
		{"windows", local, "", filepath.Join(local, "Storm", "history.sqlite3")},
		{"darwin", "", "", filepath.Join(home, "Library", "Application Support", "Storm", "history.sqlite3")},
		{"linux", "", xdg, filepath.Join(xdg, "storm", "history.sqlite3")},
		{"linux", "", "relative", filepath.Join(home, ".local", "share", "storm", "history.sqlite3")},
	} {
		got, err := dataPath(tc.platform, home, tc.local, tc.xdg)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("path: %s %v", got, err)
		}
	}
	if _, err := dataPath("windows", home, "", ""); err == nil {
		t.Fatal("missing user directory should fail explicitly")
	}
	if _, err := Open("history.sqlite3"); err == nil {
		t.Fatal("repository-relative database accepted")
	}
	block := filepath.Join(home, "file")
	if err := os.WriteFile(block, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(block, "history.sqlite3")); err == nil {
		t.Fatal("invalid database directory accepted")
	}
}
