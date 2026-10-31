# m126 fresh-fixture acceptance — 2026-10-31

Production (`api.bex.co`, `dashboard.bex.co`), workspace `bex-canary`, QA human via device login. Authorized by the 2026-10-29 D3/D4 decisions and the 2026-10-31 `/unblock w8` A decision (bex-canary pre-cleaned first; see [m65 evidence](../../../done/m65/evidence/burst-replay-20261031/README.md)).

## Free → `expiresAt` = creation + 30 days

`POST /v1/postgres {"plan":"free","version":"18"}` → `dpg-dbj69segv99s73eclaag` (`qa-20261031-w8m126-pg`), `createdAt 2026-10-31T22:00:17Z`.

| surface | `expiresAt` |
| --- | --- |
| REST `POST` and `GET /v1/postgres/{id}` | `2026-11-30T22:00:17Z` |
| GraphQL `database(id)` | `2026-11-30T22:00:17Z` |
| MCP `get_postgres` | `2026-11-30T22:00:17Z` |
| MCP `list_postgres_instances` (`workspaceId` bex-canary) | `2026-11-30T22:00:17Z` |
| Dashboard `/databases/{id}` | "Expires in 29d", `<time datetime="2026-11-30T22:00:17Z">` |

## Upgrade clears it

`PATCH /v1/postgres/{id} {"plan":"basic_256mb"}` → `200`. Then REST omits `expiresAt`, GraphQL `expiresAt: null`, MCP get/list carry none, and the dashboard has no Expires row.

## Cleanup

`DELETE` at 2026-10-31T22:02:27Z → `204`; `GET` → `404`; the workspace's Postgres list is `[]`. The paid plan was live about 1.5 minutes.

## Sweeper lifecycle

Taken on the test clock the DoD allows: `go test ./internal/postgres -run 'Expir|FreeExpiry|Upgrade|Downgrade|PreRollout'` and `./internal/notifications` pass on `main` (2026-10-31) — the real `FreeExpirySweeper` on an injected `Service.Clock` warns at −7 d, suspends at `expiresAt` (`billing` suspender, `POSTGRES_FREE_EXPIRED`, resume refused), final-warns, deletes after 14 d through the user delete path, writes the four `postgres.FreeExpiry*` audit rows, collapses overdue stages, and claims each stage once across two replicas. **Limit:** not replayed against a live dev-cluster apiserver; the production stages fall on 2026-11-19 / 11-26 / 12-07 / 12-10 for the rollout-clamped databases (see t009's 2026-10-29 read-only check).
