# Run comparisons (Phase 6B)

## Workflow

Open **Test History** and select exactly two saved reports. The first selected
report is **Baseline**, the second **Candidate**. The third selection is disabled
until one of those is removed. Selection survives pagination and navigation from
Compare Runs back to history. **Swap selection** changes roles before opening;
**Swap Baseline and Candidate** reverses them in the comparison view.

Comparisons use the existing two `GET /api/history/{id}` requests. There are no
new API endpoints, tables, engine changes, or permanent comparison records.
Saved reports are never edited. A deleted/unavailable report produces a clear
error with retry and navigation back to history.

The comparison URL is `#history/compare/{baseline-id}/{candidate-id}`. Refreshing
that URL reloads both saved reports and restores their roles. Returning to history
retains the selection and page offset in the current page session. Navigation to
Dashboard or a full reload of the history list can reset that transient selection.

## Compatibility

The pure helpers compare full target URL strings (including path/query), HTTP
method, mode, workers, configured standard rate/duration, stepped start/increment/
maximum/stage duration, recorded stage schedule, and configured analysis thresholds.
Query strings are not reordered or stripped: differing URLs are conservatively
different workloads, even if an application might interpret them equivalently.

- **Comparable configuration:** matching settings and complete, internally
  consistent results. This does not prove equivalent environments.
- **Different configuration — inspection only:** material settings differ.
  Raw values/deltas remain available but no favorable/unfavorable labels are shown.
- **Unsupported or insufficient data:** unsupported modes, incomplete/canceled/
  interrupted runs, missing fields, invalid accounting, or inconsistent stage data.
  Available measurements remain inspectable without performance conclusions.

Matching configuration is not a guarantee of equal machine/network conditions,
target state, background load, connection warmup, or response populations.

## Metrics and interpretation

Standard comparison rows include total/successful/failed requests, completed
throughput, actual starts, average/P50/P95/P99 successful-response latency, HTTP
error rate, and missed scheduling opportunities.

```text
signed absolute change = Candidate - Baseline
relative change (%) = (Candidate - Baseline) / Baseline * 100
actual start rate = started_requests / load_window_seconds
HTTP error rate (%) = http_error_count / (successes + http_error_count) * 100
```

Zero or missing baselines give `N/A` for percentage change, including 0 → 0;
the signed absolute change can still be zero. Missing values, invalid negative
metrics, and non-finite results remain unavailable. Error-rate absolute changes
are **percentage points (pp)**; relative changes are percentages. This HTTP-only
rate excludes network errors and cancellations; the Failed requests row still
contains all failures. History's HTTP/transport error-rate column has a different,
explicitly labeled denominator.

No successes means latency is unavailable, not measured zero. Latency statistics
describe successful responses only and should be inspected alongside failures.
Request counts, actual start rate, and missed-schedule counts are neutral changes.

Directional latency/error/throughput labels additionally require both achieved
start rates to be **95–105% of target**, within **5 percentage points** of each
other, and no canceled outcomes. For stepped tests this check is per
matched stage. This comparison heuristic is independent of Breakpoint Discovery's
user-configured start threshold. A higher failure fraction suppresses throughput
favorability. There is no automatic whole-run improved/degraded verdict, confidence
test, or claim of sustainable capacity.

## Charts and stepped results

- Grouped latency bars compare saved P50/P95/P99. Baseline is the left bar,
  Candidate the right; labels and positions supplement color.
- Completed-throughput overlays are shown only for comparable standard
  configurations. Actual interval-end elapsed seconds are used, never wall-clock
  timestamps. Missing timestamps/windows remain null gaps. No nearest-neighbor
  resampling, smoothing, or synthetic measurements are introduced. Straight segments
  only connect adjacent available points. Misaligned samples can appear as isolated
  dots. Baseline uses solid lines/circles; Candidate uses dashed lines/diamonds.
- Stepped comparisons pair **unique common target-RPS levels only**, displaying
  P95 and actual start rate. Unmatched levels are listed explicitly. Partial pairs
  remain inspection-only in tables and are excluded from completed-stage plots.
  Differing stage configurations suppress performance conclusions.
- If both reports contain saved breakpoint assessments, the view compares highest
  passing tested level, first failing tested level, status, and original messages.
  Missing assessments are never treated as PASS. A higher tested passing level
  does not establish increased sustainable capacity.

## Validation

From `frontend`:

```powershell
npm run lint
npm run build
node --test src/hooks/timeline.test.js src/components/dashboard/stagePlan.test.js src/components/dashboard/assessmentRendering.test.js src/components/history/history.test.js src/components/comparison/compare.test.js src/components/comparison/rendering.test.js
```

Existing backend regression tests remain `go test ./...` from `backend`.
Tests cover selection limits/swaps, compatibility, missing/zero values, deltas,
denominators, offered-load gates, standard/stepped reports, unmatched or missing
stages, canceled/interrupted runs, absent assessments, warning rendering, and
immutable input objects. Rendering-test Vite caches are isolated from the running
dashboard to avoid stale development dependency URLs.

Browser verification covered material-URL mismatch warnings, role swapping,
return-to-history selection, compatible standard and stepped comparisons, and
unchanged source reports. Two short `/fast` candidate tests were saved using the
existing engine to compare against the earlier standard and stepped demos.

## Files

Added [pure helpers/tests](../frontend/src/components/comparison/compare.js),
[comparison controller/view](../frontend/src/components/comparison/CompareRuns.jsx),
[charts](../frontend/src/components/comparison/ComparisonCharts.jsx),
[rendering tests](../frontend/src/components/comparison/rendering.test.js), and this guide.
Updated [history selection/navigation](../frontend/src/components/history/HistoryPage.jsx),
[styles](../frontend/src/App.css), [test cache setup](../frontend/src/components/dashboard/assessmentRendering.test.js),
[VS Code frontend task](../.vscode/tasks.json), and [README](../README.md).
No new dependencies or backend/database changes were needed for Phase 6B.

Limitations: comparisons are descriptive, not statistical evidence; stage histories
may represent differing target state despite matching settings; truncated/missing
time series are not reconstructed; selection is transient; unsupported or partial
data intentionally limits conclusions. No exports, cloud comparisons, adaptive
traffic, or persistent comparison tables were added.
