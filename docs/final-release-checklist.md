# Final release candidate checklist — Phase 8B

**Historical artifact record:** documentation-encoding repairs subsequently repackaged the existing executable. Archive hashes below identify the earlier phase build, not the repackaged ZIP. For the current local archive, use `release/storm-v0.1.0-beta.1-windows-amd64.zip.sha256`. The executable and application behavior were unchanged by that documentation/notice repair.


Verified on Windows AMD64, 2026-10-19 local time (build timestamp 2026-10-20T05:18:32.852Z).
Version: **v0.1.0-beta.1**. **No commit, push, tag, upload or GitHub release was performed.**

## Decision

**Local candidate verification passed. Public release: HOLD pending source review/clean-commit rebuild and clean Windows acceptance.**

No new confirmed Critical/High runtime defect was found in this focused verification. This does not make the current dirty working tree a reviewed, reproducible release commit. Phase 7B's “Ready with caveats” engineering assessment remains distinct from distribution approval.

## Exact artifacts

Paths relative to the repository root:

- ZIP: `release/storm-v0.1.0-beta.1-windows-amd64.zip`
- Checksum: `release/storm-v0.1.0-beta.1-windows-amd64.zip.sha256`
- Executable: `release/windows-amd64/storm.exe`
- Build provenance: `release/windows-amd64/build-metadata.json`
- Notices: `release/windows-amd64/THIRD-PARTY-NOTICES.txt`

ZIP SHA-256:

```text
bb0e511b61bef7225bacb15ddb7be9614a6d458ed589e2c9793e61231622085f
```

Executable SHA-256:

```text
f892987ac88aed0a7fed9e3d03fa2fa625730cf94a33a5e87f0d438b1b879c59
```

These identify this Phase 8B rebuild and supersede earlier-phase candidate archive hashes. A later rebuild requires its own checksum and acceptance evidence.

## Verified requirements

- [x] Repository inspected: **71 modified/untracked status entries at audit start**. Many contain essential changes from earlier phases, including storage, embedded serving, release scripts, version metadata, tests and docs. Nothing was automatically staged or committed.
- [x] HEAD is `d8162288825486d98663f9f25a7fa83f5ac4fc94`. Build metadata and the executable both report a modified/dirty source tree.
- [x] `frontend/package-lock.json` is tracked and identical to committed HEAD. `npm ci` installed the lockfile without updating it. Go module checksums exist locally but remain untracked and must be included in source review.
- [x] Existing MIT license is tracked, unchanged from HEAD, and byte-identical in the release directory. It was not replaced with another license.
- [x] No tracked executable, release ZIP, SQLite database, node_modules or generated dist files found. Ignore checks cover release files, embedded assets, dependencies, `.env`, databases and WAL sidecars. The root .gitignore itself is still untracked and must be committed after review.
- [x] Focused working-source checks found no hard-coded user-home paths or obvious private-key/GitHub-token/AWS-key patterns. The same key-pattern check over Git history found zero matches. This is not an exhaustive secret-scanning certification. LICENSE author attribution is intentional.
- [x] `git diff --check` passed. Documentation correctly describes local-only HTTP GET, unsigned Windows packaging, unverified clean-VM/non-Windows status and approval requirements. Older audit hashes are historical, not the current candidate checksum.
- [x] Canonical VERSION, CLI output and health response agree on `0.1.0-beta.1`; the sidebar consumes health version. The private frontend package version is not the product release version.
- [x] Frontend freshly built from source. All **10 files** in `frontend/dist` and `backend/internal/webui/dist` have identical names and SHA-256 hashes; production-tag tests also verified embedded serving.
- [x] Executable build information independently confirms `GOOS=windows`, `GOARCH=amd64`, `CGO_ENABLED=0`, `production` tag and trimpath.
- [x] Archive independently enumerated: exactly `storm.exe`, `LICENSE`, `README.txt`, `RELEASE-NOTES.md`, `THIRD-PARTY-NOTICES.txt`, `build-metadata.json`. No database, source tree, private path, or developer dependency directory is packaged.
- [x] ZIP checksum matches its checksum file. Extracted executable hash matches the release executable.
- [x] Full-text dependency notices include installed production npm/vendor license files, Go module/runtime/vendor notices, and the pinned upstream Victory license. All compiled Go dependency names/versions are represented; Victory's pinned version matches the lockfile. No user-home paths found in notices. Presence/inventory is verified, not a legal compliance opinion.
- [x] Extracted ZIP executable passed the existing standalone smoke script outside the repository, with isolated temporary SQLite and no developer tools in the child PATH. Standard/stepped tests, Start conflict, cancellation, chart intervals/accounting, breakpoint analysis, restart persistence, forced-exit recovery, origin/Host protection and version assertions passed.

## Tests executed and exact build commands

From the repository root:

```powershell
go -C backend test ./...
go -C backend vet ./...
# This machine has the supported compiler here; substitute your compiler PATH for race testing.
$env:Path = "C:\msys64\ucrt64\bin;$env:Path"
$env:CGO_ENABLED = '1'
go -C backend test -count=1 -race ./...
node scripts/build.mjs
go -C backend test -count=1 -tags=production ./...
npm --prefix frontend test
npm --prefix frontend run lint
```

The build script overrides CGO to 0 for the distributed executable and runs `npm ci` and the frontend production build. Existing Storm was confirmed idle and stopped before replacing its executable.

All commands passed: Go unit/integration tests, uncached race tests, vet, uncached production-tag Go tests, frontend **34/34 tests**, ESLint, frontend production build and the Windows archive pipeline. npm reported zero vulnerabilities during installation; no new dependency-wide Go vulnerability scan was performed.

Extracted artifact check (after hash verification below):

```powershell
$destination = Join-Path $env:TEMP ('storm-rc-' + [guid]::NewGuid().ToString('N'))
Expand-Archive -LiteralPath .\release\storm-v0.1.0-beta.1-windows-amd64.zip -DestinationPath $destination
node scripts/smoke-windows.mjs (Join-Path $destination 'storm.exe')
```

**PASS**, using the actual extracted executable. Test history was isolated; the user's database was preserved. This development-machine test is not a clean Windows VM test. Tests were not reimplemented with mock UI data.

The documented build workflow succeeds from the current working tree using the committed frontend lockfile. **Building from a reviewed clean source commit has not been demonstrated. Byte-for-byte reproducibility across machines/environments has not been tested or claimed.** ZIP timestamps and metadata include build time.

## Security and distribution review

| Area | Verified behavior / remaining boundary |
| --- | --- |
| Listener | Explicit IPv4 loopback binding; production accepts its actual Host or localhost with the same port |
| Browser control | Exact Origin/port and Fetch Metadata checks; JSON mutations; no permissive CORS |
| Targets | HTTP GET, loopback checked at validation and dial time, localhost pinned, proxies disabled, redirects not followed |
| Authorization/limits | Confirmation required server-side; 1–10 workers, 1–100 target RPS, 1–30 seconds/stage, at most 10 stages and 120 seconds stepped load |
| Storage | `%LOCALAPPDATA%\Storm\history.sqlite3`; transactional writes, OS-held ownership lock; user data outside archive/repository |
| Signing | PowerShell Authenticode result: **NotSigned**. Security/reputation warnings can vary by Windows policy |
| Reporting | SECURITY.md conditionally directs users to GitHub private reporting and offers a no-details contact request fallback; actual repository setting was not verified or changed |
| Notices | Existing MIT unchanged; full dependency texts present; maintainer review required before distribution |

No security check was weakened. Local native processes remain within the trust boundary. Full URLs, including query strings, are saved unencrypted. Do not put secrets in URLs, expose Storm through a LAN/public proxy, or disable Windows security protections to get a passing installation result.

## Release blockers and unverified requirements

- [ ] Owner reviews the numerous uncommitted/untracked files, including `.gitignore`, `go.sum`, scripts, tests, screenshots and license notices. HEAD alone cannot produce this feature set.
- [ ] After explicit authorization, record the reviewed source in a commit. Rebuild from that clean commit and repeat tests/hash/extracted-artifact checks. No automatic commit is permitted by this milestone.
- [ ] Complete real **clean Windows 11 AMD64** acceptance without Go, Node.js, Python, GCC or MSYS2. Record the artifact hash, OS/browser versions and all results. No such environment was available here.
- [ ] Verify a usable private security-reporting channel and review third-party notice obligations. GitHub settings were not inspected or changed.
- [ ] Obtain explicit approval before any push, tag, upload or release creation.

## Accepted, documented limitations (not claims of verification)

Unsigned executable; no installer/updater/tray application. SmartScreen/Defender behavior and signing reputation are not certified. macOS/Linux and other architectures are unverified and are not release targets here. No remote-host testing, HTTPS targets, accounts or telemetry.

Measurement limits remain: two-second request timeout, successful-response-only latency, practical rather than statistical sample gates, generator saturation and shared-machine variability. Old reports retain historical semantics. Forced termination can lose unsaved metrics; interrupted recovery does not invent them. Local history is unencrypted and grows until deleted.

## Installation and checksum verification

For the approved candidate, place ZIP and checksum together in a normal user-owned folder. Do not run as administrator merely to suppress warnings.

```powershell
$archive = '.\storm-v0.1.0-beta.1-windows-amd64.zip'
$expected = ((Get-Content -LiteralPath ($archive + '.sha256') -Raw).Trim() -split '\s+')[0]
$actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ($expected -notmatch '^[0-9a-fA-F]{64}$' -or $actual -ne $expected.ToLowerInvariant()) {
    throw 'Checksum mismatch: do not extract or run this archive.'
}
Expand-Archive -LiteralPath $archive -DestinationPath '.\Storm Beta'
Set-Location '.\Storm Beta'
.\storm.exe --version
.\storm.exe
```

Expected version: `Storm 0.1.0-beta.1`. Default dashboard: `http://127.0.0.1:8080`. Keep all notices with the extracted executable. A checksum detects changes but does not authenticate an untrusted source. If security policy blocks execution, record the block and consult the machine owner; do not disable safeguards.

Alternative launch: `.\storm.exe --port 8090 --no-browser`, then open `http://127.0.0.1:8090`. Stop cleanly with Ctrl+C in the console. Closing a browser tab does not stop Storm.

## Clean Windows acceptance — ALL UNCHECKED

Use the self-contained PowerShell/.NET local listener in [clean Windows checklist](clean-windows-checklist.md), or an explicitly authorized local API. Neither option requires Python or developer tools. The example listener is serial and only a functional target; if Windows denies its reservation, record that result and use an approved target, not silent elevation. Run only one target on port 8081.

1. [ ] Verify SHA-256 and extract the ZIP into a user-owned folder with spaces.
2. [ ] Launch storm.exe without developer runtimes installed; record unsigned warnings.
3. [ ] Confirm the dashboard opens in the browser.
4. [ ] Confirm Backend connected and beta version.
5. [ ] Confirm explicit authorization and run a standard GET against the local target (2 workers, 20 RPS, 10 seconds).
6. [ ] Run stepped 10/20 RPS, 5 seconds/stage; inspect actual stage results.
7. [ ] Observe live throughput/latency/failure charts, then final charts.
8. [ ] Stop a 30-second test and verify canceled status.
9. [ ] Wait for Saved locally and reopen reports read-only from Test History.
10. [ ] Compare two saved standard runs with matching settings; swap roles.
11. [ ] Restart Storm after cleanly closing it.
12. [ ] Verify earlier history, charts and comparisons persist in the user's LOCALAPPDATA.
13. [ ] Verify `--port 8090`, `--no-browser`, `--port 0`, occupied-port errors and duplicate-instance rejection.
14. [ ] Press Ctrl+C during a test, reopen, and verify graceful cancellation and saved final state; stop the disposable target too.

Tester, OS build, browser, hash and evidence: **NOT RUN**. A fresh clean-commit artifact requires fresh evidence; do not reuse checks from a different ZIP.

## Required final review / changes made

Review source and notices, finish clean-machine testing, establish the reporting channel, then request distribution approval. Only after that approval should a reviewed commit/tag and Windows prerelease be published with the matching ZIP and checksum. No publication commands were executed here.

Phase 8B changed this checklist and added pointers from README and the historical Phase 8A preparation report. Generated binaries/ZIP/checksum were rebuilt locally. No product source, visual design, dependencies, algorithms, security restrictions or platform support changed. Earlier-phase working-tree changes were preserved.
