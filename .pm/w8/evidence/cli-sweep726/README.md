# CLI sweep 726 — rename via REST and delete by name (live, 2026-10-19)

Disposable free web service `qa726-web` (`srv-dbbcbrdfsg2s73akoag0`). Deleted and verified absent (REST 404).

- `PATCH {"name":"qa726-web"}` (unchanged) → 200 no-op; `PATCH {"name":"Qa726 Display!"}` → 200, and `{"displayName":"Qa726 Display"}` normalized onto the same label — `name`/`displayName` are one display label via REST too, `slug`/`immutableName` untouched (sweeps 583/653 semantics on the REST surface).
- `bex services delete 'Qa726 Display'` resolved the current display name and deleted the service.

No bug filed.
