# CLI sweep 756 — datastore list filters (live, 2026-10-19)

Read-only.

- `GET /postgres?status=bogus` → 400 `unsupported query parameter "status" for list-postgres` (no status filter on this route; the validator says so).
- `GET /postgres?name=<baseline>` → 200 with exactly the baseline.
- `GET /key-value?region=bogus` → 400 `region must be one of frankfurt|oregon|ohio|singapore|virginia` (the Render-named enum at the filter layer, while stored instances report `fsn1` — sweep 658/ADR018 normalization).

No bug filed.
