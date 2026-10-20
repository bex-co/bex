# CLI sweep 1069 — metrics query discipline (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `GET /metrics/cpu?resource={fresh-service}` → 200 (empty-or-early series window answers, no 404 for a just-created resource).
- A malformed `startTime` → 400; an unsupported `step` parameter → coded 400 naming the parameter and the operation (`unsupported query parameter "step" for get-cpu`) — the same strict per-operation query vocabulary as sweep 1028.
