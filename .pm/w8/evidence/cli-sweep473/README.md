# Static build pipeline via CLI — sweep473

Passing functional coverage, no new issue. Same isolated session as sweeps 454–472, `bex-canary`, HEAD checkout build, 2026-10-18T07:37–07:42Z. The loop runs inline (no cron, per user instruction).

Static site with a real build step through the CLI (`--build-command 'mkdir -p out && cp README out/index.html' --publish-directory out` on `octocat/Hello-World`): build logs show the exact command in the build stage; deploy reaches `live`; `/` serves HTTP 200 with the built `index.html` body (`Hello World!`) — contrasting sweep457's no-build publish where `/` was an honest 404 — and a deep unknown path returns the SPA fallback 200. The build→publish→serve pipeline is CLI-complete.

Cleanup: exact-owned `srv-dba7dkj3ar5s73d9rdr0` deleted by ID (404-verified); baseline deltas are concurrent foreign churn, observed read-only. No plan mutations. Session remains active; next inline sweep is 474; m116 + evidence 465–473 remain uncommitted pending `/ship` (m115 shipped in 72a8d9ea4 and is being worked upstream).
