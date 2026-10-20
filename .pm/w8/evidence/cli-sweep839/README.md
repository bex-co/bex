# CLI sweep 839 — single-event retrieval + derived-id validation (live, 2026-10-20)

Read-only probes on the baseline (read only). No new bug filed.

- `GET /events/{evt-…}` for a real baseline event → 200 bare `{id, timestamp, serviceId, type, details}` (the w8/m24 single-event contract; the latest baseline event is the bex-extension `service_hibernated`, documented in w8/060).
- Derived-id discipline: `evt-` ids are 100 bits of SHA-256 in base32hex (`[0-9a-v]{20}`, `lego/backend/internal/id/id.go` `Derive`). A 22-char suffix → 400 `invalid path parameter "eventId"`; a `z`-containing 20-char suffix (outside base32hex) → coded 400 `EVENT_ID_INVALID`; a well-formed all-zeros id → coded 404 `EVENT_NOT_FOUND` echoing the id in `params`.
