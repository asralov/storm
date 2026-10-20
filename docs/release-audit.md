# Windows public beta release audit — Phase 7B

Audit date: 2026-10-17. Recommendation: **Ready with caveats** for a local-only Windows AMD64 beta, subject to owner approval to distribute. No release was published or uploaded.

## Scope and evidence

Reviewed the load engine, scheduler, metrics/interval collector, run/stage manager,
assessment logic, REST/security handlers, SQLite lifecycle, embedded static handler,
launchers/build script, React polling/history/comparison/rendering, tests, and docs.
This is a source review plus tests on this Windows development machine, not a formal
penetration test, statistical certification, or clean-OS installation certification.
No new traffic algorithms, remote target support, or product modes were added.

## Findings by severity

### Critical

No confirmed Critical issue found in the inspected scope.

### High

No confirmed High issue found in the inspected scope. Loopback-only dialing,
redirect refusal, single-active-job admission, cancellation/joining, and database
ownership remained intact under the exercised tests. This is not a claim that no
undiscovered defect exists.

### Medium — fixed

1. **Client timeouts were counted as cancellation.** Request deadline errors with a
   live run context were excluded from the HTTP/transport failure denominator and
   made breakpoint assessments inconclusive. The engine now normalizes run-context
   termination to cancellation; independent client/header/body timeouts count as
   transport errors. Fully read responses alone can contribute success latency.
   Existing saved reports retain their original classification; rerun old workloads
   to obtain corrected measurements. There is no retrospective data migration.
2. **Mixed-case localhost was accepted but failed at dial time.** Validation accepted
   `LOCALHOST`, whereas the connection guard only pinned lowercase `localhost`.
   Case-insensitive matching now pins all accepted variants directly to 127.0.0.1;
   no arbitrary DNS resolution was introduced.

### Low — fixed

- Several earlier-phase docs claimed that history disappeared on restart or that
  saved stage browsing/comparison was unavailable. Updated the current behavior,
  database lock description, timeout semantics, and frontend setup documentation.
- An old test called normal client timeout “cancellation” and raced a two-second
  server response against the two-second client timeout. It now tests a deliberately
  slower response and expects transport errors; explicit cancellation has separate
  coverage.
- Added a reusable executable smoke script, plus malformed-body, cross-origin
  mutation, timeout/body-read, hostname/dial-boundary, and React escaping regressions.

### Low — remaining

- The executable version includes beta metadata, while the dashboard and health
  endpoint still show the base 0.1.0 version. Use `--version` and the binary hash to
  identify this candidate.
- Unsigned executable; no installer, update mechanism, or tray lifecycle.
- A failed rebuild can leave a previous executable in the output directory. Treat
  a successful build exit and candidate hash as required release evidence.

## Exact measurement contract

- Target RPS is a requested global request-start schedule, not a throughput promise.
- Scheduled opportunities count observed ticker events. Go may coalesce delayed
  ticks; this is not a count of every ideal wall-clock opportunity.
- Dispatched opportunities are accepted worker handoffs. Missed opportunities have
  no available worker. `scheduled = dispatched + missed`.
- Started requests pass the worker's cancellation check and enter execution.
  Cancellation between dispatch and start means `started <= dispatched`.
- After workers and results are drained: `started = requests = successes + failures`.
- `failures = http_error_count + network_errors + canceled`.
- HTTP errors are fully read non-2xx responses, including unfollowed redirects.
  Connection refusal, truncated bodies and client timeouts are transport errors.
  Explicit run termination is cancellation. A broken non-2xx body is classified as
  transport failure, not counted twice as an HTTP error.
- Completed throughput is all recorded outcomes / total engine time, including drain
  and cancellation outcomes. It is not successful-response goodput.
- Observed request-start rate is started / measured active load seconds. The load
  clock is capped at configured duration and frozen during drain. Legacy
  `attempted_rps` uses final recorded outcomes / active load seconds, numerically
  equivalent when the final started/outcome invariant holds.
- Live numeric throughput uses outcomes recorded since the preceding engine sample
  divided by that actual sample duration. Chart throughput uses outcomes assigned by
  worker completion timestamp / measurement-window duration. Collector delay can
  temporarily make these differ. Delayed results correct retained windows.
- Intervals cover [start, end), approximately one second plus a final partial window.
  Interval totals match final outcomes while all windows are retained (300 maximum).
  A very short last window can legitimately show a noisy rate.
- Latencies include full response-body transfer for HTTP 2xx successes only.
  Nearest rank is sorted sample at `ceil(p*n)-1`; no interpolation or histogram.
  Whole-run percentiles are not averages of interval percentiles. Missing interval
  latency is null; legacy summary zeros with zero successes render as N/A.
- Stepped stages have separate collectors and complete worker cleanup before the
  next stage. No overlapping load windows; connection warmup repeats per stage.
- Breakpoint error denominator is successes + HTTP errors + transport errors;
  cancellations are excluded and make a stage inconclusive. At least 100 relevant
  outcomes and 100 successful latency samples are practical gates, not confidence
  intervals. Inadequate offered load never proves target overload. Partial,
  nonmonotonic or inconclusive sequences do not get an invented bounded crossing.
- Comparison HTTP error rate is HTTP errors / (successes + HTTP errors); it excludes
  transport errors and differs deliberately from the history/assessment rate.

## Security and reliability review

- Production listens on IPv4 loopback only. Exact listener Host or localhost with
  the same port is required; remote/rebinding names are rejected.
- API Origin must match Host including port; cross-site/same-site Fetch Metadata is
  rejected. JSON is required for POST/DELETE; no broad CORS is emitted. Missing
  browser headers remain allowed for native local clients: this is not authentication.
- Targets are HTTP GET, authorized, loopback literals or pinned localhost. Dialing
  rechecks the destination, proxy use is disabled, and redirects are never followed.
  Workers, rates, durations and stage count/total-duration limits remain server-side.
- Start payloads are bounded to 4 KiB, unknown fields and trailing JSON rejected.
  Header/read/write/idle timeouts bound slow HTTP handling. Error bodies remain JSON
  for the exercised API cases; unknown API routes never receive SPA HTML.
- Static files come only from the embedded FS; invalid paths and missing assets are
  rejected. React renders target text through escaped JSX, not injected HTML.
- SQL is parameterized; writes are transactional, WAL/FULL, with bounded wait times.
  Database ownership is guarded by an OS lock independently of listening port.
  Windows relies on inherited user-profile ACLs; Unix file/directory modes exist but
  Unix runtime permissions were not exercised here.
- Manager admission is mutex-protected, progress queues are bounded and nonblocking,
  final outcomes are drained, and shutdown joins work before closing storage.
  Storage errors preserve the in-memory report and surface a persistence warning.
- Crash recovery marks start markers interrupted, never resumes load, and does not
  invent unsaved metrics. Marker/final-save failures can still lose durable results.

## Automated validation executed

| Check | Result |
| --- | --- |
| `go test ./...` | PASS after correcting the legacy timeout expectation |
| `go test -race ./...` with MSYS2 compiler | PASS |
| `go vet ./...` | PASS |
| `go test -tags=production ./internal/app ./internal/webui` | PASS |
| Frontend Node test suite | 34 passed, 0 failed |
| `npm run lint` | PASS |
| Lockfile install and frontend production build | PASS; npm reported 0 vulnerabilities at install time |
| `node scripts/build.mjs` | PASS, Windows AMD64, CGO=0 |
| `node scripts/smoke-windows.mjs` | PASS |

Existing suites exercise simultaneous starts, repeated Stop/Start and retention,
active/draining cancellation, shutdown, concurrency, slow consumers, HTTP failures,
redirect protection, scheduler saturation, interval boundaries, percentiles,
assessment gates, SQLite failures/versioning/reopen, and immutable comparisons.
New tests add timeout/body-read distinctions, truncated 200 bodies, hostname casing,
non-loopback dial rejection, malformed/oversized configurations, destructive-operation
Origin rejection, and escaped hostile text in saved report rendering.

The first smoke attempt failed because Node Fetch did not send the overridden Host;
the test now uses raw HTTP for this check. The corrected Host test returns 403.

## Windows acceptance checks

Fresh binary copied outside the repository to a temporary directory. Child PATH
contained no developer tools; no Vite or external database installation was used.
SQLite was isolated using a temporary LOCALAPPDATA, preserving the user's history.

- `--version`, `--no-browser`, automatic port selection and explicit port 8092 worked.
- Standard and two-stage stepped runs completed with exact outcome/interval accounting.
- Concurrent start returned 409; Stop produced a saved canceled report.
- Stepped breakpoint analysis was present (`not_reached`, all tested stages passing).
- Restart preserved the standard report exactly. Forced termination during a later
  active run produced an interrupted history record and no resumed active job.
- Browser opened the embedded dashboard, rendered 9 live chart paths while a standard
  run was active, displayed final charts, saved stage charts and breakpoint results.
- Browser Stop reached canceled. Saved comparison loaded and correctly warned that
  the two chosen configurations differed (inspection only).
- Ctrl+C during an active run saved `run_9c47c5fcfda419212472d68ed4195712` as canceled;
  it remained saved after reopening. These IDs belong only to the temporary audit DB.
- Port conflicts and duplicate-database ownership are covered by application tests;
  the previous Phase 7A executable acceptance also exercised both manually.
- Default-browser auto-opening was exercised in Phase 7A; this audit uses
  `--no-browser` and the integrated browser. No new default-browser matrix was run.

Smoke database: temporary `storm-release-smoke-JcPFKx` directory. It is intentionally
retained for inspection. The test target used by the smoke script is stopped afterward.

## Candidate and release decision

Candidate: `release/windows-amd64/storm.exe`, built with CGO disabled.
SHA-256: `A82366CB981C06BA379D8B8135816E87A6D87637E7005850693F538BABC6AAD3`.

**Ready with caveats** for a local Windows AMD64 beta. No confirmed unresolved
Critical/High defect remains in this audit. Publishing still requires explicit owner
approval. Do not distribute an older binary or one from a failed build.

Remaining release work/limitations:

- No clean Windows VM without any development software installed was available;
  outside-repository execution with stripped PATH passed, but is not the same test.
- Signing/SmartScreen reputation and installer behavior are not verified.
- macOS, Linux, Windows ARM64 and other architectures were not runtime-tested.
- No dependency-wide Go vulnerability scan or third-party license inventory was
  performed; npm's install audit is not a comprehensive security assessment.
- Local native processes/admins can call or modify this unauthenticated local app.
  Do not expose it through a LAN listener or external reverse proxy.
- SQLite is unencrypted; full target URLs, including query strings, are saved. Do
  not put tokens/secrets in test URLs. History grows until manually deleted.
- Prior reports have no per-report producer/measurement version; old timeout counts
  cannot be reliably reclassified. New runs are recommended for comparisons.
- Fixed two-second request timeout, successful-only latency, short workloads,
  host scheduling and generator saturation limit interpretation. No capacity guarantee.
- Forced termination/power loss can lose the final report. Recovery reports the
  interruption rather than reconstructing measurements that were never persisted.

## Files changed for Phase 7B

- [Engine](../backend/internal/loadtest/engine.go), [metrics](../backend/internal/loadtest/metrics.go),
  [engine tests](../backend/internal/loadtest/engine_test.go), [metrics tests](../backend/internal/loadtest/metrics_test.go),
  [new release regressions](../backend/internal/loadtest/release_test.go).
- [Assessment explanation](../backend/internal/assessment/assessment.go) and
  [assessment tests](../backend/internal/assessment/assessment_test.go).
- [API release regressions](../backend/internal/api/release_test.go),
  [React history regression](../frontend/src/components/history/history.test.js),
  [Windows smoke script](../scripts/smoke-windows.mjs).
- [README](../README.md), [frontend README](../frontend/README.md),
  [async guide](async-tests.md), [lab guide](performance-lab.md),
  [stepped guide](stepped-tests.md), [assessment guide](breakpoint-discovery.md),
  [history guide](persistent-history.md), [comparison guide](run-comparisons.md), this report.

Existing uncommitted changes from earlier phases were preserved. No release upload,
new dependency, database migration, UI redesign, or scheduling algorithm was introduced.
