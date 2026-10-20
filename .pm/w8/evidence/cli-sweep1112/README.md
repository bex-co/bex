# CLI sweep 1112 — JSON null in PATCH fields (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `PATCH {name: null}` → schema 400 at `/name` (the immutable name cannot be nulled). `PATCH {displayName: null}` → 200 treated as omission — the name reads back intact. Null and omission coincide for the pointer-shaped label field, while required identity fields refuse null outright.
