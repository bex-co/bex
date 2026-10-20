# CLI sweep 852 — `maxShutdownDelaySeconds` + `port` PATCH round-trips (live, 2026-10-20)

Disposable free image-backed web service created, patched, deleted (204, then GET 404). No new bug filed. Full-parse harness (sweep 851's helper) used throughout, so no truncation artifacts.

- `maxShutdownDelaySeconds: 90` → 200 and read back; `-5` → named 400 `maxShutdownDelaySeconds must be 1-300`.
- `port: 8080` → 200 and read back on `serviceDetails.port`; both values coexist on the final GET.
