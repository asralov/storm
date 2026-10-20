# Breakpoint Discovery (Phase 5B)

This optional feature assesses existing stepped-test summaries against user
thresholds. It does not schedule adaptive traffic, infer guaranteed server
capacity, or claim statistical SLO confidence.

## Local example

Start **Terminal → Run Task → Storm: Start servers** in VS Code and keep the task
terminals open. Start `npm run dev` from the frontend directory if needed. Open
http://localhost:5173, choose **Stepped Load Test**, and enable **Breakpoint Discovery**.

Use `http://127.0.0.1:8081/fast`, 4 workers, start 40 RPS, increment 20, maximum
60 RPS, and 3 seconds per stage. Keep the suggested thresholds: **200 ms P95,
1% errors, 95% achieved starts**. This normally supplies over 100 successes per
stage and reports PASS / breakpoint not reached, subject to actual measurements.

Other deliberate demonstrations:

| Target and configuration | Expected distinction |
| --- | --- |
| `/slow?ms=100`, 10 workers, one 40-RPS stage for 3 s, P95 threshold 50 ms | Latency FAIL when sample and offered-load requirements are met. |
| `/errors?percent=100`, 4 workers, one 40-RPS stage for 3 s | Intentional error-rate FAIL, with unavailable P95. |
| `/slow?ms=1000`, 1 worker, one 100-RPS stage for 2 s | INCONCLUSIVE offered load/sample count; not proof of server overload. |

Include the endpoint path: the lab's bare root URL returns 404. The ordinary
stepped defaults (10 RPS for 5 s initially) are too short for the 100-outcome gate.
Use longer stages at low rates. No recipe guarantees a timing result or crossing.

## Thresholds and denominators

Include `thresholds` only for opted-in stepped jobs. Omitted or null disables
analysis. All three fields are required in a non-null object; empty objects,
missing/null values, non-finite values, and out-of-range values are rejected.

| Field | Range |
| --- | --- |
| `max_p95_latency_ms` | > 0 and ≤ 60000 ms |
| `max_error_rate_percent` | 0–100%, including valid zero tolerance |
| `min_start_rate_percent` | > 0 and ≤ 100% |

Equality satisfies a threshold. Decisions use unrounded values. UI formatting
does not change decisions; backend reasons include the measured numeric values.

```text
observed_start_rps = started_requests / load_window_seconds
achieved_start_percent = observed_start_rps / target_rps * 100

eligible_outcomes = successes + http_error_count + network_errors
error_rate_percent = (http_error_count + network_errors) / eligible_outcomes * 100
```

Completed throughput is not substituted for request-start rate: it includes
draining in its time denominator. HTTP errors follow the engine's existing
non-2xx classification, including unfollowed redirects. P95 uses fully read
successful HTTP 2xx response bodies only.

Request timeouts (including body-read timeouts) are transport errors when the run context is still active. Explicit Stop, shutdown, or run-context termination is cancellation. Cancellations are excluded from the error-rate denominator and make a stage INCONCLUSIVE. Reports saved before Phase 7B retain their original timeout classification; rerun them for the corrected semantics.

## Stage assessment algorithm

1. Require a completed, uninterrupted stage with a summary, valid accounting,
   a positive finite load duration, and a valid target. Otherwise INCONCLUSIVE.
2. Require the configured achieved-start percentage. A shortfall is INCONCLUSIVE
   even if latency/error thresholds appear violated.
3. Require **100 eligible outcomes** for error-rate assessment and no canceled outcomes. Failure of either evidence gate is INCONCLUSIVE.
4. A sufficiently sampled error-rate violation establishes FAIL. P95 can also
   establish FAIL when there are at least **100 successful responses**. Both
   reasons are returned if both adequately sampled metrics violate their limits.
5. If there is no established violation, fewer than 100 successes leaves the
   latency criterion INCONCLUSIVE. Only sufficient evidence and all passing
   criteria produce PASS.

Zero successes produce `p95_latency_ms: null`, never a measured zero. One hundred
HTTP failures can still establish an error-rate FAIL. If the error ceiling is
100%, that same stage is INCONCLUSIVE because successful-response latency remains
unavailable. These minimums are practical gates, **not confidence intervals or a
statistically validated service-level objective**. Repeat tests for reproducibility.

## Overall breakpoint interpretation

The report identifies highest passing and first failing tested levels as
descriptive observations, including in partial runs.

| Status | Meaning |
| --- | --- |
| `in_progress` | Provisional assessments; the job is still active. |
| `partial` | Canceled, failed, or interrupted job; no definitive range. |
| `inconclusive` | Missing evidence, an inconclusive stage, invalid order, or no stages. |
| `inconsistent` | A PASS follows a FAIL. Repeat tests; no range is invented. |
| `not_reached` | All tested stages passed; breakpoint not observed at tested levels. |
| `at_or_below_first` | First tested level failed with no passing level; no passing lower bound. |
| `bounded` | Completed, strictly ordered PASS stages followed by FAIL stages, without gaps. |

Only `bounded` sets `ordered_transition: true`, `bounded: true`, and a `range`
between the highest passing target and first failing target. Wording is
“Observed threshold crossing between X and Y target RPS.” It does not describe
a guaranteed maximum sustainable throughput. No range bridges an inconclusive
stage. Interrupted jobs never receive a definitive range, even if some earlier
stages form a PASS-to-FAIL pattern.

## API additions

Existing endpoints and fields are preserved. Example `POST /api/tests` payload:

```json
{
  "mode": "stepped",
  "url": "http://127.0.0.1:8081/fast",
  "method": "GET",
  "workers": 4,
  "start_rps": 40,
  "rps_increment": 20,
  "max_rps": 60,
  "stage_duration_seconds": 3,
  "authorized": true,
  "thresholds": {
    "max_p95_latency_ms": 200,
    "max_error_rate_percent": 1,
    "min_start_rate_percent": 95
  }
}
```

Use `Content-Type: application/json`. A non-null threshold object in standard
mode returns 400. Standard and stepped requests without thresholds behave as before.

Start/status/active snapshots include optional `analysis` when enabled:

- `thresholds`: independent copy of the values used.
- `sample_policy`: minimum counts and rate definitions.
- `stages`: stage number/target, verdict, reasons, eligible/success counts,
  observed start rate, achieved-start percentage, error percentage, nullable P95,
  and `metrics_available`. Partial counts are preserved for canceled stages even
  though their verdict is INCONCLUSIVE; missing final metrics are labeled unavailable.
- `highest_passing_stage`, `first_failing_stage`: nullable tested-level objects.
- `breakpoint`: status, message, reasons, ordered/bounded flags, optional range.
- `limitations`: explicit observation/statistical caveats for display.

Analysis reads whole-stage summaries, not interval-percentile averages. The
manager computes each report from its copied run snapshot so cancellation cannot
leave a stale cached conclusion. Thresholds are copied on acceptance and reads.
The analyzer is independent of HTTP, React, and scheduling.

## UI and checks

Threshold settings are collapsible and restored on active-run recovery. The panel
displays backend conclusions and observed bounds. The table adds badges and
specific evidence. Chart markers combine color and shape (circle/diamond/triangle),
and tooltips include their verdict. The frontend does not reimplement assessments.

From `backend`:

```powershell
go test ./...
$env:Path = "C:\msys64\ucrt64\bin;$env:Path"
$env:CGO_ENABLED = '1'
go test -race ./...
go vet ./...
```

From `frontend`:

```powershell
npm run lint
npm run build
node --test src/hooks/timeline.test.js src/components/dashboard/stagePlan.test.js src/components/dashboard/assessmentRendering.test.js
```

Tests cover all-pass/all-fail/clean/non-monotonic patterns, evidence gaps, generator
shortfalls, zero successes, cancellation and transport timeouts, invalid/missing thresholds,
equality boundaries, denominators, immutable snapshots, API compatibility, all
three rendered verdicts, explanations, partial/bounded output, markers, and controls.

## Changed files and limitations

Added [analyzer](../backend/internal/assessment/assessment.go) and [tests](../backend/internal/assessment/assessment_test.go),
[manager tests](../backend/internal/runs/assessment_test.go), [API tests](../backend/internal/api/assessment_test.go),
[assessment panel/markers](../frontend/src/components/dashboard/BreakpointAssessment.jsx),
[rendering tests](../frontend/src/components/dashboard/assessmentRendering.test.js), and this guide.

Updated [configuration](../backend/internal/runs/config.go), [manager](../backend/internal/runs/manager.go),
[form](../frontend/src/components/dashboard/TestConfig.jsx), [stage comparisons](../frontend/src/components/dashboard/SteppedResults.jsx),
[styles](../frontend/src/App.css), [stepped guide](stepped-tests.md), and [README](../README.md).

No new dependencies or traffic algorithms were added. Existing job retention,
duration/RPS limits, and restart behavior are unchanged. Short stages, shared-machine
load, scheduler timing, connection warmup, target-state changes, and the fixed two-second request timeout limit interpretation. There is no adaptive scheduling,
automatic retesting, AI explanation, or capacity guarantee.
