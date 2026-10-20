# CLI sweep 954 — env-var key validation edges (live, 2026-10-20)

One disposable free web service created, exercised, deleted (204; GET 404). No new bug filed.

- Duplicate key in one PUT batch → 200; read-back shows the last value (`2`). Last-wins set semantics; no pinned contract requires rejecting duplicates, so recorded, not filed.
- Empty key → coded 400 `ENVIRONMENT_VARIABLE_INVALID` naming the key, with `params.field=key` and `maxLength=253`; a key containing a space → 400 the same way. Refusals are coded and field-specific, not bare.
- Unicode value round-trips byte-identical (`héllo→✓`).
