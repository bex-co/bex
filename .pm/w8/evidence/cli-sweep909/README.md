# CLI sweep 909 — size behavior at create/write bounds (live, 2026-10-20)

Disposable free web service created, exercised, deleted (204, then GET 404); the oversized create was refused. No new bug filed.

- A 200 KB env-var value → 200 accepted. Recorded without filing: no pinned schema or ADR states an env-value size cap for this surface, so there is no contract to measure the acceptance against (the store's own ceiling would surface at apply, not at this write).
- A 10 000-char service name → the same coded `RESOURCE_NAME_INVALID` (the 30-char rule fires regardless of magnitude; no length-based 500 or truncation).
