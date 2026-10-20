# Changelog

## v0.1.0-beta.1 — Prepared, not published

Initial Windows AMD64 beta candidate:

- Standard and stepped HTTP load tests with a global RPS scheduler.
- Asynchronous Start/Stop, graceful draining, interval charts and final percentiles.
- Optional conservative Breakpoint Discovery assessments.
- SQLite history, saved report charts and baseline/candidate comparisons.
- Standalone executable with embedded React dashboard and pure-Go SQLite.
- Loopback-only execution, browser-origin protections and server-side limits.
- Separate optional Storm Lab workloads.
- Release-audit fixes for timeout classification and case-insensitive localhost.
- Shared version metadata, release archive/checksums, dependency notices and open-source guidance.

Known caveats: unsigned Windows binary; clean Windows 11 VM verification outstanding; macOS/Linux unverified. No remote-host tests, cloud features, telemetry, or automatic updates.
