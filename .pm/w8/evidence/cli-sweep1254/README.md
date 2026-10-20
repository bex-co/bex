# CLI sweep 1254 — per-request ID for tenant traffic (read-only research, 2026-10-20)

No new fixtures; this reuses the live captures from sweep 1241 (headers an echo-server app received, response headers) and sweep 1228 (request-log entry labels). **Finding → [w8/148](../../148.md):** Render's documented `Rndr-Id` request/response header and the matching `requestID` in HTTP request logs have no Bex equivalent. Neither the app nor the client sees a request ID, and request-log entries carry none.

- Also checked: the pinned CLI's interactive `deploys create` waits for a `queued` deploy only when the create response has no body (`pkg/tui/views/deploycreate.go:40–52`, `WaitForDeployCreate`). Bex always answers 201 with the deploy, so that path is not reached.
- No new production pin: still `94533d795067`, so the source fixes for w8/140–142, w8/m122 and w8/146 cannot be re-verified live yet.
