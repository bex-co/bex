# CLI sweep 505 — resume returns a suspended service to serving

**Verdict: no bug filed.** Closes the one unverified leg left by sweep 503 (its fixture was deleted 10 s after resume, while the host still read `503`). Same isolated session, `bex-canary`, HEAD build `37dffc290`, 2026-10-19T09:04–09:05Z.

| step | observed result |
| --- | --- |
| Create free image web service, wait `live` | Public URL `200`. |
| Suspend (REST, same QA credential) | `202`; URL `503` (suspended page) within 8 s. |
| Resume (REST) | `202`; URL polls `503` at +2/+13/+24/+34 s, then `200` at **+45 s** — the rollout, not a stuck resume. Sweep 503's +10 s `503` was this same window. |
| Delete by ID | Former URL `404`. |

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbatq21kv6rc73en2cug` deleted by ID; former-URL 404 verified. Suspend/resume have no pinned-CLI verb, so they ran over REST as setup/diagnosis, not CLI journey coverage (CLI suspend/resume round trip over REST was sweep 500).
- Unrun: resume of a hibernated (auto-slept) free service vs an explicitly suspended one; resume while a deploy is in flight.
- Deploy lag means deployed revisions can precede local `main`.
