# Security policy

Storm is a local network-testing tool. Use it only against systems you own or have explicit authorization to test. Authorization is still required for localhost targets.

## Boundary

Storm supports HTTP GET to loopback only, binds its API to loopback, validates browser Host/Origin/Fetch Metadata, requires JSON mutations, and disables redirects and outbound proxies. Do not remove these safeguards or expose the API through a LAN/public proxy. These checks do not authenticate local native processes or protect against a compromised account, administrator, or browser extension.

Reports live in unencrypted local SQLite. Full URLs are saved, including query strings: never put secrets in URLs. No telemetry or cloud account is included. A forced shutdown can lose unsaved metrics. The Windows beta is unsigned; obtain artifacts from the approved repository release and verify the checksum. A checksum does not authenticate an untrusted download source.

## Reporting

**Maintainer verification required:** private vulnerability reporting has not been verified from this environment. The unauthenticated repository and reporting-status requests returned 404; that does not establish whether reporting is enabled. Before publication, a maintainer must verify the repository visibility and a usable private reporting channel.

Do not publish exploit details, credentials or private report data in public issues. Use GitHub's **Report a vulnerability** action in this repository's Security tab if available. Private vulnerability reporting must be enabled by the maintainer before public release. If it is unavailable, open only a minimal issue requesting a private contact channel, without vulnerability details. No unverified email address or guaranteed response SLA is advertised.

Include affected version/hash, OS, minimal local reproduction, expected/actual behavior and impact. Redact tokens, personal paths and sensitive target data. Avoid probing third-party systems while reproducing a problem.

## Support status

The initial v0.1.0-beta.1 candidate is under review. There are no promises of long-term support for pre-beta snapshots. Maintainers will triage reproducible security reports for the latest beta. Clean Windows 11 verification and macOS/Linux runtime validation remain outstanding; see the release checklist.
