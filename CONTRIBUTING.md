# Contributing to Storm

Thanks for helping improve a local-first performance tool. Discuss major behavior changes in an issue before implementation. Keep changes focused and explain the problem, final behavior and validation in your pull request.

## Development

Use the Go version in backend/go.mod and Node.js 22.12 or newer within the Node 22.x range with npm. Run `npm --prefix frontend ci`, `go -C backend run ./cmd/storm-server`, and `npm --prefix frontend run dev` in separate terminals. Optional lab: `go -C backend run ./cmd/storm-lab`. Node.js 22.17.0 was the development version reported as tested. Node.js is a source-development/build prerequisite, not a requirement for the standalone Windows executable.

## Required checks

Run `go -C backend test ./...`, `go -C backend test -race ./...` where supported, `go -C backend vet ./...`, `npm --prefix frontend test`, `npm --prefix frontend run lint`, and `npm --prefix frontend run build`. Race checking on Windows needs a supported compiler; the product build does not.

For packaging changes, stop Storm and Vite, run `node scripts/build.mjs`, then `node scripts/smoke-windows.mjs`. Use temporary databases for failure/restart tests. Do not delete another person's saved history.

## Review expectations

- Preserve loopback-only targets, server-side limits, origin checks and authorization confirmation.
- Preserve the accounting invariants in [architecture](docs/architecture.md). Add focused regression tests for defects; use local httptest targets, not external services.
- Do not fabricate chart samples, reinterpret old reports silently, or claim capacity from generator saturation.
- Keep the engine independent of HTTP handlers, persistence and UI.
- Never commit databases, binaries, node_modules, credentials, tokens, or private benchmark URLs. Review `git diff --cached` before committing.
- Update relevant docs and CHANGELOG. Application version lives in backend/internal/buildinfo/VERSION; generated metadata is not edited by hand.

Contributions use the repository's existing MIT license. For vulnerabilities, follow [SECURITY](SECURITY.md), not a public bug report. Maintainers must explicitly approve release publication; build scripts only produce local artifacts.
