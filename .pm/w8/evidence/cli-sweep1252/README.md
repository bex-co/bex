# CLI sweep 1252 — API rate-limit headers against Render's current contract (read-only, 2026-10-20)

Read-only: one authenticated `GET /v1/services?limit=1`, plus source and spec reads. **Finding → [w8/147](../../147.md).**

- A Bex response carries no `RateLimit-Limit`/`-Remaining`/`-Reset`. Render's current rate-limiting reference and the pinned `render-public-api-1.json` (`components.responses.429RateLimit`) specify them on every response and on 429s. w7/m34's 2026-07-15 “parity-by-absence” relied on an older spec copy.
- Coverage checks for Postgres pooling, read replicas, recovery and Blueprint cross-references found them well covered by earlier sweeps (87, 75, 73 and 254 evidence files), so no new fixtures were spent there.
