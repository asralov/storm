# v0.1.0-beta.1 release notes (draft — not published)

Storm — Find your API's limits before your users do.

A local-first HTTP performance tool with an embedded dashboard, standard/stepped tests, live latency percentiles, optional threshold assessments, SQLite history and report comparisons.

## Windows AMD64

Download the approved `storm-v0.1.0-beta.1-windows-amd64.zip` and checksum, extract, and run `storm.exe`. No Go, Node.js, MSYS2/GCC or SQLite installation is needed. Browser URL defaults to http://127.0.0.1:8080. Flags: `--port`, `--no-browser`, `--version`. Stop with Ctrl+C.

Use only authorized HTTP GET targets on loopback. Storm Lab is optional and separately built from source. This archive contains Storm, license, dependency notices, offline usage instructions, these notes and build metadata.

## Caveats

Unsigned binary; Windows may show reputation/security warnings. Clean Windows 11 VM testing has not yet been performed. macOS/Linux and non-AMD64 binaries are not verified. Results are observations, not guaranteed capacity; successful-response latency must be considered alongside failures and generator shortfalls. Reports created before timeout-classification fixes retain their original semantics.

Data stays in `%LOCALAPPDATA%\Storm\history.sqlite3` and is not deleted when the executable is replaced. URLs are stored unencrypted: omit secrets. Forced termination can lose the final report.

## Before publishing

Complete the clean Windows checklist, verify the exact archive checksum, review dependency notices, enable private security reporting, review source changes, and obtain explicit owner approval. Do not describe pending checks as passed.
