# CLI sweep 1243 — OAuth refresh through the real CLI, single and concurrent (live, 2026-10-20)

The QA identity's device-login grant and the isolated CLI config were used. Only token fingerprints (SHA-256 prefixes) were compared; no token values were recorded. No hosting resources were touched. No new bug filed.

- The pinned client refreshes when `expires_at` is within 24 h (`pkg/client/client.go:56–80`). A private config copy with `expires_at` = now+1 h ran `bex whoami` → exit 0 in 1.7 s. The copy now holds a new access token, a new refresh token and a 168 h expiry (Bex's 7-day TTL).
- The pre-refresh access token is revoked on rotation. The untouched original config then gets `Error: your token is expired; run \`render login\` …` (exit 1; the `render login` wording is the documented upstream residual). The refreshed config was promoted to be the loop's config.
- **Concurrent refresh** (three `bex whoami` processes started together from copies sharing one refresh token): all exit 0. Two received byte-identical new tokens (the `cliauth` idempotency store); the third got its own pair inside Hydra's 60 s rotation grace. Both families still worked 70 s later. Parallel CLI commands inside the refresh window do not log a user out.
- Cleanup: the extra local copies were deleted. Their server-side tokens were not revoked, because revoking a sibling could cut the shared chain the loop depends on, so they lapse at their 7-day expiry.
