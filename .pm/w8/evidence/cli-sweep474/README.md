# Static redeploy/restart + bad-branch failure — sweep474

Passing functional coverage, no new issue. Same isolated session as sweeps 454–473, `bex-canary`, HEAD checkout build (`4a5d27ff5`), 2026-10-18T07:40–07:44Z. Inline loop (no cron).

Static site: explicit `deploys create --wait` redeploys to `live`; `restart <static-id>` is accepted ("restarted successfully") rather than refused. Bad-branch native create (`--branch qa474-no-such-branch`) accepts the create, then fails the build in under a minute with the exact git diagnostic in build logs (`fatal: couldn't find remote ref qa474-no-such-branch`) and the same text as `failureReason` on the `deploy_ended` event; deploy reads `build_failed` with `finishedAt` set — no queue hang (the initial `queued` minutes were ordinary build-queue wait).

Cleanup: exact-owned `srv-dba7g233ar5s73d9ref0` and `srv-dba7g2eonb0s73deel00` deleted by ID (404-verified); baseline byte-identical. No plan mutations. Session remains active; next inline sweep is 475; m116 + evidence 465–474 remain uncommitted pending `/ship` (m115 shipped in 72a8d9ea4, being worked upstream).
