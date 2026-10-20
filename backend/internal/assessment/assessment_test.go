package assessment

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/asralov/storm/internal/loadtest"
)

func thresholds() *Thresholds { return &Thresholds{value(200), value(1), value(95)} }
func stage(number, rate int, p95 float64) StageInput {
	return StageInput{Number: number, TargetRPS: rate, Status: "completed", Summary: &loadtest.Summary{Requests: 200, StartedRequests: 200, Successes: 200, LoadWindowSeconds: 200 / float64(rate), Throughput: 1, P95LatencyMs: p95}}
}
func analyze(t *testing.T, input []StageInput, status string) Report {
	t.Helper()
	r, err := Analyze(thresholds(), input, status)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestBreakpointPatterns(t *testing.T) {
	for _, tc := range []struct {
		name           string
		latencies      []float64
		status         string
		highest, first int
		bounded        bool
	}{
		{"all pass", []float64{100, 150, 200}, "not_reached", 30, 0, false},
		{"all fail", []float64{201, 250, 300}, "at_or_below_first", 0, 10, false},
		{"clean transition", []float64{100, 200, 201, 250}, "bounded", 20, 30, true},
		{"nonmonotonic", []float64{100, 201, 100}, "inconsistent", 30, 20, false},
		{"first fail then pass", []float64{201, 100}, "inconsistent", 20, 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input []StageInput
			for i, p95 := range tc.latencies {
				input = append(input, stage(i+1, (i+1)*10, p95))
			}
			r := analyze(t, input, "completed")
			if r.Breakpoint.Status != tc.status || r.Breakpoint.Bounded != tc.bounded || r.Breakpoint.OrderedTransition != tc.bounded {
				t.Fatalf("unexpected result: %+v", r.Breakpoint)
			}
			highest, first := 0, 0
			if r.HighestPassingStage != nil {
				highest = r.HighestPassingStage.TargetRPS
			}
			if r.FirstFailingStage != nil {
				first = r.FirstFailingStage.TargetRPS
			}
			if highest != tc.highest || first != tc.first {
				t.Fatalf("levels: %d %d", highest, first)
			}
			if tc.bounded {
				if r.Breakpoint.Range.LowerTargetRPS != 20 || r.Breakpoint.Range.UpperTargetRPS != 30 {
					t.Fatal("wrong bounds")
				}
			} else if r.Breakpoint.Range != nil {
				t.Fatal("unjustified range")
			}
		})
	}
}

func TestStageEvidenceGates(t *testing.T) {
	for _, tc := range []struct {
		name         string
		change       func(*StageInput)
		want, reason string
	}{
		{"too few outcomes", func(s *StageInput) {
			s.Summary = &loadtest.Summary{Requests: 99, StartedRequests: 99, Successes: 90, Failures: 9, HTTPErrorCount: 9, LoadWindowSeconds: 1, P95LatencyMs: 500}
		}, "INCONCLUSIVE", "eligible outcomes"},
		{"too few successes", func(s *StageInput) {
			s.Summary = &loadtest.Summary{Requests: 100, StartedRequests: 100, Successes: 99, Failures: 1, HTTPErrorCount: 1, LoadWindowSeconds: 1, P95LatencyMs: 500}
		}, "INCONCLUSIVE", "successful samples"},
		{"generator shortfall", func(s *StageInput) { s.Summary.LoadWindowSeconds = 40; s.Summary.P95LatencyMs = 1000 }, "INCONCLUSIVE", "offered load"},
		{"zero successes fail on errors", func(s *StageInput) {
			s.Summary.Successes = 0
			s.Summary.Failures = 200
			s.Summary.HTTPErrorCount = 200
			s.Summary.P95LatencyMs = 0
		}, "FAIL", "error rate"},
		{"zero outcomes", func(s *StageInput) { s.Summary = &loadtest.Summary{LoadWindowSeconds: 10} }, "INCONCLUSIVE", "eligible outcomes"},
		{"canceled stage", func(s *StageInput) { s.Status = "canceled" }, "INCONCLUSIVE", "uninterrupted"},
		{"canceled outcome", func(s *StageInput) { s.Summary.Successes--; s.Summary.Failures = 1; s.Summary.Canceled = 1 }, "INCONCLUSIVE", "canceled"},
		{"missing summary", func(s *StageInput) { s.Summary = nil }, "INCONCLUSIVE", "unavailable"},
		{"invalid counts", func(s *StageInput) { s.Summary.Requests++ }, "INCONCLUSIVE", "accounting"},
		{"invalid load window", func(s *StageInput) { s.Summary.LoadWindowSeconds = math.NaN() }, "INCONCLUSIVE", "load duration"},
		{"invalid latency", func(s *StageInput) { s.Summary.P95LatencyMs = math.Inf(1) }, "INCONCLUSIVE", "P95 is invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := stage(1, 20, 100)
			tc.change(&s)
			r := analyze(t, []StageInput{s}, "completed")
			a := r.Stages[0]
			if a.Status != tc.want || !strings.Contains(strings.Join(a.Reasons, " "), tc.reason) {
				t.Fatalf("unexpected assessment: %+v", a)
			}
			if s.Summary != nil && s.Summary.Successes == 0 && a.P95LatencyMs != nil {
				t.Fatal("invented zero latency")
			}
		})
	}
}

func TestBoundariesAndDenominators(t *testing.T) {
	s := stage(1, 20, 200)
	s.Summary.Successes = 198
	s.Summary.Failures = 2
	s.Summary.HTTPErrorCount = 1
	s.Summary.NetworkErrors = 1
	r := analyze(t, []StageInput{s}, "completed")
	a := r.Stages[0]
	if a.Status != "PASS" || *a.ErrorRatePercent != 1 || *a.AchievedStartPercent != 100 || *a.ObservedStartRPS != 20 {
		t.Fatalf("inclusive boundaries or denominators incorrect: %+v", a)
	}
	s.Summary.P95LatencyMs = 200.000001
	if analyze(t, []StageInput{s}, "completed").Stages[0].Status != "FAIL" {
		t.Fatal("latency above threshold passed")
	}
	s.Summary.P95LatencyMs = 100
	s.Summary.Successes = 197
	s.Summary.HTTPErrorCount = 2
	s.Summary.Failures = 3
	if analyze(t, []StageInput{s}, "completed").Stages[0].Status != "FAIL" {
		t.Fatal("error rate above threshold passed")
	}
	s = stage(1, 20, 100)
	s.Summary.Requests = 190
	s.Summary.StartedRequests = 190
	s.Summary.Successes = 190
	s.Summary.LoadWindowSeconds = 10
	if analyze(t, []StageInput{s}, "completed").Stages[0].Status != "PASS" {
		t.Fatal("95% offered-load boundary should pass")
	}
	s.Summary.Requests--
	s.Summary.StartedRequests--
	s.Summary.Successes--
	if analyze(t, []StageInput{s}, "completed").Stages[0].Status != "INCONCLUSIVE" {
		t.Fatal("offered load below boundary not gated")
	}
	s = stage(1, 20, 100)
	s.Summary.Requests = 225
	s.Summary.StartedRequests = 225
	s.Summary.Successes = 200
	s.Summary.Failures = 25
	s.Summary.HTTPErrorCount = 10
	s.Summary.NetworkErrors = 10
	s.Summary.Canceled = 5
	a = analyze(t, []StageInput{s}, "completed").Stages[0]
	if a.EligibleOutcomes != 220 || math.Abs(*a.ErrorRatePercent-100.0*20/220) > 1e-12 || a.Status != "INCONCLUSIVE" {
		t.Fatalf("cancellation denominator incorrect: %+v", a)
	}
	// An error ceiling of 100 does not make a no-success stage pass its latency criterion.
	top := thresholds()
	top.MaxErrorRatePercent = value(100)
	s = stage(1, 20, 0)
	s.Summary.Successes = 0
	s.Summary.Failures = 200
	s.Summary.HTTPErrorCount = 200
	r, err := Analyze(top, []StageInput{s}, "completed")
	if err != nil || r.Stages[0].Status != "INCONCLUSIVE" || r.Stages[0].P95LatencyMs != nil {
		t.Fatal("zero successes treated as successful latency")
	}
}

func TestIncompleteAndAmbiguousRuns(t *testing.T) {
	canceled := stage(1, 20, 100)
	canceled.Status = "canceled"
	partial := analyze(t, []StageInput{canceled}, "canceled").Stages[0]
	if partial.Status != "INCONCLUSIVE" || !partial.MetricsAvailable || partial.SuccessfulSamples != 200 || partial.EligibleOutcomes != 200 {
		t.Fatalf("partial observations were hidden: %+v", partial)
	}
	input := []StageInput{stage(1, 20, 100), stage(2, 30, 300)}
	for _, status := range []string{"canceled", "failed", "running", "pending", "draining"} {
		r := analyze(t, input, status)
		if r.Breakpoint.Bounded || r.Breakpoint.Range != nil {
			t.Fatal("partial or active run has a range")
		}
		if r.HighestPassingStage == nil || r.FirstFailingStage == nil {
			t.Fatal("partial observations lost")
		}
	}
	gap := stage(2, 30, 100)
	gap.Summary.LoadWindowSeconds = 40
	r := analyze(t, []StageInput{stage(1, 20, 100), gap, stage(3, 40, 300)}, "completed")
	if r.Breakpoint.Status != "inconclusive" || r.Breakpoint.Bounded {
		t.Fatal("range bridged an inconclusive stage")
	}
	r = analyze(t, []StageInput{stage(2, 30, 100), stage(1, 20, 300)}, "completed")
	if r.Breakpoint.Bounded {
		t.Fatal("unordered stages produced a range")
	}
	if analyze(t, nil, "completed").Breakpoint.Status != "inconclusive" {
		t.Fatal("empty run got a conclusion")
	}
}

func TestThresholdValidationAndDeterminism(t *testing.T) {
	for _, tune := range []func(*Thresholds){
		func(t *Thresholds) { t.MaxP95LatencyMs = nil }, func(t *Thresholds) { t.MaxErrorRatePercent = nil }, func(t *Thresholds) { t.MinStartRatePercent = nil },
		func(t *Thresholds) { t.MaxP95LatencyMs = value(0) }, func(t *Thresholds) { t.MaxP95LatencyMs = value(60001) }, func(t *Thresholds) { t.MaxP95LatencyMs = value(math.NaN()) },
		func(t *Thresholds) { t.MaxErrorRatePercent = value(-1) }, func(t *Thresholds) { t.MaxErrorRatePercent = value(101) }, func(t *Thresholds) { t.MaxErrorRatePercent = value(math.Inf(1)) },
		func(t *Thresholds) { t.MinStartRatePercent = value(0) }, func(t *Thresholds) { t.MinStartRatePercent = value(101) },
	} {
		settings := thresholds()
		tune(settings)
		if _, err := Analyze(settings, nil, "completed"); err == nil {
			t.Fatal("invalid thresholds accepted")
		}
	}
	if _, err := Analyze(nil, nil, "completed"); err == nil {
		t.Fatal("missing thresholds accepted")
	}
	settings := thresholds()
	settings.MaxErrorRatePercent = value(0)
	if err := settings.Validate(); err != nil {
		t.Fatal("zero error tolerance must be valid")
	}
	input := []StageInput{stage(1, 20, 100), stage(2, 30, 300)}
	a, _ := Analyze(settings, input, "completed")
	b, _ := Analyze(settings, input, "completed")
	if !reflect.DeepEqual(a, b) {
		t.Fatal("analysis is not deterministic")
	}
	*a.Thresholds.MaxP95LatencyMs = 999
	if *settings.MaxP95LatencyMs != 200 {
		t.Fatal("returned report aliases threshold input")
	}
}
