# CLI sweep 740 — malformed ids and single-event reads (live, 2026-10-19)

Disposable free web service `qa740-web` (`srv-dbbciv54c21c738t032g`). Deleted in sweep 741 and verified absent (REST 404).

- `GET /services/not-a-real-id` and a wrong-prefix id → 404 `NOT_FOUND` (no pattern 400 on service ids; lookup-shaped).
- `PATCH` on a synthetic service → 404 `NOT_FOUND`.
- `GET /events/{synthetic}` → 404 `EVENT_NOT_FOUND` with `params.id` echoing the request.
- `GET /events/{real}` for the fixture's create event → 200 with the bare event object (`deploy_ended`, deploy id/status in details), the w8/m24 single-event surface.

No bug filed.
