# CLI sweep 651 — `GET /services` pagination validation (live, 2026-10-19)

Read-only sweep against the workspace service list (no fixture created or mutated).

| Query | Result |
| --- | --- |
| `limit=1` | 200, one entry with `cursor` |
| `limit=0` | 400 `limit must be an integer between 1 and 100` |
| `limit=-1` | 400 same |
| `limit=101` | 400 same |
| `limit=abc` | 400 `invalid query parameter "limit"` |
| `cursor=bogus-cursor` | 200 `[]` (unknown cursor reads as past-the-end) |
| `limit=1&cursor=` | 200, first page (empty cursor ignored) |

Cursor walk verified: page 1 cursor (the entry name) fed back with `limit=1` returned the next distinct service. Bounds are named; the walk is functional. No bug filed.
