# CLI sweep 1183 — numeric detail fields are range/type checked (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `maxShutdownDelaySeconds` of `-1` and `"30"` (string) → 400; `60` → 200 and reads back 60. Same negative/string refusal pattern as port (sweep 1182) and scale (sweep 1136).
