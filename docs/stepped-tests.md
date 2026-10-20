# Stepped load tests (Phase 5A)

## Use the dashboard

Start the existing **Storm: Start servers** VS Code task (API on 8080 and lab on
8081), and run `npm run dev` from the frontend directory. Alternatively, run
`go run ./cmd/storm-server` and `go run ./cmd/storm-lab` from the backend directory
in separate terminals. Keep their terminals open.

At http://localhost:5173 select **Stepped Load Test**. For a short demonstration:

- Target: `http://127.0.0.1:8081/variable`
- Workers: 4
- Starting RPS: 10
- RPS step size: 10
- Maximum RPS: 30
- Duration per stage: 2 seconds

Confirm authorization and select Run. The preview shows the exact generated
rates and configured load duration. Maximum RPS is a ceiling: 10 → 35 with a
step of 10 generates 10, 20, 30; it does not append a 35-RPS stage.

The progress panel shows the current stage, its target, stage load time, overall
elapsed time, and overall measured load progress. The bar excludes drain time,
so 100% load progress is not a promise that all requests have finished. Live
counts/rates and the interval timeline belong to the current stage only.

Completed-stage comparisons appear during execution. Final results include a P95
versus target chart, a target/actual-start/completed-rate chart, and a stage table.
Canceled partial stages remain in the table but are excluded from the comparison
charts because they did not run for the configured full stage duration. Stages
that never started are labeled **skipped** and have no invented measurements.

Stop cancels the active work and prevents later stages from starting. Refreshing
recovers the current stage, its measured interval history, and prior-stage results.
Switch back to **Standard Test** for the original fixed-rate workflow.

## Architecture and lifecycle

Both modes use the same [manager](../backend/internal/runs/manager.go), active-job
slot, cancellation context, retention policy, and execution adapter. A stepped
configuration produces at most ten engine configurations. Each calls the existing
`loadtest.RunWithProgress` with fresh collectors and latency samples. The next
stage is admitted only after the previous engine call returns, its HTTP connections
are closed, and its execution goroutine is joined. There are no overlapping stage
load windows and no separate duplicate HTTP engine.

Stop and stage admission synchronize on the manager mutex. Once cancellation is
acknowledged, no future stage is admitted. If cancellation races with a stage that
was already admitted, the shared canceled context prevents new traffic and cancels
outstanding requests. The slot remains occupied through cleanup of the entire job.
Server shutdown uses the same cancellation and join path.

Whole-run summary fields and standard-mode responses remain available as before.
A stepped run has **per-stage summaries**, not an aggregate latency summary.
Percentiles from separate stages cannot be averaged into a meaningful whole-job
percentile. Each stage also retains separate interval history, with sequence IDs
local to that stage; frontend buffers reset on stage changes.

## API

Use the existing endpoints:

| Endpoint | Behavior |
| --- | --- |
| `POST /api/tests` | Starts either mode; 202 Accepted, or 409 if any job is active. |
| `GET /api/tests/{id}` | Current stage progress and finished-stage results. |
| `GET /api/tests/active` | Active standard or stepped job, or `{"run":null}`. |
| `POST /api/tests/{id}/stop` | 202 cancellation acknowledgment; poll for final cleanup. |

An omitted `mode` retains standard behavior. A stepped payload is:

```json
{
  "mode": "stepped",
  "url": "http://127.0.0.1:8081/variable",
  "method": "GET",
  "workers": 4,
  "start_rps": 10,
  "rps_increment": 10,
  "max_rps": 30,
  "stage_duration_seconds": 2,
  "authorized": true
}
```

Do not mix standard `rps`/`duration_seconds` fields with stepped fields. Requests
require `Content-Type: application/json`; existing Host/Origin, authorization,
GET-only, and loopback protections remain in effect.

PowerShell example:

```powershell
$base = 'http://127.0.0.1:8080'
$body = @{
    mode = 'stepped'
    url = 'http://127.0.0.1:8081/variable'
    method = 'GET'
    workers = 4
    start_rps = 10
    rps_increment = 10
    max_rps = 30
    stage_duration_seconds = 2
    authorized = $true
} | ConvertTo-Json
$run = Invoke-RestMethod "$base/api/tests" -Method Post -ContentType 'application/json' -Body $body
Invoke-RestMethod "$base/api/tests/$($run.id)"
# Optional cancellation:
Invoke-RestMethod "$base/api/tests/$($run.id)/stop" -Method Post -ContentType 'application/json' -Body '{}'
```

Stepped snapshots add `current_stage` (one-based, zero before stage admission),
`total_stages`, `overall_elapsed_seconds`, `overall_load_seconds`,
`planned_load_seconds`, and `stages`. `progress` is the current stage's existing
engine progress, including `target_rps` and stage `elapsed_seconds`.

Each stage contains `number`, `target_rps`, `status`, `started_at`, `completed_at`,
`actual_start_rps`, `intervals`, and a final `summary` when executed. That summary
contains successes, failures, completed throughput, P50/P95/P99, missed schedules,
and all existing request-accounting fields. The top-level `summary` is omitted
for stepped jobs. Counts and percentile values are never copied from prior stages.

## Rate definitions and interpretation

- Actual start rate: stage started requests / measured stage load window.
- Completed throughput: stage recorded outcomes / total stage execution time,
  including graceful draining. Outcomes include failures and cancellations.
- Target RPS: configured global request-start target, not guaranteed throughput.
- P50/P95/P99: nearest-rank latency among that stage's successful HTTP 2xx responses.
  No successful samples means `N/A` in the UI. Small samples have limited tail meaning.

Missed schedules or a gap between target and actual starts can reflect worker or
generator constraints. They are not proof of target-server overload. Optional
[Breakpoint Discovery](breakpoint-discovery.md) assesses completed stages against
user thresholds with evidence gates; it does not establish guaranteed capacity.

## Limits

- 1–10 workers; rates and increments 1–100; maximum at least starting RPS.
- 1–10 stages; 1–30 seconds per stage; at most 120 seconds of configured load.
- Each stage may add up to the existing 3-second graceful drain, giving a configured
  load-plus-drain budget of at most 150 seconds. OS scheduling can add overhead.
- One active job across both modes; at most 20 finished jobs retained in memory.
- Each engine call creates its own client/connection pool, so connection warmup
  repeats per stage. Target-server state is not reset between stages.
- The interval timeline shows the current/last stage. Prior stage histories are saved in SQLite and can be selected in the saved report.
- Backend restart interrupts active jobs; saved reports survive in Test History. Refresh restores active jobs, not a completed dashboard automatically. No adaptive load scheduling, guaranteed capacity claims, cloud load, or exports.

## Validation

From `backend`: `go test ./...`, `go test -race ./...`, and `go vet ./...`.
On Windows race detection uses the installed MSYS2 GCC and `CGO_ENABLED=1` as
described in [the existing setup guide](async-tests.md).

From `frontend`: `npm run lint`, `npm run build`, and
`node --test src/hooks/timeline.test.js src/components/dashboard/stagePlan.test.js`.

Tests cover exact plan generation and limits, legacy API compatibility, cross-mode
conflicts, deterministic stage result isolation, sequential timestamps/cleanup,
cancellation at a stage boundary and during slow requests, shutdown, final
accounting, snapshot isolation, and frontend stage-buffer reset.

## Files changed

- [Configuration](../backend/internal/runs/config.go): mode validation and stage generation.
- [Manager](../backend/internal/runs/manager.go): shared execution adapter, sequential orchestration, copied stage results.
- [Stepped manager tests](../backend/internal/runs/stepped_test.go) and [API tests](../backend/internal/api/stepped_test.go).
- [Form](../frontend/src/components/dashboard/TestConfig.jsx), [stage plan helper](../frontend/src/components/dashboard/stagePlan.js), and [plan tests](../frontend/src/components/dashboard/stagePlan.test.js).
- [Dashboard controller](../frontend/src/components/TestForm.jsx), [live progress](../frontend/src/components/dashboard/LiveProgress.jsx), and [stage results](../frontend/src/components/dashboard/SteppedResults.jsx).
- [Timeline merging](../frontend/src/hooks/timeline.js), [timeline tests](../frontend/src/hooks/timeline.test.js), and [styles](../frontend/src/App.css).
- This guide and [README](../README.md). No new dependencies or load-generation algorithm changes.
