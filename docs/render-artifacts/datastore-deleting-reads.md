# Deleted datastore by-id reads (Render evidence, 2026-09-08)

**Why:** pins the documented Render behavior that `w8/m35` folds Postgres/Key Value onto, with an explicit live-capture boundary.

## Documented behavior

Render's pinned public OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`, capture used by bex's request validator):

| Operation | Path | Declared success | Declared miss |
| --- | --- | --- | --- |
| Retrieve Postgres | `GET /postgres/{postgresId}` | `200` | `404` |
| Retrieve Redis (Key Value predecessor) | `GET /redis/{redisId}` | `200` | `404` |

Neither schema declares a first-class `deleting` (or similar) status on the retrieve response that would let a deleted/deleting instance keep answering `200`. The enum/string inventory in the pin contains **zero** `deleting` tokens. Connection-info / sibling retrieve shapes inherit the same miss vocabulary (404) rather than a long-lived deleting object.

Public docs describe delete as removing the instance; there is no documented "get while deleting returns a deleting status" contract for Postgres or Redis/Key Value that would contradict 404.

## Evidence boundary

**No live Render capture this session.** This environment has no `RENDER_API_KEY` (tracked as `.pm/w10/002.md`), so a create→delete→GET cycle against api.render.com was not run. The decision below rests on the pinned OpenAPI + docs the way other ADR018 rows record a documented-evidence boundary when live keys are unavailable.

## Decision

**Option 1 confirmed** — fold bex Postgres/Key Value by-id reads onto consistent-absence (`core.NotFoundIfDeleting`), matching services (`w3/m81`) and Render's documented 404 on retrieve. Option 2 (deliberate visible `deleting`) is not warranted by the documented evidence.
