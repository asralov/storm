# Persistent local history (Phase 6A)

Storm now saves standard and stepped reports to a local SQLite database. Test
History opens saved reports and charts without executing traffic. Existing run
comparisons and breakpoint assessments are preserved; cross-run comparison is not
part of this phase.

## Driver and database location

The storage layer uses `database/sql` with **modernc.org/sqlite v1.60.1**, pinned
in the Go module along with its compatible dependencies. This is a pure-Go driver,
so the application builds with `CGO_ENABLED=0`; the Windows race detector still
uses the existing GCC setup. Its platform support and performance tradeoffs are
documented in the [driver reference](https://pkg.go.dev/modernc.org/sqlite@v1.60.1).
Indexed, bounded report queries and two lifecycle writes per job suit this workload.

Default locations (never relative to the repository or executable):

| Platform | Location |
| --- | --- |
| Windows | `%LOCALAPPDATA%\Storm\history.sqlite3` |
| macOS | `~/Library/Application Support/Storm/history.sqlite3` |
| Linux | `$XDG_DATA_HOME/storm/history.sqlite3` if the variable is absolute; otherwise `~/.local/share/storm/history.sqlite3` |

On this Windows setup the path is `%LOCALAPPDATA%\Storm\history.sqlite3`.
The server logs its database path at startup. SQLite may also create `-wal` and
`-shm` files beside the database. The application directory is private on Unix,
and the database uses mode 0600; Windows uses the user profile's inherited access
controls. The database is not encrypted. Current URLs, including query strings,
are saved exactly, so do not embed secrets in test URLs. Only current supported
configuration fields are selected for storage; future headers/tokens/request
bodies require an explicit redaction or secure-storage design before persistence.

## Storage architecture

[Storage](../backend/internal/storage/storage.go) is independent of the engine.
The manager accepts a small persistence interface and the application supplies the
SQLite implementation. The engine does not import storage and no scheduling or
per-request recording algorithm was changed.

- Schema version 1 is recorded with `PRAGMA user_version`. Initialization from
  version 0 runs in one transaction; unsupported newer schemas fail explicitly.
- Each report also has a snapshot version and a versioned JSON envelope. Unknown
  snapshot formats fail rather than being guessed or silently rewritten.
- `history_runs` stores indexed ID/status/mode/target/start metadata, a compact
  list overview, and the complete public report snapshot in the same transaction.
- The list index orders finalized records by start time descending, with ID as a
  deterministic tie breaker. Listing reads only overview data, not chart payloads.
- WAL journaling, synchronous FULL, a bounded busy timeout, and a one-connection
  pool serialize database access. Queries use parameters and bounded pagination.
- A snapshot is limited to 8 MiB; existing engine/stage interval bounds remain in
  effect. No individual HTTP request records are stored.

Saved snapshots include configuration, timestamps, terminal status, final summary,
intervals, every stepped-stage summary and interval history, and the original
optional analysis with its thresholds. Reading history does **not** rerun analysis
under possibly different code or reconstruct missing measurements.

## Lifecycle, errors, and recovery

Before traffic starts, the manager attempts to store a small start record. This
record is hidden from normal history listing. After all workers and stages finish,
the manager atomically saves the final report. Both operations occur outside the
manager mutex and outside the request-execution hot path, with five-second operation
timeouts. There is no unbounded background write queue.

Live responses add `persistence` with status `pending`, `saving`, `saved`, or
`failed`, and an error message when applicable. Execution metrics are terminal
before the final database write completes; polling continues while status is
`saving`. The active-job slot is held until that write attempt finishes. Stop
cannot change an already-finalized report. Normal shutdown joins the final write
before closing the database.

A failed start write does not prevent testing; final saving is still attempted.
A failed final write preserves the in-memory result and displays an explicit
warning. It is not retried indefinitely. A failed report remains subject to the
existing 20-finished-job memory retention and can be lost on restart. If opening
or initializing the database itself fails, the API fails startup with a clear log
message rather than silently presenting an empty history database.

Startup acquires the listening port and exclusive OS-held database lock before recovery. The database lock also prevents another instance on a different port from marking a live run as interrupted. Leftover start records
are finalized as `interrupted`, with no invented final metrics or completion time.
They retain their configuration and an explanation. Recovery processes bounded
batches and never resumes traffic. Since this phase does not checkpoint ongoing
metrics, abnormal shutdown cannot recover unsaved partial request results.

## API

| Request | Response |
| --- | --- |
| `GET /api/history?limit=20&offset=0` | `{items,total,limit,offset}`, newest first. Default limit 20, maximum 100; offset 0–1,000,000. |
| `GET /api/history/{id}` | Complete saved run, including stored stages, intervals, and analysis. |
| `DELETE /api/history/{id}` | `{"deleted":"run_..."}` after deletion. |

IDs must match `run_` followed by 32 lowercase hexadecimal digits. Invalid IDs,
unknown/duplicate pagination parameters, and out-of-range values return 400.
Missing/removed records return 404. Storage failures return 503 JSON errors and
are logged server-side. DELETE requires `Content-Type: application/json` and uses
the same Host/Origin protections as execution endpoints. No broad CORS is enabled.

```powershell
$base = 'http://127.0.0.1:8080'
$page = Invoke-RestMethod "$base/api/history?limit=20&offset=0"
$page.items
# Pick an actual saved ID from the list:
$id = $page.items[0].id
Invoke-RestMethod "$base/api/history/$id"
# Only when you intend to delete that report:
Invoke-RestMethod "$base/api/history/$id" -Method Delete -ContentType 'application/json' -Body '{}'
```

List rows include URL, method, workers, configured load, execution duration,
request count, completed rate, latency, error rate, status, and breakpoint status.
For stepped rows, requests/time are summed across executed stages, completed rate
is their ratio, and latency is explicitly **Max stage P95**, not an overall
percentile. Error rate is HTTP/network errors divided by successes plus those
errors, excluding cancellation. Missing measurements remain unavailable.

## History UI and restart verification

1. Start the existing **Storm: Start servers** VS Code task and the Vite frontend.
2. Run a standard test against `http://127.0.0.1:8081/fast`.
3. Wait for **Saved locally**, then open **View saved report** or **Test History**.
4. Run a stepped test with optional Breakpoint Discovery and save it too.
5. Stop the **Storm: API** task, then run it again. The lab can remain running.
6. Reload the browser and open Test History. Both reports should still exist with
   their metrics, chart windows, and original assessments.

Saved reports are read-only: there are no Run/Stop controls. A stage selector lets
you inspect each saved stage's timeline. Deletion asks for confirmation; canceling
the dialog preserves the report. Confirming removes only that saved run. A report
already displayed in the current dashboard can remain visible in memory, labeled
as deleted from history. Paging, loading, retry, missing-record, and empty states
are provided. Dashboard polling stays active when visiting history, but hidden
dashboard charts are unmounted to avoid zero-size rendering.

## Validation

From `backend`:

```powershell
go test ./...
$env:Path = "C:\msys64\ucrt64\bin;$env:Path"
$env:CGO_ENABLED = '1'
go test -race ./...
go vet ./...
$env:CGO_ENABLED = '0'
go build ./...
```

From `frontend`:

```powershell
npm run lint
npm run build
node --test src/hooks/timeline.test.js src/components/dashboard/stagePlan.test.js src/components/dashboard/assessmentRendering.test.js src/components/history/history.test.js
```

Backend tests use temporary directories and cover initialization/reopening,
standard/stepped/analysis/interval round trips, pagination, deletion, missing IDs,
concurrent access, unsupported versions, migration rollback, canceled/failed
records, interrupted recovery, read-only/closed storage, and nonblocking manager
access during final saving. Frontend tests cover loading, empty/error states,
read-only detail, deletion confirmation, API requests/failures, and persistence
warnings alongside the existing suite.

Browser verification saved standard and stepped reports, checked original analysis
and selectable stage history, restarted the real API, and verified identical SHA-256
hashes of both saved response bodies before and after restart. A canceled test was
also saved; dismissal of deletion kept it (200), confirmation removed it (404).

## Files and limitations

Added [storage](../backend/internal/storage/), [history API](../backend/internal/api/history.go),
[history API tests](../backend/internal/api/history_test.go), [persistence lifecycle tests](../backend/internal/runs/persistence_test.go),
[history page/tests](../frontend/src/components/history/), [shared report](../frontend/src/components/dashboard/RunReport.jsx),
[deletion action](../frontend/src/services/historyActions.js), Go dependency checksums, and this guide.

Updated the [manager](../backend/internal/runs/manager.go), [API server](../backend/internal/api/server.go),
[mutation protections](../backend/internal/api/security.go), [entry point](../backend/cmd/storm-server/main.go),
[Go module](../backend/go.mod), [app navigation](../frontend/src/App.jsx), [sidebar](../frontend/src/components/layout/Sidebar.jsx),
[header](../frontend/src/components/layout/Header.jsx), [dashboard](../frontend/src/components/TestForm.jsx),
[polling](../frontend/src/hooks/useTestRun.js), [services](../frontend/src/services/testService.js),
[styles](../frontend/src/App.css), and reused report components' accessible IDs/archived empty states.

- Existing in-memory reports from before this feature are not retroactively saved.
- History grows on disk until deleted; there is no automatic disk-retention policy,
  backup UI or export. Saved runs can be compared in Test History.
- Interrupted runs preserve identification/configuration, not unsaved final metrics.
- The database is local and unencrypted; current URLs are persisted verbatim.
- Windows runtime behavior was verified. macOS/Linux path selection is unit-tested;
  this environment did not run the application on those operating systems.
