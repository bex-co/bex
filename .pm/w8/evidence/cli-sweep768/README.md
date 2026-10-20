# CLI sweep 768 — empty `resource` on metrics: live confirmation of w4/370 (live, 2026-10-19)

Read-only.

- `GET /metrics/cpu?resource=` and `/metrics/memory?resource=` → 500 `internal error`. This is the already-filed w4/370 (`empty REST metrics resource answers 500`), independently live-confirmed here — not refiled in w8.
- Control: omitting `resource` entirely → 400 `resource is required` (sweep 675). The empty-string value slips past the presence check into the 500.

No new bug filed.
