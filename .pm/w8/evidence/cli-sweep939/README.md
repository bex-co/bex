# CLI sweep 939 — deploy cancel lifecycle (live, 2026-10-20)

Disposable free web service created, second deploy triggered and canceled, service deleted (204, then GET 404). No new bug filed.

- Cancel of the in-flight deploy → 200; the deploy reads back `status: "canceled"` (verified on the single GET, not just the cancel response).
- Canceling the already-canceled (terminal) deploy → coded 409 `deploy "…" is already canceled` — terminal-state verbs conflict by name rather than re-succeeding silently (the counterpart to suspend/resume's idempotency, sweep 797: cancel is a one-way transition, so the repeat is a conflict, not a no-op).
- Harness note: the probe's inline double-invoke printed the first call's 200 beside the second call's conflict body; the single-GET status is the authoritative read and is what this evidence rests on.
