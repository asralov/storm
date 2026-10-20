package storage

import (
	"fmt"
	"github.com/asralov/storm/internal/loadtest"
	"github.com/asralov/storm/internal/runs"
)

func summarize(r runs.Run) Item {
	c := r.Config
	item := Item{ID: r.ID, Mode: c.Mode, URL: c.URL, Method: c.Method, Workers: c.Workers, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt, Status: r.Status, P95Label: "P95"}
	if item.Mode == "" {
		item.Mode = "standard"
	}
	var summaries []*loadtest.Summary
	if c.Mode == "stepped" {
		item.ConfiguredLoad = fmt.Sprintf("%d–%d req/s (+%d), %d stages × %ds", c.StartRPS, c.MaxRPS, c.RPSIncrement, len(r.Stages), c.StageDurationSeconds)
		item.P95Label = "Max stage P95"
		for _, stage := range r.Stages {
			if stage.Summary != nil {
				summaries = append(summaries, stage.Summary)
			}
		}
	} else {
		item.ConfiguredLoad = fmt.Sprintf("%d req/s × %ds", c.RPS, c.DurationSeconds)
		if r.Summary != nil {
			summaries = append(summaries, r.Summary)
		}
	}
	if r.Status == "interrupted" {
		return item
	}
	if len(summaries) > 0 {
		total, eligible, errors := 0, 0, 0
		duration := 0.0
		for _, s := range summaries {
			total += s.Requests
			duration += s.ElapsedSeconds
			eligible += s.Successes + s.HTTPErrorCount + s.NetworkErrors
			errors += s.HTTPErrorCount + s.NetworkErrors
			if s.Successes > 0 && (item.P95LatencyMs == nil || s.P95LatencyMs > *item.P95LatencyMs) {
				p := s.P95LatencyMs
				item.P95LatencyMs = &p
			}
		}
		item.TotalRequests = &total
		item.DurationSeconds = &duration
		if duration > 0 {
			rate := float64(total) / duration
			item.AchievedRate = &rate
		}
		if eligible > 0 {
			rate := 100 * float64(errors) / float64(eligible)
			item.ErrorRatePercent = &rate
		}
	}
	if r.Analysis != nil {
		item.BreakpointStatus = r.Analysis.Breakpoint.Status
	}
	return item
}
