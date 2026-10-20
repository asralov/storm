# Phase 8A — GitHub presentation and release preparation

**Historical artifact record:** documentation-encoding repairs subsequently repackaged the existing executable. Archive hashes below identify the earlier phase build, not the repackaged ZIP. For the current local archive, use `release/storm-v0.1.0-beta.1-windows-amd64.zip.sha256`. The executable and application behavior were unchanged by that documentation/notice repair.


Status: prepared locally; **not published, pushed, tagged or uploaded**.
Candidate: **v0.1.0-beta.1, Windows AMD64**. Existing MIT license retained.

## Materials

- Root README: positioning, real screenshots, features, launch/download status, build/dev setup, lab, privacy, local-only restrictions, tests and roadmap.
- [Architecture](architecture.md): component boundaries, Mermaid diagram, scheduling and rate/percentile semantics.
- [Contributing](../CONTRIBUTING.md), [security policy](../SECURITY.md), [bug/feature issue templates](../.github/ISSUE_TEMPLATE/), [changelog](../CHANGELOG.md).
- [Draft release notes](release-notes.md), [offline Windows guide](windows-usage.txt), and the unchecked [clean Windows 11 checklist](clean-windows-checklist.md).
- [Three real screenshots](images/README.md): standard dashboard and charts, stepped/breakpoint report, saved comparison. Each PNG is below 210 KB.

## Source and license review

The repository already selected MIT. It remains unchanged: no new license choice is required. MIT is a short permissive license; Apache-2.0 adds an explicit patent grant and more notice provisions. Changing the project's license would require a separate owner decision.

The Go placeholder module path was corrected to `github.com/asralov/storm` with matching imports. Personal absolute path examples were removed. The working-source scan found no remaining user-home paths or obvious private-key/GitHub-token/AWS-key patterns. A focused all-history key-pattern scan also returned zero matches. These are pattern checks, not a complete credential audit. The author's copyright in LICENSE is intentional attribution.

No tracked executable, SQLite database, node_modules, or generated dist directory was found. Ignore rules now cover release outputs, embedded bundles, local databases/sidecars, environment secrets and common temporary files. The generated release tree remains excluded. Existing startup/history-path logs are useful local diagnostics; no remote logging or telemetry was introduced. Runtime frontend dependencies remain React, React DOM and Recharts; no new dependencies were added. Existing Go SQLite and OS-lock dependencies remain necessary.

Full third-party texts are gathered automatically: 72 notice sections in this candidate, including conservative Go module notices and nested vendor licenses. The upstream Victory license missing from the npm root is pinned under [licenses](licenses/README.md). Maintainer review of this inventory remains a prepublication step.

## Versioning and packaging

[VERSION](../backend/internal/buildinfo/VERSION) is canonical. Go embeds it for CLI and health; the sidebar reads health. Build scripts read the same file for filenames and metadata. Documentation/release-note versions identify this specific candidate. The private frontend package version is not a product version.

Run from the root:

```powershell
node scripts/build.mjs
node scripts/smoke-windows.mjs
```

The build installs lockfile dependencies, builds fresh Vite assets, compiles with CGO=0, and packages locally. It refuses a conflicting STORM_VERSION override. Outputs:

- `release/windows-amd64/storm.exe`
- `release/storm-v0.1.0-beta.1-windows-amd64.zip`
- `release/storm-v0.1.0-beta.1-windows-amd64.zip.sha256`

ZIP contents: storm.exe, LICENSE, README.txt, RELEASE-NOTES.md, THIRD-PARTY-NOTICES.txt, build-metadata.json. Metadata records version, target, CGO, Go/Node versions, build time, commit and dirty state. **This preparation build records dirty=true** because prior phases and this phase remain uncommitted. It is not represented as reproducible from the old HEAD alone. No source commit was created automatically.

Candidate archive SHA-256:

`8ed4edcda4dd10240f1b3d532ee4835639e2442d864d16e889c64714f21552ec`

The ZIP checksum was independently checked, the six-file archive extracted outside the repository, and its extracted executable passed the smoke suite. A fresh build can change ZIP bytes/timestamps and metadata; always use the accompanying newly generated checksum. The executable hash in the Phase 7B audit describes that older audit candidate, not this updated packaging/version build.

## Validation

| Check | Result |
| --- | --- |
| Go unit/integration tests | PASS |
| Go race detector, Windows with MSYS2 compiler | PASS |
| go vet | PASS |
| Production-tag API/static/lifecycle tests | PASS |
| Frontend tests | 34 passed, 0 failed |
| ESLint | PASS |
| Frontend production build | PASS |
| CGO-disabled Windows AMD64 build | PASS |
| Full build and ZIP/notice/checksum pipeline | PASS |
| Existing executable smoke suite | PASS |
| Extracted-ZIP executable smoke suite | PASS |
| CLI/health/sidebar version agreement | 0.1.0-beta.1 |
| Clean Windows 11 VM | NOT RUN |
| macOS/Linux/other architecture runtime | NOT RUN |

Two preparation failures were corrected: Vite held a Windows native-module file during npm ci (stopped that project server and retried), and notice discovery initially missed nested Victory vendor licenses (added recursive collection and the pinned upstream license). The final full pipeline passed. npm reported zero vulnerabilities during lockfile install; this is not a complete security certification.

## Outstanding blockers and exact prepublication steps

1. Review all uncommitted source/doc changes from earlier phases and this phase. Ensure source, tests, VERSION, screenshots, pinned notice and templates are included; exclude release/dist/database/environment files. Run the final staged secret/diff review.
2. Complete [clean Windows checklist](clean-windows-checklist.md) on an actual clean Windows 11 AMD64 machine/VM. Record the artifact hash and every pass/fail/blocked outcome. Do not mark it passed based on stripped-PATH development-machine tests.
3. Review third-party notices and the unsigned-binary caveat. Signing/SmartScreen reputation is unverified. macOS/Linux verification remains outside this Windows beta; do not advertise verified binaries for them.
4. Verify the GitHub repository identity/visibility and enable private vulnerability reporting. SECURITY.md deliberately does not invent a contact email or claim that reporting is already enabled.
5. Obtain owner approval for the source commit/push and release publication. None of these operations was performed here. After the reviewed source is committed, rebuild from that exact clean commit, rerun automated checks and extracted-archive smoke tests, and record the new checksums/metadata.
6. Review the release notes, screenshot rendering, version/tag agreement and checksum. With explicit publishing approval, create tag `v0.1.0-beta.1` at that reviewed commit and a Windows prerelease; attach only the reviewed ZIP and its checksum, and use the draft release notes. Verify download/hash/launch after publication.

Recommendation: presentation and candidate artifacts are prepared; **public distribution still awaits owner approval and completion/disposition of the outstanding checks**. No GitHub mutation or binary upload occurred.

## Changed files

Added: CONTRIBUTING.md, SECURITY.md, CHANGELOG.md, two issue templates, architecture/release-notes/clean-Windows/release-preparation docs, offline usage text, three screenshot PNGs and provenance, pinned Victory license/provenance, shared Go buildinfo VERSION/helper, and package-release.mjs.

Updated: README.md, .gitignore, build.mjs, smoke-windows.mjs (optional extracted executable and exact version assertions), frontend/package.json (test command), persistent-history doc (portable path), Go module/import paths, production CLI and health version sourcing. Earlier phase changes were preserved. No product modes, analytics, load algorithms or dashboard layout were changed.
