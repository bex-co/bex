# CLI sweep 689 — `deploys create` via pinned CLI (live, 2026-10-19)

Disposable free web service `qa689-web` (`srv-dbbboodfsg2s73ako8tg`), live at 200 before the trigger. Deleted and verified absent (REST 404).

- `bex deploys create <sid> --confirm -o json` (non-interactive) returned the created deploy object (`dep-…`, `status:"queued"`, `trigger:"api"`) instead of hanging on log streaming.
- The deploy progressed `update_in_progress` → `live` within ~25s; public URL stayed 200.

No bug filed.
