# CLI sweep 595 — w8/132 re-verification on the new production pin: still open

**Verdict: no new filing; w8/132 confirmed still live.** Same isolated QA session, `bex-canary`, 2026-10-19. Checkout pulled to `1a49cb806` (production pin `ebfefea261db`) and the QA CLI rebuilt from it; session re-verified with `whoami`. Read-only log queries against the baseline service (no fixture created, nothing to clean up).

- Unfiltered `GET /v1/logs?ownerId=<ws>&resource=<baseline>&limit=100` → 100 rows, `hasMore: true` (the response is now the paginated `{hasMore, nextStartTime, nextEndTime, logs}` shape).
- Adding `&task=tsk-bogus595`, `&taskRun=run-bogus595`, or `&sandbox=sbx-bogus595` → the same 100 rows each. The filters are still accepted and silently ignored.
- Controls still discriminate: `&host=bogus.example.com` and `&instance=instance-bogus595` → 0 rows.
- Current source agrees: `parseLogParams` in [rest.go](/lego/backend/internal/logs/rest.go) still never reads `task`/`taskRun`/`sandbox`, despite the m237 live-tail work landing in the same package. The w8/132 note stands as filed; its preferred fix (thread the filters through, refuse on the tail) also covers the new `/logs/subscribe` path.

## Tail route checked too

- `GET /v1/logs/subscribe?…&task=tsk-bogus595` opens the stream (held open past an 8s probe) instead of refusing, while a misspelled `surpriseParam=1` gets `400 unsupported query parameter "surpriseParam" for subscribe-logs`. So the tail shares the same declared-but-unimplemented acceptance — no rows flowed during the window, so widening on the tail is inferred from the shared parser, not observed with data. This is inside w8/132's filed fix scope ("the refusal must fire on all three routes"); no separate filing.
