# Usage metering

bex records month-to-date resource consumption per workspace and exposes it over REST, GraphQL, and MCP so any client — curl, the dashboard, or an MCP agent — can read the same numbers (pillar 1: API-first; pillar 3: agents as operators).

## Four meters

| Meter | Unit | Source |
| --- | --- | --- |
| `instance_seconds` | seconds (per tier) | cAdvisor container-presence signal via Prometheus — pod count × window seconds |
| `egress_bytes` | bytes | Traefik `traefik_service_responses_bytes_total` increase over the window |
| `build_seconds` | seconds | k8s build-Job `completionTime − startTime` for Jobs whose completion falls in the window |
| `storage_gb_seconds` | decimal GB-seconds | average `kubelet_volume_stats_used_bytes` over the window × window seconds, summed across a datastore's PVCs |

The first three meters match Render's compute, bandwidth, and pipeline-minute dimensions. Storage adds the separately-priced Postgres dimension while remaining API-first. Render charges provisioned Postgres capacity; bex meters actual used PVC bytes, so the rate is comparable but the usage basis is deliberately more usage-sensitive. Render has no separate Key Value storage line; bex exposes the same storage meter for Valkey because it also owns a persistent volume.

## Meter applicability by resource kind

The rollup loop emits meters for three resource kinds. Not every meter applies to every kind:

| Meter | App service (`service`) | Managed Postgres (`postgres`) | Managed Key Value (`key_value`) |
| --- | --- | --- | --- |
| `instance_seconds` | ✅ ReplicaSet pods (`<name>-[a-z0-9]+-[a-z0-9]{5}`) | ✅ CNPG stateful pods (`<name>-[0-9]+`) | ✅ Valkey stateful pods (`<name>-[0-9]+`) |
| `egress_bytes` | ✅ Traefik HTTP router counter | — TCP/SNI routes not tracked by Traefik's HTTP metrics | — TCP/SNI routes not tracked by Traefik's HTTP metrics |
| `build_seconds` | ✅ CNB build Job `completionTime − startTime` | — no build step | — no build step |
| `storage_gb_seconds` | — stateless App storage is not a supported product surface | ✅ CNPG PVCs (`<name>-<n>`) | ✅ Valkey PVC (`data-<name>-<n>`) |

Each row in `usage_hourly` and `usage_monthly` carries a `resource_kind` column (`DEFAULT 'service'`, migration `0015_usage_resource_kind.up.sql`) so the REST/GraphQL/MCP surfaces can distinguish App compute from managed-datastore compute. The column is backward-compatible: rows written before the migration surface as `"service"`. Migration `0021_usage_resource_identity.up.sql` also makes `resource_kind` part of each table's primary key because a `Database` and `KeyValue` CR may legally share a Kubernetes name; their usage remains separate even when `service_id`, meter, tier, and window are identical.

Migration `0022_usage_storage_kind.up.sql` extends both row-oriented tables' `kind` constraint with `storage_gb_seconds`. Existing rows need no rewrite: absence of a storage row means no previously-recorded storage usage, while successful zero-byte Prometheus queries persist an explicit zero row.

## How it works

An hourly rollup loop (`usage.Service.Run`) writes rows to the `usage_hourly` table (migration `0006_usage.up.sql`) whenever both `BEX_CP_DB_URI` (the control-plane store) and `BEX_PROM_URL` (Prometheus) are set. Each row is keyed on `(resource_kind, service_id, kind, tier, window_start)` and is idempotent: re-processing a window (`ON CONFLICT … DO UPDATE`) never double-counts. On startup and every hourly pass, the loop catches up missed windows bounded to the last 48 hours.

### Successful zeroes and gap-free App-meter cursors

For App services, each meter (`instance_seconds`, `egress_bytes`, and `build_seconds`) owns an independent, contiguous cursor:

- A successful source read always persists an hourly row, including quantity zero. An empty Prometheus vector or a successful Kubernetes Job list with no completed build is measured zero usage, not missing data.
- An unavailable source (unset Prometheus/Kubernetes client or a failed request/list) and a failed store write persist no row. That meter stops at the failed window and retries it on the next hourly pass before advancing to newer windows.
- The three meters advance independently and are collected concurrently. A transient egress failure therefore cannot hold build/instance metering back, and vice versa.
- The existing 48-hour catch-up bound still applies and is clamped to the App's creation hour, so a new service never gains synthetic pre-creation coverage. Outages longer than 48 hours are visible as gaps rather than silently synthesized as zero.

Successful zero egress/build rows are coverage anchors in `usage_hourly`; the period aggregation omits their all-zero groups so REST/GraphQL/MCP response semantics remain unchanged. `instance_seconds` retains zero groups for tiered suspended services, as before. Any analysis that needs to prove collection completeness must query raw hourly rows from the deployment time of this corrected contract; older positive-only rows are consumption evidence, not coverage evidence.

The loop iterates `ListApps` (App services) and then all `Database` and `KeyValue` CRs in the operator's namespace — both using the `bex.co/tenant` label to identify the owning workspace. Database/KeyValue CRs use their CR name as `service_id` (name-as-id, the same documented deviation managed Postgres takes — [docs/ADR020-identifiers.md](ADR020-identifiers.md)). For each closed datastore window, the collector averages the kubelet used-byte gauge across the hour, sums all matching PVCs (including CNPG replicas), converts bytes to decimal GB, and multiplies by elapsed seconds. Per-meter catch-up cursors let storage backfill independently for up to 48 hours and idempotent upserts prevent double-counting when a window is retried.

With `BEX_CP_DB_URI` set but `BEX_PROM_URL` absent the service is wired (the month-to-date read still works) but the metering side of the loop is skipped — only the retention compaction below runs.

## Retention: hourly detail compacts into monthly aggregates

`usage_hourly` accrues one row per resource × meter kind × tier × hour forever, so the same loop bounds its growth (w8/m4): once a calendar month falls out of the **hot window**, a daily compaction pass folds its hourly rows into the `usage_monthly` table (migration `0007_usage_monthly.up.sql`) — one row per `(resource_kind, service_id, kind, tier, month)` — and purges the compacted hourly detail.

`BEX_USAGE_RETENTION_MONTHS` sets the hot window: how many calendar months (current month included) keep full hourly detail. Default **3** (this month + the prior two — the common historical-comparison range), minimum 1; a very large value effectively disables compaction.

Properties of the compaction pass:

- **Lossless for totals.** Compaction is a plain `SUM` per calendar month, not a sample: `period=` queries return exactly the same totals before and after a month is compacted. Only sub-month (hourly) resolution is lost, and only for months older than the hot window.
- **Transparent to the API.** The period query sums `usage_hourly` and `usage_monthly` together, so REST/GraphQL/MCP answers are correct whether a month is still hot, already compacted, or (never yet) compacted — correctness never depends on the compaction having run.
- **Atomic and idempotent.** The aggregate-insert and hourly-purge are a single statement, so a crash can never leave the two tables inconsistent, and re-running over the same window is a no-op. Straggler hourly rows compacted by a later pass _add_ to their month's aggregate.
- **Never races the rollup.** The compaction boundary is clamped to 48 hours ago (the rollup's restart catch-up limit), so a window the metering loop might still rewrite is never compacted underneath it.

The pass runs daily (plus once at startup) inside `usage.Service.Run`; it needs only `BEX_CP_DB_URI`, not Prometheus.

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
| `period` | Calendar month as `YYYY-MM`. Defaults to the current month. For a past month the full month is returned; for the current month, data up to now is returned. Months older than the hot window are served from monthly aggregates with identical totals (see Retention above). |

Response:

```json
{
  "workspaceId": "tea-abc123",
  "period": "2026-07",
  "services": [
    {
      "serviceId": "srv-xyz456",
      "resourceKind": "service",
      "rows": [
        { "kind": "instance_seconds", "tier": "starter", "total": 14400 },
        { "kind": "egress_bytes", "total": 2048 },
        { "kind": "build_seconds", "total": 120 }
      ]
    },
    {
      "serviceId": "mydb",
      "resourceKind": "postgres",
      "rows": [
        { "kind": "instance_seconds", "tier": "basic-256mb", "total": 3600 },
        { "kind": "storage_gb_seconds", "total": 7200 }
      ]
    }
  ]
}
```

`tier` is omitted on the JSON response when it is the empty string (non-compute meters). `resourceKind` identifies the resource type (`"service"`, `"postgres"`, `"key_value"`). `services` is always a JSON array (never `null`).

### GraphQL

```graphql
query Usage($period: String) {
  usage(period: $period) {
    workspaceId
    period
    services {
      serviceId
      resourceKind
      rows {
        kind
        tier
        total
      }
    }
  }
}
```

The `usage` query is workspace-scoped — no `resourceId` argument needed. `period` is optional (`YYYY-MM`); omitting it returns the current calendar month. The `period` field in the response echoes back the queried month (identical semantics as REST). REST, GraphQL, and MCP are now capability-symmetric on historical period queries.

`UsageRow.total` is a GraphQL `Float`, not `Int`: GraphQL's standard `Int` is only signed 32-bit, while one TB-month is already 2,628,000,000 storage GB-seconds. The service resolves an integer counter and IEEE-754 represents all realistic monthly totals here exactly; REST and MCP continue to serialize the same counter as an integer JSON number.

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
| `BEX_PROM_URL` unset | The store reads still work and the retention compaction still runs; only the metering rollup doesn't (existing rows are served) |
| Prometheus unreachable at rollup time | The affected storage window is deferred (logged) and retried before newer storage windows; the query surface remains available |

## Pre-declared drift from Render

Render's public REST API and GraphQL surface have no usage or billing endpoints (billing is dashboard-only; verified against Render's OpenAPI spec 2026-07-09 — no `/usage`, `/billing`, or equivalent exists). This surface is therefore a bex extension. Render's Postgres storage rate is `$0.30/GB-month`, prorated by the second, but applies to provisioned capacity; bex applies a 30%-lower `$0.21/GB-month` estimate to actual used bytes. Render has no independent Key Value storage charge; bex applying the same transparent used-storage rate to Valkey is a deliberate extension. See [docs/ADR018-render-parity.md](ADR018-render-parity.md) § bex ahead of Render.
