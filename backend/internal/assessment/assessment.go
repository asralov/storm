// Package assessment evaluates observed stepped-test results. It does not
// schedule traffic, infer server capacity, or claim statistical SLO confidence.
package assessment

import (
	"errors"
	"fmt"
	"math"

	"github.com/asralov/storm/internal/loadtest"
)

const MinErrorOutcomes = 100
const MinLatencySuccesses = 100

type Thresholds struct {
	MaxP95LatencyMs     *float64 `json:"max_p95_latency_ms"`
	MaxErrorRatePercent *float64 `json:"max_error_rate_percent"`
	MinStartRatePercent *float64 `json:"min_start_rate_percent"`
}

func finite(v float64) bool    { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func value(v float64) *float64 { return &v }
func (t *Thresholds) Validate() error {
	if t == nil || t.MaxP95LatencyMs == nil || t.MaxErrorRatePercent == nil || t.MinStartRatePercent == nil {
		return errors.New("all three breakpoint thresholds are required")
	}
	if v := *t.MaxP95LatencyMs; !finite(v) || v <= 0 || v > 60000 {
		return errors.New("P95 threshold must be greater than 0 and at most 60000 ms")
	}
	if v := *t.MaxErrorRatePercent; !finite(v) || v < 0 || v > 100 {
		return errors.New("failure-rate threshold must be between 0 and 100 percent")
	}
	if v := *t.MinStartRatePercent; !finite(v) || v <= 0 || v > 100 {
		return errors.New("minimum achieved start rate must be greater than 0 and at most 100 percent")
	}
	return nil
}
func (t *Thresholds) Clone() *Thresholds {
	if t == nil {
		return nil
	}
	copy := *t
	if t.MaxP95LatencyMs != nil {
		copy.MaxP95LatencyMs = value(*t.MaxP95LatencyMs)
	}
	if t.MaxErrorRatePercent != nil {
		copy.MaxErrorRatePercent = value(*t.MaxErrorRatePercent)
	}
	if t.MinStartRatePercent != nil {
		copy.MinStartRatePercent = value(*t.MinStartRatePercent)
	}
	return &copy
}

type StageInput struct {
	Number    int
	TargetRPS int
	Status    string
	Summary   *loadtest.Summary
}
type Level struct {
	StageNumber int `json:"stage_number"`
	TargetRPS   int `json:"target_rps"`
}
type StageAssessment struct {
	Level
	MetricsAvailable     bool     `json:"metrics_available"`
	Status               string   `json:"status"`
	Reasons              []string `json:"reasons"`
	EligibleOutcomes     int      `json:"eligible_outcomes"`
	SuccessfulSamples    int      `json:"successful_samples"`
	ObservedStartRPS     *float64 `json:"observed_start_rps"`
	AchievedStartPercent *float64 `json:"achieved_start_percent"`
	ErrorRatePercent     *float64 `json:"error_rate_percent"`
	P95LatencyMs         *float64 `json:"p95_latency_ms"`
}
type TransitionRange struct {
	LowerTargetRPS int `json:"lower_target_rps"`
	UpperTargetRPS int `json:"upper_target_rps"`
}
type Breakpoint struct {
	Status            string           `json:"status"`
	Message           string           `json:"message"`
	OrderedTransition bool             `json:"ordered_transition"`
	Bounded           bool             `json:"bounded"`
	Range             *TransitionRange `json:"range,omitempty"`
	Reasons           []string         `json:"reasons"`
}
type SamplePolicy struct {
	MinErrorOutcomes    int    `json:"min_error_outcomes"`
	MinLatencySuccesses int    `json:"min_latency_successes"`
	ErrorRateDefinition string `json:"error_rate_definition"`
	StartRateDefinition string `json:"start_rate_definition"`
}
type Report struct {
	Thresholds          *Thresholds       `json:"thresholds"`
	SamplePolicy        SamplePolicy      `json:"sample_policy"`
	Stages              []StageAssessment `json:"stages"`
	HighestPassingStage *Level            `json:"highest_passing_stage"`
	FirstFailingStage   *Level            `json:"first_failing_stage"`
	Breakpoint          Breakpoint        `json:"breakpoint"`
	Limitations         []string          `json:"limitations"`
}

func assessStage(stage StageInput, t *Thresholds) StageAssessment {
	a := StageAssessment{Level: Level{stage.Number, stage.TargetRPS}, Status: "INCONCLUSIVE", Reasons: []string{}}
	if stage.Status != "completed" {
		a.Reasons = append(a.Reasons, fmt.Sprintf("Stage is %s; only completed, uninterrupted stages can be assessed.", stage.Status))
	}
	s := stage.Summary
	if s == nil {
		a.Reasons = append(a.Reasons, "Final stage summary is unavailable.")
		return a
	}
	if stage.TargetRPS <= 0 || stage.Number <= 0 || s.Requests < 0 || s.Successes < 0 || s.Failures < 0 || s.HTTPErrorCount < 0 || s.NetworkErrors < 0 || s.Canceled < 0 || s.StartedRequests < 0 || s.Requests != s.Successes+s.Failures || s.Failures != s.HTTPErrorCount+s.NetworkErrors+s.Canceled || s.StartedRequests != s.Requests {
		a.Reasons = append(a.Reasons, "Stage target or request accounting is invalid; measurements cannot be assessed.")
		return a
	}
	a.MetricsAvailable = true
	a.SuccessfulSamples = s.Successes
	a.EligibleOutcomes = s.Successes + s.HTTPErrorCount + s.NetworkErrors
	if a.EligibleOutcomes > 0 {
		a.ErrorRatePercent = value(100 * float64(s.HTTPErrorCount+s.NetworkErrors) / float64(a.EligibleOutcomes))
	}
	if s.Successes > 0 {
		if !finite(s.P95LatencyMs) || s.P95LatencyMs < 0 {
			a.Reasons = append(a.Reasons, "Successful-response P95 is invalid.")
			return a
		}
		a.P95LatencyMs = value(s.P95LatencyMs)
	}
	if s.LoadWindowSeconds <= 0 || !finite(s.LoadWindowSeconds) {
		a.Reasons = append(a.Reasons, "A valid measured load duration is required to assess the request-start rate.")
		return a
	}
	startRPS := float64(s.StartedRequests) / s.LoadWindowSeconds
	percent := startRPS / float64(stage.TargetRPS) * 100
	if !finite(startRPS) || !finite(percent) {
		a.Reasons = append(a.Reasons, "The observed start rate is invalid.")
		return a
	}
	a.ObservedStartRPS, a.AchievedStartPercent = value(startRPS), value(percent)
	// Explicit run cancellation is excluded; request timeouts are transport errors.
	if s.Canceled > 0 {
		a.Reasons = append(a.Reasons, fmt.Sprintf("%d canceled outcomes make this stage inconclusive; these outcomes are excluded from the HTTP/transport error-rate denominator.", s.Canceled))
	}
	if startRPS < float64(stage.TargetRPS)**t.MinStartRatePercent/100 {
		a.Reasons = append(a.Reasons, fmt.Sprintf("Achieved start rate is %g%% of target, below %g%%. Requested offered load was not demonstrated; this does not prove server overload.", percent, *t.MinStartRatePercent))
	}
	if a.EligibleOutcomes < MinErrorOutcomes {
		a.Reasons = append(a.Reasons, fmt.Sprintf("Only %d eligible outcomes; at least %d are required to assess error rate.", a.EligibleOutcomes, MinErrorOutcomes))
	}
	if len(a.Reasons) > 0 {
		return a
	}
	// A sufficiently sampled violation is decisive even when a different metric
	// is unavailable: 100 HTTP failures can FAIL without inventing a latency value.
	if float64(s.HTTPErrorCount+s.NetworkErrors)*100 > *t.MaxErrorRatePercent*float64(a.EligibleOutcomes) {
		a.Reasons = append(a.Reasons, fmt.Sprintf("HTTP/transport error rate %g%% exceeds %g%%.", *a.ErrorRatePercent, *t.MaxErrorRatePercent))
	}
	if s.Successes >= MinLatencySuccesses && s.P95LatencyMs > *t.MaxP95LatencyMs {
		a.Reasons = append(a.Reasons, fmt.Sprintf("P95 latency %g ms exceeds %g ms.", s.P95LatencyMs, *t.MaxP95LatencyMs))
	}
	if len(a.Reasons) > 0 {
		a.Status = "FAIL"
		if s.Successes < MinLatencySuccesses {
			a.Reasons = append(a.Reasons, fmt.Sprintf("P95 was not assessed: %d successful samples, fewer than %d. The failure is established by error rate.", s.Successes, MinLatencySuccesses))
		}
		return a
	}
	if s.Successes < MinLatencySuccesses {
		a.Reasons = append(a.Reasons, fmt.Sprintf("Only %d successful samples; at least %d are required to assess P95. Missing latency is not zero.", s.Successes, MinLatencySuccesses))
		return a
	}
	a.Status = "PASS"
	a.Reasons = append(a.Reasons, "Observed P95 and error rate are within the configured limits, with sufficient samples and achieved offered load.")
	return a
}

// Analyze is deterministic and read-only. Input order is the tested stage order.
// It returns no bounded conclusion for gaps, interruptions, or reversed outcomes.
func Analyze(t *Thresholds, stages []StageInput, runStatus string) (Report, error) {
	if err := t.Validate(); err != nil {
		return Report{}, err
	}
	r := Report{Thresholds: t.Clone(), Stages: []StageAssessment{}, SamplePolicy: SamplePolicy{MinErrorOutcomes, MinLatencySuccesses, "(HTTP errors + network errors) / (successes + HTTP errors + network errors) × 100; cancellations excluded and make a stage inconclusive", "started requests / measured load seconds / target RPS × 100"}, Limitations: []string{"These are observed test criteria, not a statistically validated service-level objective or guaranteed sustainable throughput.", "The 100-sample minimums are practical evidence gates, not confidence intervals. Repeat tests to check reproducibility."}}
	r.Breakpoint = Breakpoint{Status: "inconclusive", Message: "There is insufficient evidence to localize a threshold crossing.", Reasons: []string{}}
	seenFail, inconsistent, unknown, ordered := false, false, false, true
	for i, stage := range stages {
		a := assessStage(stage, t)
		r.Stages = append(r.Stages, a)
		if stage.Number <= 0 || stage.TargetRPS <= 0 || (i > 0 && (stage.TargetRPS <= stages[i-1].TargetRPS || stage.Number <= stages[i-1].Number)) {
			ordered = false
		}
		switch a.Status {
		case "PASS":
			if seenFail {
				inconsistent = true
			}
			if r.HighestPassingStage == nil || a.TargetRPS > r.HighestPassingStage.TargetRPS {
				level := a.Level
				r.HighestPassingStage = &level
			}
		case "FAIL":
			seenFail = true
			if r.FirstFailingStage == nil {
				level := a.Level
				r.FirstFailingStage = &level
			}
		default:
			unknown = true
			r.Breakpoint.Reasons = append(r.Breakpoint.Reasons, fmt.Sprintf("Stage %d at %d target RPS is inconclusive; see its assessment reasons.", a.StageNumber, a.TargetRPS))
		}
	}
	if len(stages) == 0 {
		r.Breakpoint.Reasons = append(r.Breakpoint.Reasons, "No stages are available.")
		return r, nil
	}
	if runStatus == "pending" || runStatus == "running" || runStatus == "draining" {
		r.Breakpoint.Status = "in_progress"
		r.Breakpoint.Message = "Assessments are provisional while the stepped run is active; no breakpoint conclusion is available."
		return r, nil
	}
	if runStatus != "completed" {
		r.Breakpoint.Status = "partial"
		r.Breakpoint.Message = "The run was interrupted or did not complete. Partial stage assessments do not establish a definitive breakpoint."
		r.Breakpoint.Reasons = append(r.Breakpoint.Reasons, "Run status: "+runStatus+".")
		return r, nil
	}
	if !ordered {
		r.Breakpoint.Reasons = append(r.Breakpoint.Reasons, "Stage numbers and target rates must be strictly increasing.")
		return r, nil
	}
	if inconsistent {
		r.Breakpoint.Status = "inconsistent"
		r.Breakpoint.Message = "A passing stage followed a failing stage. Results are inconsistent with an ordered threshold crossing; repeat the tests."
		return r, nil
	}
	if unknown {
		return r, nil
	}
	if r.FirstFailingStage == nil {
		r.Breakpoint.Status = "not_reached"
		r.Breakpoint.Message = "All tested stages passed the observed criteria. A breakpoint was not reached within the tested levels."
		return r, nil
	}
	if r.HighestPassingStage == nil {
		r.Breakpoint.Status = "at_or_below_first"
		r.Breakpoint.Message = fmt.Sprintf("The first tested level (%d target RPS) failed. The observed threshold crossing is at or below that level, without a passing lower bound.", r.FirstFailingStage.TargetRPS)
		return r, nil
	}
	r.Breakpoint.Status = "bounded"
	r.Breakpoint.OrderedTransition, r.Breakpoint.Bounded = true, true
	r.Breakpoint.Range = &TransitionRange{r.HighestPassingStage.TargetRPS, r.FirstFailingStage.TargetRPS}
	r.Breakpoint.Message = fmt.Sprintf("Observed threshold crossing between %d and %d target RPS.", r.HighestPassingStage.TargetRPS, r.FirstFailingStage.TargetRPS)
	return r, nil
}
