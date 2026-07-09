# Usage metering

bex records month-to-date resource consumption per workspace and exposes it over REST, GraphQL, and MCP so any client — curl, the dashboard, or an MCP agent — can read the same numbers (pillar 1: API-first; pillar 3: agents as operators).

## Three meters

| Meter | Unit | Source |
| --- | --- | --- |
| `instance_seconds` | seconds (per tier) | cAdvisor container-presence signal via Prometheus — pod count × window seconds |
| `egress_bytes` | bytes | Traefik `traefik_service_responses_bytes_total` increase over the window |
| `build_seconds` | seconds | k8s build-Job `completionTime − startTime` for Jobs whose completion falls in the window |

Meters match Render's three billing dimensions exactly (compute time per tier, bandwidth, pipeline minutes) so the data is comparable for operators migrating from Render.

## How it works

An hourly rollup loop (`usage.Service.Run`) writes rows to the `usage_hourly` table (migration `0004_usage.up.sql`) whenever both `BEX_CP_DB_URI` (the control-plane store) and `BEX_PROM_URL` (Prometheus) are set. Each row is keyed on `(service_id, kind, tier, window_start)` and is idempotent: re-processing a window (`ON CONFLICT … DO UPDATE`) never double-counts. On restart the loop catches up missed windows bounded to the last 48 hours.

With `BEX_CP_DB_URI` set but `BEX_PROM_URL` absent the service is wired (the month-to-date read still works) but the loop does not start.

## API surface

All three adapters call the same `MonthToDate` verb and return identical quantities. This is a deliberate bex extension — Render's public REST API has no usage/billing endpoints.

### REST

```
GET /v1/usage
```

Optional query parameters:

| Parameter | Description |
| --- | --- |
| `ownerId` | Accepted but ignored; the response always reflects the caller's own workspace. |
| `period` | Calendar month as `YYYY-MM`. Defaults to the current month. For a past month the full month is returned; for the current month, data up to now is returned. |

Response:

```json
{
  "workspaceId": "tea-abc123",
  "period": "2026-07",
  "services": [
    {
      "serviceId": "srv-xyz456",
      "rows": [
        { "kind": "instance_seconds", "tier": "starter", "total": 14400 },
        { "kind": "egress_bytes", "total": 2048 },
        { "kind": "build_seconds", "total": 120 }
      ]
    }
  ]
}
```

`tier` is omitted on the JSON response when it is the empty string (non-compute meters). `services` is always a JSON array (never `null`).

### GraphQL

```graphql
{
  usage {
    workspaceId
    services {
      serviceId
      rows {
        kind
        tier
        total
      }
    }
  }
}
```

The `usage` query is workspace-scoped — no `resourceId` argument needed. Period always defaults to the current calendar month; use REST `?period=` for historical queries.

### MCP

```json
{
  "name": "get_usage",
  "arguments": {
    "period": "2026-07"
  }
}
```

`period` is optional (defaults to current month). Returns the same JSON envelope as REST.

## Authorization

The `MonthToDate` verb checks `can_view` on the caller's workspace. A caller with no workspace relation is denied (HTTP 403 / GraphQL error / MCP error). A caller from another workspace cannot read a different workspace's data — the workspace is always resolved from the caller's own token.

## Availability

| Condition | Behavior |
| --- | --- |
| `BEX_CP_DB_URI` unset | All three adapters return HTTP 503 / GraphQL error / MCP error |
| `BEX_PROM_URL` unset | The store reads still work; only the rollup loop doesn't run (existing rows are served) |
| Prometheus unreachable at rollup time | The affected window is skipped (logged); the query surface is unaffected |

## Pre-declared drift from Render

Render's public REST API and GraphQL surface have no usage or billing endpoints (billing is dashboard-only; verified against Render's OpenAPI spec 2026-07-09 — no `/usage`, `/billing`, or equivalent exists). This surface is therefore a bex extension. See [docs/render-parity.md](render-parity.md) § bex ahead of Render.
