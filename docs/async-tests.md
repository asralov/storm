# Asynchronous tests and live progress

## Run locally

From the repository root, run these in separate PowerShell terminals:

```powershell
cd backend
go run ./cmd/storm-server
```

```powershell
cd frontend
npm install # Only needed when dependencies are not installed.
npm run dev
```

Open http://localhost:5173. The backend binds to 127.0.0.1:8080. Vite proxies
`/api` with the browser's original Host header; its port is fixed at 5173.
An existing backend process must be restarted after backend source changes.

For a local target, start Storm Lab from the backend directory:

```powershell
go run ./cmd/storm-lab
```

Use `http://127.0.0.1:8081/variable` in the dashboard, confirm authorization, and run.
See [the lab guide](performance-lab.md) for all endpoints and interval definitions.
Stop requests cancellation; the UI waits for the terminal response. Refreshing
recovers an active run. Closing the page does not cancel it: configured duration
and request timeouts still bound its lifetime. Ctrl+C on the Go server cancels
active work and waits for its results before completing shutdown.

## API

`POST /api/tests` now returns **202**, not a synchronous summary. The accepted
response includes an ID, configuration, initial status, and progress. A second
active run returns **409**, including while the first run is draining or stopping.

```powershell
$base = 'http://127.0.0.1:8080'
$config = @{
    url = 'http://127.0.0.1:8081/variable'
    method = 'GET'
    workers = 2
    duration_seconds = 10
    rps = 20
    authorized = $true
} | ConvertTo-Json

$run = Invoke-RestMethod "$base/api/tests" -Method Post -ContentType 'application/json' -Body $config
$id = $run.id
Invoke-RestMethod "$base/api/tests/$id"
Invoke-RestMethod "$base/api/tests/active"
Invoke-RestMethod "$base/api/tests/$id/stop" -Method Post -ContentType 'application/json' -Body '{}'
Invoke-RestMethod "$base/api/tests/$id"
```

| Endpoint | Response |
| --- | --- |
| `POST /api/tests` | 202 run snapshot, initially `pending` |
| `GET /api/tests/{id}` | 200 snapshot, with `summary` after termination; 404 if unknown/evicted |
| `POST /api/tests/{id}/stop` | 202 `{id,status,cancellation_requested:true}`; repeated requests while active are safe; 409 after termination |
| `GET /api/tests/active` | 200 `{"run":{...}}` or `{"run":null}` |

Errors have the form `{"error":"message"}`. Mutation requests require
`Content-Type: application/json`. Start configuration is limited to 4 KiB and
one JSON object. HTTP method is GET, workers 1–10, rate 1–100 req/s, duration
1–30 seconds, authorization required. Omitted/zero RPS retains the legacy default 20.

## Lifecycle and architecture

- [Run manager](../backend/internal/runs/manager.go): one active slot, random ID,
  configuration, start/completion timestamps, cancellation context, and copied
  snapshots protected by a mutex. Contexts belong to the manager, not POST requests.
- [Engine](../backend/internal/loadtest/engine.go): workers execute requests while
  one collector records outcomes. A one-slot, nonblocking progress channel cannot
  delay traffic. Final results travel separately and are never dropped.
- [API](../backend/internal/api/tests.go): validates requests and exposes snapshots.
- [Frontend polling](../frontend/src/hooks/useTestRun.js): recovers active work on
  mount, polls sequentially every 750 ms after each response, retries read failures,
  and aborts requests/timers on unmount or terminal state. It never retries a POST
  automatically after an ambiguous network failure.

Normal lifecycle: `pending → running → draining → completed`. Draining can be
too brief to observe through polling. Explicit Stop or server shutdown sets a
cancellation flag, immediately cancels scheduling and request contexts, and
publishes `canceled` only after collection completes. Stop does not free the active
slot early. A run with HTTP/network failures can still be `completed`; `failed`
is reserved for an execution/accounting failure, not a failed target response.

Normal duration expiry allows the configured 3-second graceful drain. The existing
2-second HTTP request timeout still applies, so it can end a request sooner.
Explicit cancellation interrupts requests immediately rather than waiting for the
drain timer. Final request accounting includes canceled requests and body-read errors.

## Measurement definitions

Engine samples arrive approximately once per second, plus lifecycle transitions.
The UI displays these measured values without interpolation or simulated charts.

| Field | Definition |
| --- | --- |
| `sampled_at` | Engine sample timestamp |
| `load_elapsed_seconds` | Measured scheduling duration, capped at configured duration and frozen during drain |
| `elapsed_seconds` | Total execution time at the sample, including drain |
| `started_requests` | Requests accepted by workers for execution |
| `completed_requests` | Outcomes recorded by the collector, including failures/cancellations |
| `observed_start_rps` | Started requests / measured load duration |
| `recent_throughput` | Newly recorded outcomes / actual interval since the previous engine sample |
| `sample_interval_seconds` | That interval; 0 means no live rate is available |

A dropped progress sample does not change a later sample's denominator: it reports
the interval between engine samples, not between browser polls. Initial rates are
zero until work starts. Recent throughput is unavailable on final snapshots; use
`summary.throughput` there. It remains all recorded outcomes / total execution time.
The legacy `summary.attempted_rps` remains recorded outcomes / load window duration.

P50/P95/P99 use nearest rank over completed HTTP 2xx responses with fully read
bodies. Whole-run values are calculated once at finalization; interval values are
calculated separately for live charts. No successful samples means
`N/A` in the UI, including average latency. The progress bar measures the load
window; 100% while draining does not mean the run has terminated.

## Browser and network boundary

The API rejects non-loopback Host names, mismatched Origin (including another
localhost port), cross-site/same-site Fetch Metadata, and non-JSON POST requests.
No CORS permissions are emitted. Host validation also rejects DNS-rebinding names.
The Vite proxy must preserve Host and Origin (`changeOrigin: false`). Browser
protections follow the Origin/Fetch Metadata approach described in the official
[Go HTTP documentation](https://pkg.go.dev/net/http#CrossOriginProtection), with
an additional exact-origin comparison including the port.

CLI clients without browser headers are supported. These controls protect against
browser-driven requests; they are not authentication against other local processes.
HTTP targets must be loopback. The engine pins `localhost` to 127.0.0.1, checks
loopback addresses again at dial time, disables proxies, and never follows redirects.
For an IPv6-only target, use an explicit `[::1]` URL.

## Checks

```powershell
cd backend
go test ./...
$env:Path = "C:\msys64\ucrt64\bin;$env:Path"
$env:CGO_ENABLED = '1'
go test -race ./...
go vet ./...
```

```powershell
cd frontend
npm run lint
npm run build
```

Tests use `httptest` and cover lifecycle, simultaneous starts, slow-response and
draining cancellation, shutdown, retention, concurrent reads, preserved final
results, accounting, progress denominators, blocked progress consumers, redirects,
validation, and browser-origin protection.

## Limits

### Live charts (Phase 4)

The charts now use retained one-second interval statistics, including live
P50/P95/P99/Average, completed throughput, and split failures. The backend retains
up to 300 windows per run. Active-run recovery restores those measured windows;
missed polls do not fabricate or duplicate data. Final partial intervals are
included and chart history remains available until the run is evicted or the
backend restarts; saved chart history remains available in Test History. Whole-run summary cards and cumulative numeric progress remain
separate. See [the full interval contract and lab guide](performance-lab.md).

- At most one active run and 20 finished runs are retained. Evicted IDs return 404.
- Finalized reports persist in SQLite and remain available through Test History after restart. In-memory active-run endpoints do not restore old IDs; interrupted markers appear in history.
- Refresh recovers an active run; a completed run is accessible by its retained ID
  through the API but is not automatically restored by the dashboard after refresh.
- Polling and engine sampling introduce display delay; short states can be missed.
- Latency samples remain in memory for the bounded MVP. High-volume testing would
  require histograms. Scheduler counters represent observed ticks, not every ideal
  scheduling opportunity, and do not imply precise open-loop rate accuracy.
- No WebSockets, cloud access, accounts, or external-host testing.

## Phase 2 file inventory

Created:

- [Run configuration and validation](../backend/internal/runs/config.go)
- [Run manager](../backend/internal/runs/manager.go)
- [Manager tests](../backend/internal/runs/manager_test.go)
- [Concurrent-start and shutdown tests](../backend/internal/runs/concurrency_test.go)
- [Engine progress](../backend/internal/loadtest/progress.go)
- [Progress and redirect tests](../backend/internal/loadtest/progress_test.go)
- [API browser protections](../backend/internal/api/security.go)
- [API tests](../backend/internal/api/tests_test.go)
- [Polling and recovery hook](../frontend/src/hooks/useTestRun.js)
- [Live progress component](../frontend/src/components/dashboard/LiveProgress.jsx)
- This guide.

Modified:

- [Engine](../backend/internal/loadtest/engine.go): progress publication and loopback dialing.
- [Metrics](../backend/internal/loadtest/metrics.go): safe summary cloning.
- [API server](../backend/internal/api/server.go): manager ownership, routes, JSON helpers, security checks.
- [API test handlers](../backend/internal/api/tests.go): asynchronous start, status, active, stop.
- [Server entry point](../backend/cmd/storm-server/main.go): signal-driven cancellation and HTTP shutdown.
- [Dashboard controller](../frontend/src/components/TestForm.jsx): asynchronous states and final reports.
- [Configuration form](../frontend/src/components/dashboard/TestConfig.jsx): restored configuration and active-state labels.
- [API service](../frontend/src/services/testService.js): start/read/stop calls, errors, request timeouts.
- [Dashboard CSS](../frontend/src/App.css): progress and Stop styling.
- [Vite configuration](../frontend/vite.config.js): fixed local port and explicit Host preservation.
- [Project README](../README.md): link to setup and API documentation.

Existing Phase 1 dashboard files were preserved; they may still appear as
uncommitted changes from the preceding phase.
