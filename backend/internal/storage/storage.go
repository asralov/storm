// Package storage persists versioned report snapshots, never per-request data.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/asralov/storm/internal/loadtest"
	"github.com/asralov/storm/internal/runs"
	_ "modernc.org/sqlite"
)

const SchemaVersion = 1
const SnapshotVersion = 1

var ErrNotFound = errors.New("saved run not found")
var ErrInvalid = errors.New("invalid history ID or pagination")
var idPattern = regexp.MustCompile(`^run_[0-9a-f]{32}$`)

func ValidID(id string) bool { return idPattern.MatchString(id) }

type Store struct{ db *sql.DB }
type envelope struct {
	Version int      `json:"version"`
	Run     runs.Run `json:"run"`
}
type Item struct {
	ID               string     `json:"id"`
	Mode             string     `json:"mode"`
	URL              string     `json:"url"`
	Method           string     `json:"method"`
	Workers          int        `json:"workers"`
	ConfiguredLoad   string     `json:"configured_load"`
	StartedAt        *time.Time `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	Status           string     `json:"status"`
	DurationSeconds  *float64   `json:"duration_seconds"`
	TotalRequests    *int       `json:"total_requests"`
	AchievedRate     *float64   `json:"achieved_rate"`
	P95LatencyMs     *float64   `json:"p95_latency_ms"`
	P95Label         string     `json:"p95_label"`
	ErrorRatePercent *float64   `json:"error_rate_percent"`
	BreakpointStatus string     `json:"breakpoint_status,omitempty"`
}
type Page struct {
	Items  []Item `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func Open(path string) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("history database path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db}
	if err = s.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	if runtime.GOOS != "windows" {
		if err = os.Chmod(path, 0600); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}
func (s *Store) initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout=3000"); err != nil {
		return err
	}
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > SchemaVersion {
		return fmt.Errorf("history schema %d is newer than supported schema %d", version, SchemaVersion)
	}
	if version == 0 {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		_, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS history_runs (
		id TEXT PRIMARY KEY, created_at INTEGER NOT NULL, status TEXT NOT NULL,
		mode TEXT NOT NULL, target_url TEXT NOT NULL, finalized INTEGER NOT NULL CHECK(finalized IN (0,1)),
		snapshot_version INTEGER NOT NULL, overview_json TEXT NOT NULL, payload_json TEXT NOT NULL);
		CREATE INDEX IF NOT EXISTS history_newest ON history_runs(finalized,created_at DESC,id DESC);
		PRAGMA user_version=1;`)
		if err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, "SELECT id,created_at,status,mode,target_url,finalized,snapshot_version,overview_json,payload_json FROM history_runs LIMIT 0"); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL")
	return err
}
func (s *Store) Close() error { return s.db.Close() }

func terminal(status string) bool {
	return status == "completed" || status == "canceled" || status == "failed" || status == "interrupted"
}

// Explicitly select current supported configuration fields. Future headers,
// credentials, and bodies must get a deliberate storage/redaction policy first.
func safeRun(r runs.Run) runs.Run {
	c := r.Config
	r.Config = runs.Config{Mode: c.Mode, URL: c.URL, Method: c.Method, Workers: c.Workers, DurationSeconds: c.DurationSeconds, Authorized: c.Authorized, RPS: c.RPS, StartRPS: c.StartRPS, RPSIncrement: c.RPSIncrement, MaxRPS: c.MaxRPS, StageDurationSeconds: c.StageDurationSeconds, Thresholds: c.Thresholds.Clone()}
	return r
}

func (s *Store) Save(ctx context.Context, r runs.Run) error {
	if !ValidID(r.ID) {
		return ErrInvalid
	}
	if !terminal(r.Status) && r.Status != "pending" && r.Status != "running" && r.Status != "draining" {
		return errors.New("invalid stored run status")
	}
	r = safeRun(r)
	payload, err := json.Marshal(envelope{SnapshotVersion, r})
	if err != nil {
		return err
	}
	if len(payload) > 8<<20 {
		return errors.New("report exceeds the 8 MiB snapshot limit")
	}
	overview, err := json.Marshal(summarize(r))
	if err != nil {
		return err
	}
	created := time.Now().UnixNano()
	if r.StartedAt != nil {
		created = r.StartedAt.UnixNano()
	}
	finalized := 0
	if terminal(r.Status) {
		finalized = 1
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO history_runs(id,created_at,status,mode,target_url,finalized,snapshot_version,overview_json,payload_json)
	VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,mode=excluded.mode,target_url=excluded.target_url,
	finalized=excluded.finalized,snapshot_version=excluded.snapshot_version,overview_json=excluded.overview_json,payload_json=excluded.payload_json
	WHERE history_runs.finalized=0 OR excluded.finalized=1`, r.ID, created, r.Status, r.Config.Mode, r.Config.URL, finalized, SnapshotVersion, string(overview), string(payload))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func decode(payload string, version int) (runs.Run, error) {
	if version != SnapshotVersion {
		return runs.Run{}, fmt.Errorf("unsupported report version %d", version)
	}
	var e envelope
	if err := json.Unmarshal([]byte(payload), &e); err != nil {
		return runs.Run{}, fmt.Errorf("invalid saved report: %w", err)
	}
	if e.Version != SnapshotVersion {
		return runs.Run{}, errors.New("saved report envelope version mismatch")
	}
	return e.Run, nil
}
func (s *Store) Get(ctx context.Context, id string) (runs.Run, error) {
	if !ValidID(id) {
		return runs.Run{}, ErrInvalid
	}
	var payload string
	var version int
	err := s.db.QueryRowContext(ctx, "SELECT payload_json,snapshot_version FROM history_runs WHERE id=? AND finalized=1", id).Scan(&payload, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return runs.Run{}, ErrNotFound
	}
	if err != nil {
		return runs.Run{}, err
	}
	return decode(payload, version)
}
func (s *Store) List(ctx context.Context, limit, offset int) (Page, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return Page{}, ErrInvalid
	}
	p := Page{Items: []Item{}, Limit: limit, Offset: offset}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM history_runs WHERE finalized=1").Scan(&p.Total); err != nil {
		return p, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT overview_json,snapshot_version FROM history_runs WHERE finalized=1 ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var data string
		var version int
		if err = rows.Scan(&data, &version); err != nil {
			rows.Close()
			return p, err
		}
		if version != SnapshotVersion {
			rows.Close()
			return p, fmt.Errorf("unsupported report version %d", version)
		}
		var item Item
		if err = json.Unmarshal([]byte(data), &item); err != nil {
			rows.Close()
			return p, err
		}
		p.Items = append(p.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, err
	}
	return p, tx.Commit()
}
func (s *Store) Delete(ctx context.Context, id string) error {
	if !ValidID(id) {
		return ErrInvalid
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM history_runs WHERE id=? AND finalized=1", id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Call only after acquiring exclusive ownership of this database (the app's OS
// file lock), so another process cannot misclassify a live run. Never resumes load.
func (s *Store) RecoverInterrupted(ctx context.Context) error {
	for {
		rows, err := s.db.QueryContext(ctx, "SELECT payload_json,snapshot_version FROM history_runs WHERE finalized=0 LIMIT 100")
		if err != nil {
			return err
		}
		batch := make([]runs.Run, 0, 100)
		for rows.Next() {
			var payload string
			var version int
			if err = rows.Scan(&payload, &version); err != nil {
				rows.Close()
				return err
			}
			r, e := decode(payload, version)
			if e != nil {
				rows.Close()
				return e
			}
			batch = append(batch, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		for _, r := range batch {
			r.Status = "interrupted"
			r.CompletedAt = nil
			r.Summary = nil
			r.Analysis = nil
			r.Intervals = nil
			r.Error = "Storm stopped before a final report was saved. Execution details are unavailable; the test was not resumed."
			r.Persistence = &runs.PersistenceState{Status: "saved"}
			r.Progress = loadtest.Progress{Status: "interrupted", DurationSeconds: r.Progress.DurationSeconds, TargetRPS: r.Progress.TargetRPS}
			r.CurrentStage, r.OverallElapsedSeconds, r.OverallLoadSeconds = 0, 0, 0
			for i := range r.Stages {
				r.Stages[i].Status = "unknown"
				r.Stages[i].Summary = nil
				r.Stages[i].Intervals = nil
				r.Stages[i].StartedAt = nil
				r.Stages[i].CompletedAt = nil
				r.Stages[i].ActualStartRPS = nil
			}
			if err = s.Save(ctx, r); err != nil {
				return err
			}
		}
	}
}
