# Performance lab and interval charts

## Start the three applications

In two separate terminals, from `backend`:

```powershell
go run ./cmd/storm-server
```

```powershell
go run ./cmd/storm-lab
```

In a third terminal, from `frontend`:

```powershell
npm run dev
```

Open http://localhost:5173. Test `http://127.0.0.1:8081/variable` with 10 workers,
60 req/s, and 10 seconds, after confirming authorization. These settings generally
provide enough successes per window to see the latency distribution. Other settings
can produce different completed distributions and fewer samples.

The lab owns port 8081; stop any older demo HTTP server on that port first. Storm's
API stays on 127.0.0.1:8080. Ctrl+C stops either server gracefully and cancels its
outstanding requests. The lab does not generate outbound traffic.

## Lab endpoints

All endpoints use GET and are relative to `http://127.0.0.1:8081`.

| Endpoint | Behavior |
| --- | --- |
| `/health` | JSON service status. |
| `/fast` | Small JSON response with no intentional delay. |
| `/slow?ms=100` | Default 100 ms delay. Integer `ms` must be 0–1500. Invalid values return 400. |
| `/variable` | Deterministic 100-arrival cycle: 80 at 10 ms, 15 at 100 ms, 4 at 400 ms, 1 at 900 ms. |
| `/errors?percent=20` | Default 20% HTTP 500, otherwise 200. Integer percentage is 0–100; exactly that many failures in each full 100-arrival cycle with constant settings. |
| `/degrade` | Two concurrent service slots, each taking 100 ms. Excess requests wait up to 500 ms, then return 503. |

`/variable` and `/errors` each maintain one atomic arrival counter. Their cycle
position is `(sequence * 37) % 100`, which spreads slower/error responses through
the cycle instead of grouping them together. Counters restart with the lab process.
Concurrent completion order is naturally affected by the delays.

Supply `seq=0` through `seq=1000000` to choose a position explicitly without
advancing that endpoint's counter. Examples:

- `/variable?seq=0`: 10 ms.
- `/variable?seq=8`: 400 ms.
- `/variable?seq=27`: 900 ms.
- `/errors?percent=25&seq=0`: HTTP 500.

Holding `seq` fixed during a load test repeats that single outcome, rather than
sampling the distribution. Arrival proportions do not guarantee identical
proportions in every *completion* window. Even across 100 observed successful
completions, overhead and concurrent in-flight work affect measured percentiles.

For `/degrade`, roughly 20 requests/s is the service capacity before queue overhead.
Increasing offered load or worker count can add queue latency and eventually 503s.
This is an intentionally simplified queue model, not automatic capacity discovery.
It has at most 64 admitted handlers (including queued work); further requests get
503 immediately. The listener also caps accepted connections at 128, with finite
header/read/write/idle timeouts. There are no per-client maps or background jobs.
All waits, including queued work, respect request cancellation.

## Interval measurement contract

Every run snapshot from `GET /api/tests/{id}` and `GET /api/tests/active` now includes
an `intervals` array. It contains up to 300 windows, also retained after completion
under the existing 20-finished-run policy. The API and engine remain independent
of the charting library.

Each full interval covers `[start_seconds, elapsed_seconds)` with a one-second
duration. Worker completion timestamps assign outcomes to windows; request start
time does not determine the window. A request starting in one window and completing
in the next contributes only to the latter. Late channel delivery can correct an
earlier window under the same sequence ID. The frontend upserts those corrections.

After the final result is drained, the engine includes the final partial window.
The manager takes final history directly from the returned summary's internal
history, so dropping intermediate nonblocking progress messages cannot lose final
interval outcomes. Intermediate messages carry the retained history, allowing the
next successful poll to recover missed windows. History is copied for concurrent
status readers.

| Interval field | Definition |
| --- | --- |
| `sequence` | One-based window ID within this run. |
| `start_seconds`, `elapsed_seconds`, `duration_seconds` | Actual window bounds and denominator. |
| `completed` | All recorded outcomes in the window, including errors/cancellations. |
| `successes`, `failures` | Outcome classification for this window. |
| `http_errors`, `network_errors`, `canceled` | Failure breakdown; these sum to failures. |
| `throughput` | Completed outcomes divided by this window's actual duration. |
| `avg_latency_ms`, `p50_latency_ms`, `p95_latency_ms`, `p99_latency_ms` | Latency of successful HTTP 2xx responses whose bodies completed in this window. |

Missing successful samples produce JSON `null` for all four latency statistics.
Percentiles use nearest rank, `ceil(p*N)`, and sort only the window's samples.
The full-run summary retains its existing independent calculation. Empty windows
have zero counts/rate and unavailable latency. Short final windows can have volatile
rates; the tooltip includes the duration so these can be interpreted correctly.

With one sample all percentiles equal that sample; P99 with fewer than 100 samples
is often the maximum. These are mathematically valid nearest ranks, not stable
estimates of a population's tail latency. Tooltips flag fewer than 20 successes.

## Chart behavior

- Latency uses interval P50/P95/P99/Average, with distinct colors and dash patterns.
  Legend buttons support keyboard use and expose pressed state. Hiding all series
  shows an explanatory empty state. Tooltips display up to six decimal places in ms.
- Throughput uses completed outcomes per interval second. The dashed reference
  line is explicitly the **request-start target**, not a promised completion rate.
- Failures are per interval, with actual total/HTTP/network/canceled series.
- Summary cards continue to show whole-run statistics. No cumulative average is
  mixed into the latency chart.
- Final charts remain visible. A new run resets data and legend state. Refreshing
  an active run restores retained backend windows, including those missed by polling.
- Straight segments connect measurements. There is no smoothing, interpolation of
  new samples, animation, WebSocket stream, or persistent history database.

## Validation and limits

Run backend checks from `backend`:

```powershell
go test ./...
$env:Path = "C:\msys64\ucrt64\bin;$env:Path"
$env:CGO_ENABLED = '1'
go test -race ./...
go vet ./...
```

Run frontend checks from `frontend`:

```powershell
npm run lint
npm run build
node --test src/hooks/timeline.test.js
```

Deterministic tests cover interval boundaries, late outcomes, nearest ranks, empty
windows, history cloning/retention, lab status mixtures, and cancellation. Real
`httptest` requests verify fixed delays, differing variable-workload percentiles,
and error classification. Existing completion, draining, cancellation, and concurrent
read tests also verify interval totals against final counts.

At current limits (30-second load window, at most 3 seconds of drain), 300 windows
cover normal runs in full. If a run ever exceeds retained history, sums apply only
to retained windows and the UI flags missing earlier windows. OS scheduling and
HTTP overhead affect observed delays. The lab is a demonstration model, not a
production benchmark. Saved reports and charts survive backend restart in Test History; refreshing an
already-completed dashboard does not automatically reopen that run.

## Files

New: [lab entry point](../backend/cmd/storm-lab/main.go), [lab handlers](../backend/internal/lab/server.go),
[bounded listener](../backend/internal/lab/listener.go), [lab tests](../backend/internal/lab/server_test.go),
[interval collector](../backend/internal/loadtest/interval.go), [interval tests](../backend/internal/loadtest/interval_test.go), and this guide.

Updated: [engine](../backend/internal/loadtest/engine.go), [metrics](../backend/internal/loadtest/metrics.go),
[progress](../backend/internal/loadtest/progress.go), [manager](../backend/internal/runs/manager.go),
[manager tests](../backend/internal/runs/manager_test.go), [charts](../frontend/src/components/dashboard/LiveCharts.jsx),
[live progress copy](../frontend/src/components/dashboard/LiveProgress.jsx), [dashboard](../frontend/src/components/TestForm.jsx),
[timeline merging](../frontend/src/hooks/timeline.js), [timeline tests](../frontend/src/hooks/timeline.test.js),
[styles](../frontend/src/App.css), [asynchronous guide](async-tests.md), and [README](../README.md).
No dependency changes are required for Phase 4.
