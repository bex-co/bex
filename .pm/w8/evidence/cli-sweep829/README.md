# CLI sweep 829 — service PATCH field discipline (live, 2026-10-20)

Disposable free web service created, patched, deleted (204, then GET 404). No new bug filed.

- `PATCH {"name":"qa829-renamed"}` → display label changes (`name`/`displayName`) while `immutableName` and `slug` stay `qa829-web` — the documented rename-is-a-label contract (sweeps 583/653/726).
- Unknown field → 400 naming it (`request body contains unknown field "bogusField829"`); empty `{}` PATCH → 200 no-op.
- `serviceDetails.numInstances` is not a PATCH field (400 unknown field) — instance count is owned by the scale endpoint (sweep 796), so the two write paths cannot disagree.
