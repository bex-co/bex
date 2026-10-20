# CLI sweep 887 — service events window/limit discipline (live, 2026-10-20)

Disposable free web service created, deployed, deleted (204; GET and its events both 404 after). No new bug filed.

- Malformed `startTime` → 400 `invalid query parameter "startTime"`; an inverted start/end window → 200 with no rows (empty intersection, same reading as the list filters in sweep 860).
- `limit=1` returns exactly the latest event (`deploy_ended`); `limit=1000` → 400 (the 1–100 bound from sweep 859 enforced here too).
- After deletion the events subresource 404s with the service (the m81 consistent-absence contract: no feed outlives its service).
