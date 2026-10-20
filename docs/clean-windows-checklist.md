# Clean Windows 11 verification — NOT YET PERFORMED

Record tester, date, Windows edition/build, CPU architecture, browser version, artifact SHA-256 and result for every item. Use a fresh Windows 11 AMD64 VM or machine; do not infer results from development-machine tests. Leave boxes unchecked until tested.

## Preparation

- [ ] Confirm Go, Node.js/npm, Python, MSYS2/GCC and a SQLite service are not installed.
- [ ] Transfer the approved candidate ZIP and checksum through the intended download channel; verify `Get-FileHash` matches. Do not use an older cached binary.
- [ ] Extract into a normal user-owned path containing spaces. Keep notices alongside the executable.
- [ ] Record SmartScreen/Defender behavior and exact wording. The binary is unsigned; behavior depends on reputation and policy. Do not disable security software. If policy blocks it, record BLOCKED and follow the machine owner's approved process.

## Local target without developer runtimes

Use an authorized local API already installed, or a separate PowerShell window with this disposable listener. If Windows denies the listener reservation, record it and use an approved local target instead; do not silently elevate.

```powershell
$listener = [System.Net.HttpListener]::new()
$listener.Prefixes.Add('http://127.0.0.1:8081/')
$listener.Start()
try {
  while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    Start-Sleep -Milliseconds 20
    $bytes = [Text.Encoding]::UTF8.GetBytes('{"ok":true}')
    $ctx.Response.ContentType = 'application/json'
    $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
    $ctx.Response.Close()
  }
} finally { $listener.Close() }
```

This is only a functional test target; its serial handler does not establish performance capacity. Stop it with Ctrl+C after verification.

## Acceptance

- [ ] Double-click storm.exe as a normal user; default browser opens and health is connected.
- [ ] `storm.exe --version` reports 0.1.0-beta.1 and the dashboard agrees.
- [ ] Test `--no-browser`, explicit `--port 8090`, automatic `--port 0`, and an occupied port's clear error.
- [ ] Submit an authorized GET to `http://127.0.0.1:8081/fast`, 2 workers, 20 RPS, 10 seconds. Observe live charts and final actual results.
- [ ] Repeat the same standard test and compare the two saved reports; swap roles and return to history.
- [ ] Run stepped 10/20 RPS, 5 seconds/stage. Inspect stages and optional assessments; inconclusive is valid when evidence is insufficient.
- [ ] Start a 30-second test, press Stop, and confirm canceled rather than completed.
- [ ] Start again, press Ctrl+C in Storm's console, then reopen; verify cancellation/report status and earlier saved history.
- [ ] Close only the browser; verify this does not terminate Storm.
- [ ] Start a second instance on another port; verify history ownership is rejected without corruption.
- [ ] Reopen the saved charts and comparisons after restart.
- [ ] Verify SQLite is under the user's LOCALAPPDATA, not the extraction folder.
- [ ] Confirm non-loopback targets are rejected and no LAN listener is exposed.
- [ ] Record screen sizes, keyboard navigation, console errors and failure details.

## Decision

Tester/date: NOT RUN. Candidate hash: fill in from current checksum file.
Pass/fail/blocked summary: NOT RUN.
Owner approval to publish: NOT GRANTED by this checklist.
