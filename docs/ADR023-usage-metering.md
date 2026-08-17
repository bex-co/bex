# Usage metering

bex records month-to-date resource consumption per workspace and exposes it over REST, GraphQL, and MCP so any client — curl, the dashboard, or an MCP agent — can read the same numbers (pillar 1: API-first; pillar 3: agents as operators).

## Five meters

| Meter | Unit | Source |
| --- | --- | --- |
| `instance_seconds` | seconds (per tier) | cAdvisor container-presence signal via Prometheus — pod count × window seconds |
| `egress_bytes` | bytes | loss-detecting sum of exact App HTTP + WebSocket + direct-public sources, or the public datastore proxy response source |
| `build_seconds` | seconds | k8s build-Job `completionTime − startTime` for Jobs whose completion falls in the window |
| `storage_gb_seconds` | decimal GB-seconds | average `kubelet_volume_stats_used_bytes` over the window × window seconds, summed across a datastore's PVCs |
| `sandbox_compute_seconds` | milli-vCPU-equivalent seconds | OpenSandbox lifecycle phase × shape; CPU millicores plus memory GiB folded at the AgentCore `$0.00945/GB-hour ÷ $0.0895/vCPU-hour` reference ratio; only `running` intervals accrue |

The first three meters match Render's compute, bandwidth, and pipeline-minute dimensions. Storage adds the separately-priced Postgres dimension while remaining API-first. Render charges provisioned Postgres capacity; bex meters actual used PVC bytes, so the rate is comparable but the usage basis is deliberately more usage-sensitive. Render has no separate Key Value storage line; bex exposes the same storage meter for Valkey because it also owns a persistent volume.

### Complete outbound-bandwidth contract (w8/m15)

`egress_bytes` is a composition, not a synonym for one edge metric. An App row sums every applicable App source; a public Database or Key Value row reads its metered public proxy. Private datastores have no public-egress source and persist a successful zero. The collector never returns a partial sum: required-source absence, health loss, or an ambiguous counter decrease leaves the hour absent and holds that meter's cursor for retry.

The HTTP component resolves a control-plane App row to its projected CR through the immutable `bex.co/app-id` label, reads that CR's actual operator-owned Ingress, and reproduces the router-key algorithm from the production-pinned Traefik v3.7.5. Prometheus selectors are anchored to the complete `router@kubernetes` labels; display-name substrings and backend Service names are never used for bandwidth attribution. This keeps ordinary services, custom domains, and shared-backend static sites separate. A genuinely private App with no Ingress records a successful zero. A missing projected CR, missing expected public Ingress, unsupported Ingress shape, ambiguous projection or cross-Ingress router label, Kubernetes failure, or Prometheus failure writes no row and is retried by the per-meter cursor.

The contract follows Render's documented categories while keeping bex's source boundaries explicit. Traefik's HTTP response-size capture stops at connection hijack, so HTTP responses and WebSocket downstream frames are separate sources rather than one counter with an optimistic label:

| Traffic class | Counted source | Attribution | Excluded / double-count guard |
| --- | --- | --- | --- |
| App HTTP responses, including `static_site` | Traefik router response-byte counter | App Ingress/router identity | The App→Traefik private hop is not counted by the direct meter |
| App WebSocket downstream frames after upgrade | dedicated metered edge connection wrapper | exact App router identity | handshake HTTP bytes stay in the router counter; client→App frames and the private backend hop are excluded |
| App-initiated TCP/UDP to a public destination | node-local post-policy netfilter eBPF meter | immutable Pod UID + Pod IP → `service` resource | cluster/private/link-local/node destinations, DNS-to-cluster, dropped packets |
| Public managed-Postgres responses | Postgres-aware SNI proxy's backend→client copy counter | resolved parent `Database` (RW/pooler/replica all roll up) | client→backend requests, internal DB clients, backups |
| Public managed-Key-Value responses | metered SNI pass-through front door's backend→client copy counter | resolved `KeyValue` | client→backend requests and internal KV clients |

The byte bases are intentionally explicit:

- HTTP is Traefik's response-body bytes on the exact `router@kubernetes` counter. Response headers are not included.
- WebSocket is the number of bytes successfully written on the hijacked public downstream connection after the `101` header. It includes WebSocket frame headers and payload, but not the HTTP handshake or client→server frames.
- Direct public traffic is L3 wire length at host `POST_ROUTING` after Cilium policy and before source NAT: IPv4 `total_length`, or the 40-byte IPv6 header plus `payload_len`. It includes IP and TCP/UDP headers, retransmissions that reach the hook, and payload; it excludes L2 framing.
- Postgres and Key Value are encrypted backend→client TCP bytes successfully copied by the public SNI proxy after route selection. TLS records, including the backend TLS handshake, are included. The PostgreSQL proxy's synthetic one-byte `S` response and all client→backend bytes are excluded.

Each source is monotonic and independently health-checked. The hourly collector treats any required-source failure, unsupported attachment, counter loss, reset, malformed Prometheus response, or non-finite/negative sample as unavailable: it writes no row and retries the meter's oldest missing hour under the existing `m11` cursor contract. Only an explicit successful empty Prometheus vector is a real zero. Every health query checks `up`, the source-specific health gauge, at least 80% of the expected 15-second samples across the complete window, and zero counter resets, so a missing target or unknowable pre-reset tail cannot masquerade as an exact delta. HTTP, WebSocket, Postgres, and Key Value also expose process-start time: a restart before the first in-window scrape is rejected even though Prometheus has no preceding sample from which `resets()` could infer it. Direct-meter health additionally requires a durable last-loss timestamp older than the complete window. A checkpoint restoration equal to the last scrape can otherwise hide unsampled bytes without producing a Prometheus reset; the persisted timestamp closes that boundary case and rejects every overlapping hour.

The node meter pins its IPv4/IPv6 netfilter links and maps in bpffs. A process restart reopens the same links, uses a node-persistent `source_instance`, and preserves packet coverage and counter continuity; the link is replaced only when its map identities or hook contract do not match. A host checkpoint restores counters after map loss. The durable `/var/lib/bex-egress-meter/counter-loss.json` state carries both `bex_egress_meter_counter_loss_events_total` and `bex_egress_meter_last_counter_loss_time_seconds` across process/node restarts. `EgressMeterCounterLoss` pages on a recent event, while the collector's timestamp and `resets()` guards invalidate the exact overlapping windows. A live decrease below the last checkpoint marks the exporter itself unhealthy and preserves the older checkpoint instead of writing a smaller baseline. Proxy and edge process resets also invalidate their overlapping windows because their process-local pre-scrape tails are not reconstructable. The DaemonSet uses `HostToContainer` propagation for bpffs so a Cilium remount during node boot becomes visible to the meter's attachment retry loop.

### Live Cilium traffic matrix (2026-07-15)

`scripts/verify-egress-meter.sh` passed on a three-node kind cluster with Cilium 1.19.5, kube-proxy replacement, VXLAN, WireGuard, and `bpf.hostLegacyRouting=true`. The isolated manual workflow is `.github/workflows/egress-meter-live.yml`; it is not part of routine unit-test runs. Only aggregate fixture results are retained:

| path | expected | observed |
| --- | --: | --: |
| exact HTTP router response body | 4,096 B | 4,096 B |
| WebSocket server frame on the hijacked public connection | 4,100 B wire frame | 4,100 B |
| WebSocket client frame | excluded | 0 B |
| direct public UDP | 512 B payload + 4 B fixture header + 28 B IPv4/UDP | 544 B |
| direct public TCP | payload plus TCP/IP control overhead | 4,372 B |
| same-cluster TCP including cluster-DNS lookup | excluded | 0 B |
| Cilium-policy-dropped public UDP | excluded | 0 B |
| public Postgres backend→client TLS with a 65,536 B request | encrypted response direction only | 6,031–6,032 B across final reruns |
| public Key Value backend→client TLS with a 65,536 B request | encrypted response direction only | 6,031–6,032 B across final reruns |

Pod replacement retained the resource counter and attributed the replacement only after its new Pod UID/IP reconciliation. A meter process restart retained both `source_instance` and the pinned total. Deliberate map deletion restored the last checkpoint, exposed a 544 B pre-checkpoint gap through a counter decrease and a durable loss event, and therefore made the affected hourly direct source fail. A subsequent node-container reboot retained the restored total and source identity, loaded the first durable loss event, recorded a second restoration event, and advanced rather than reset the loss timestamp; the meter reached Ready after Cilium remounted bpffs. No tenant identifiers or raw traffic are recorded.

This total is a stable bex usage quantity, not a claim that it reproduces a cloud provider's opaque invoice byte-for-byte.

The source audit rejected tempting shortcuts for concrete correctness reasons:

- **cAdvisor pod transmit bytes** are cumulative but carry no destination class, so they would charge same-workspace/private and cluster traffic.
- **Hubble flow events/metrics** carry source and destination identities, but the [v1.19 flow contract](https://github.com/cilium/cilium/blob/v1.19.5/api/v1/flow/flow.proto) has no packet-byte field; `flows_to_world_total` is a flow count, not bytes.
- **Cilium policy statistics** are useful diagnostics, not a durable usage ledger. In v1.19.5 the [stats map](https://github.com/cilium/cilium/blob/v1.19.5/pkg/maps/policymap/statsmap.go) is a bounded per-CPU LRU hash keyed by endpoint id and policy key. [Endpoint removal](https://github.com/cilium/cilium/blob/v1.19.5/pkg/maps/policymap/cell.go) removes the endpoint policy map but not old stats, and [policy updates](https://github.com/cilium/cilium/blob/v1.19.5/pkg/maps/policymap/policymap.go) zero the associated stat only in debug mode. Eviction, endpoint-id reuse, and reset semantics can therefore silently lose or misattribute quota bytes.
- **Traefik service response bytes alone** cover HTTP only. Traefik documents [router response-byte metrics](https://doc.traefik.io/traefik/reference/install-configuration/observability/metrics/#router-metrics), which solve shared-backend HTTP attribution, but its [capture writer](https://github.com/traefik/traefik/blob/v3.7.5/pkg/middlewares/capture/capture.go) returns the underlying connection on `Hijack`, so WebSocket frames bypass `ResponseSize`; it also exposes no equivalent per-TCP-service response-byte counter for public datastore routes.

This design intentionally owns the accounting datapath instead of scraping a CNI's debug ABI. Production Cilium is configured with `bpf.hostLegacyRouting=true` so allowed pod egress traverses host netfilter; the meter attaches only its own `POST_ROUTING` links and does not replace Cilium programs. The local CAPD overlay still uses Calico, so the focused validation uses a dedicated Cilium 1.19.5 fixture. Unsupported kernels fail readiness and export health zero.

## Meter applicability by resource kind

The rollup loop emits meters for four resource kinds. Not every meter applies to every kind:

| Meter | App service (`service`) | Managed Postgres (`postgres`) | Managed Key Value (`key_value`) | Hosted sandbox (`sandbox`) |
| --- | --- | --- | --- | --- |
| `instance_seconds` | ✅ ReplicaSet pods (`<name>-[a-z0-9]+-[a-z0-9]{5}`) | ✅ CNPG stateful pods (`<name>-[0-9]+`) | ✅ Valkey stateful pods (`<name>-[0-9]+`) | — separate weighted meter |
| `egress_bytes` | ✅ exact HTTP router + WebSocket downstream + direct-public composition | ✅ public proxy response bytes; private is explicit zero | ✅ public proxy response bytes; private is explicit zero | — not yet metered |
| `build_seconds` | ✅ CNB build Job `completionTime − startTime` | — no build step | — no build step | — no build step |
| `storage_gb_seconds` | — stateless App storage is not a supported product surface | ✅ CNPG PVCs (`<name>-<n>`) | ✅ Valkey PVC (`data-<name>-<n>`) | — rootfs storage not priced |
| `sandbox_compute_seconds` | — | — | — | ✅ durable OpenSandbox phase cursor × weighted shape |

Each row in `usage_hourly` and `usage_monthly` carries a `resource_kind` column (`DEFAULT 'service'`, migration `0015_usage_resource_kind.up.sql`) so the REST/GraphQL/MCP surfaces can distinguish App compute, managed-datastore compute, and hosted sandboxes (`"sandbox"`). The column is backward-compatible: rows written before the migration surface as `"service"`. Migration `0021_usage_resource_identity.up.sql` also makes `resource_kind` part of each table's primary key because different resource kinds may legally share a name; their usage remains separate even when `service_id`, meter, tier, and window are identical.

Migration `0022_usage_storage_kind.up.sql` extends both row-oriented tables' `kind` constraint with `storage_gb_seconds`. Existing rows need no rewrite: absence of a storage row means no previously-recorded storage usage, while successful zero-byte Prometheus queries persist an explicit zero row.

## How it works

An hourly rollup loop (`usage.Service.Run`) writes rows to the `usage_hourly` table (migration `0006_usage.up.sql`) whenever both `BEX_CP_DB_URI` (the control-plane store) and `BEX_PROM_URL` (Prometheus) are set. Each row is keyed on `(resource_kind, service_id, kind, tier, window_start)` and is idempotent: re-processing a window (`ON CONFLICT … DO UPDATE`) never double-counts. On startup and every hourly pass, the loop catches up missed windows bounded to the last 48 hours.

Sandbox compute is the non-Prometheus exception (migration `0059_sandbox_compute_meter`): a 30-second tenant-scoped OpenSandbox poll plus immediate create/pause/resume/terminate observations advance a durable `(workspace_id, sandbox_id)` phase cursor. Advancing the cursor and adding the prior `running` interval to hour-split `usage_hourly` rows happen in one transaction; an equal or older replay is a no-op. Suspended, creating, resuming, errored, and terminated intervals add zero. A failed upstream list advances nothing, while a sandbox missing from a successful complete list closes its final interval. Sandboxes created before the shape metadata existed are not retroactively charged.

### Successful zeroes and gap-free per-resource meter cursors

For App services, each meter (`instance_seconds`, `egress_bytes`, and `build_seconds`) owns an independent, contiguous cursor. Datastore instance, storage, and egress meters use the same independent-cursor rule:

- A successful source read always persists an hourly row, including quantity zero. An empty Prometheus vector or a successful Kubernetes Job list with no completed build is measured zero usage, not missing data.
- An unavailable source (unset Prometheus/Kubernetes client or a failed request/list) and a failed store write persist no row. That meter stops at the failed window and retries it on the next hourly pass before advancing to newer windows.
- **Egress health failure is final, not retryable (w1/m51):** the egress meter gates health PER SOURCE within the hour — a source whose health product fails (in-window counter reset, meter `up` gap, counter loss) is skipped for that hour while the healthy sources' `increase()` still records; every source skipped records a successful zero. Only a transport failure (the previous bullet) defers the hour, because a past hour's samples never improve — under the earlier any-source-unhealthy-defers rule, prod recorded **zero** `egress_bytes` rows for all of July 2026 (`w1/034`). Skipping the unhealthy source keeps its possibly reset-inflated increase out of billing (never invent bytes); the recorded hour undercounts at most by the skipped source's true traffic.
- The meters advance independently and are collected concurrently. A transient egress failure therefore cannot hold build/instance/storage metering back, and vice versa.
- The existing 48-hour catch-up bound still applies and is clamped to the App's creation hour, so a new service never gains synthetic pre-creation coverage. Outages longer than 48 hours are visible as gaps rather than silently synthesized as zero.

Successful zero egress/build rows are coverage anchors in `usage_hourly`; the period aggregation omits their all-zero groups so REST/GraphQL/MCP response semantics remain unchanged. `instance_seconds` retains zero groups for tiered suspended services, as before. Any analysis that needs to prove collection completeness must query raw hourly rows from the deployment time of this corrected contract; older positive-only rows are consumption evidence, not coverage evidence.

The loop iterates `ListApps` (App services) and then all `Database` and `KeyValue` CRs in the operator's namespace — both using the `bex.co/tenant` label to identify the owning workspace. Database/KeyValue CRs use their immutable typed CR metadata name (`dpg-…` / `red-…`) as `service_id`; the former name-keyed fleet shapes were normalized and retired by w1/m56 ([docs/ADR020-identifiers.md](ADR020-identifiers.md)). For each closed datastore window, the collector averages the kubelet used-byte gauge across the hour, sums all matching PVCs (including CNPG replicas), converts bytes to decimal GB, and multiplies by elapsed seconds. Public datastore egress is collected from the exact proxy counter in the same window; private datastore egress is successful zero. Per-meter catch-up cursors let storage and egress backfill independently for up to 48 hours and idempotent upserts prevent double-counting when a window is retried.

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
| `period` | Calendar month as `YYYY-MM`. Defaults to the current month. A value that is not a well-formed `YYYY-MM` is rejected with `400` on all three surfaces rather than silently answered with the current month; a well-formed month that is not in the past (the current one, or a future one) resolves to "as of now", and the response echoes the period actually used. For a past month the full month is returned; for the current month, data up to now is returned. Months older than the hot window are served from monthly aggregates with identical totals (see Retention above). |

Response:

```json
{
  "workspaceId": "tea-abc123",
  "period": "2026-07",
  "coverage": {
    "state": "partial",
    "through": "2026-07-18T14:00:00Z",
    "degradedSources": ["direct"]
  },
  "services": [
    {
      "serviceId": "srv-xyz456",
      "serviceName": "my-web-app",
      "resourceKind": "service",
      "rows": [
        { "kind": "instance_seconds", "tier": "starter", "total": 14400 },
        { "kind": "egress_bytes", "total": 2048 },
        { "kind": "build_seconds", "total": 120 }
      ]
    },
    {
      "serviceId": "mydb",
      "serviceName": "shared",
      "resourceKind": "postgres",
      "rows": [
        { "kind": "instance_seconds", "tier": "basic-256mb", "total": 3600 },
        { "kind": "storage_gb_seconds", "total": 7200 }
      ]
    }
  ]
}
```

`tier` is omitted on the JSON response when it is the empty string (non-compute meters). `resourceKind` identifies the resource type (`"service"`, `"postgres"`, `"key_value"`). `services` is always a JSON array (never `null`). `serviceName` is the resource's user-facing display name, resolved best-effort at read time (Apps from the control-plane store, datastores from their CR spec); it is omitted when the resource no longer exists — presenters fall back to `serviceId`.

For the current calendar month, `coverage` reports the evidence behind the totals rather than changing or hiding them. `complete` means every expected hourly source stream is explicitly healthy through the latest closed UTC hour. `partial` means a source is degraded/unavailable, begins late, contains an internal gap, or has not caught up; `through` is the end of the common contiguous healthy prefix and `degradedSources` is a sorted, bounded public vocabulary (`instance`, `build`, `storage`, `http`, `websocket`, `direct`, `postgres`, `key_value`, `sandbox`, with `other` as the overflow sentinel). `unknown` means no post-contract source evidence exists; legacy usage rows are never promoted to healthy evidence. An empty totals result with `complete` is therefore a proved healthy empty result, while a store/query failure remains unavailable. Historical periods always report `unknown` because compacted or legacy consumption cannot establish hourly completeness.

The evidence is durable in `usage_source_streams` plus `usage_source_health`. Successful quantity and health writes are one transaction, including explicit zeroes; a failed multi-source attempt records its full applicable source set atomically as unavailable and a successful retry replaces those states. Each stream starts from the resource's real creation hour, never its first observation. Deleted-resource streams receive a finite expected end and stop constraining the live watermark only after their own expected range is fully evidenced. Source-health rows age out with the same hot-window compaction boundary as hourly usage. Sandbox compute currently uses its separate durable lifecycle cursor; any current-month sandbox total conservatively forces otherwise-known coverage to `partial` with source `sandbox`.

### GraphQL

```graphql
query Usage($period: String) {
  usage(period: $period) {
    workspaceId
    period
    coverage {
      state
      through
      degradedSources
    }
    services {
      serviceId
      serviceName
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

## Observability reads vs billing reads (w1/m50)

The egress health gate (`internal/egressquery.Health()`) exists for **billing integrity**: it is a product of window-wide terms — unbroken `up`, full scrape density, source health-metric = 1 throughout, `resets(counter[W]) == 0`, process started before the window, and (direct) no counter-loss event in-window — and the hourly rollup refuses to record a resource-window when any term fails, because `increase()` over a reset can lose or (after a direct-meter checkpoint restore) invent bytes. That refusal is correct for metering: a skipped hour is retried/deferred, never fabricated.

The same gate was originally applied verbatim to the **interactive reads** — `Metrics(BANDWIDTH)` and `monthToDateBandwidth` — where it is structurally wrong. Prod evidence (2026-07-18, the w1/m50 investigation): over the dashboard's default 12h window, the **direct** source failed with `up{job="bex-egress-meter"}` min = 0 and **26 of ≥2304 expected samples** (the single meter instance was absent through tenant-node churn — one tenant node cordoned leaves DaemonSet DESIRED=1; the DS template also sat at generation **67 after 45h** because it rolls with every image pin, opening an `up` gap per deploy), and the **http** source failed with **32 in-window `traefik_router_responses_bytes_total` resets** (Traefik recreates router counter series at zero on every app deploy's Ingress reload — a month-to-date window can never be reset-free). One failed term errored the entire read (`egress source X unhealthy`), so the Outbound Bandwidth panel showed "No data in range" essentially always on prod, and `monthToDateBandwidth` zeroed out.

**Decision — split the consumers (w1/m50/t002, shape "degraded-not-fatal"):**

- The **usage rollup keeps the strict gate unchanged** (`internal/usage/service.go` `queryEgressSources`): unhealthy ⇒ the window is not recorded. Never invent bytes for billing.
- The **interactive reads become best-effort**: per-source health is still computed with the same `Health()` query, but a failing source no longer errors the read — the series/sum is served anyway (`rate()`/`increase()`'s native reset handling applies: it can undercount around a gap or reset, and after a direct-meter checkpoint restore may briefly overcount; both are chart-grade, not billing-grade) and the failing sources are reported **as data**:
  - `Metrics(BANDWIDTH)` attaches a `degraded_sources` label (comma-joined source names, e.g. `direct,http`) to the returned series — riding the existing `labels` array on REST/GraphQL/MCP identically, no schema change.
  - `monthToDateBandwidth` gains a `degradedSources: [String]` field.
- The dashboard renders three distinct states: data (with a degradation annotation naming the source when `degraded_sources` is present), a genuinely healthy empty window, and a real query error — never a gate refusal masquerading as "No data in range".

Chosen over per-resolution-bucket health gating (honest but N× the instant queries and holes in the chart) and over dropping unhealthy sources from the sum (hides real traffic that the partially-present source did record).

**Prod verification (w1/m50/t006, 2026-07-18, after the `119800de` roll):** the reference service's Outbound Bandwidth panel renders a 145-point series over 12h (0–15.3 KiB/s) with the "Partial data" badge naming the direct source (that window contained the deploy's own meter-DS roll), and a 121-point series over 7d with all three sources named (a week necessarily contains resets and gaps) — both windows that were "No data in range" before. `monthToDateBandwidth` returned **83.1 MB as data** with `degradedSources: [http, websocket, direct]` where it previously returned zeros plus an error; the figure matches the app's plaintext-HTTP router's own Prometheus month increase (83.4 MiB) almost exactly — the read is faithful to its matcher. The July usage rows were **byte-identical** before and after the deploy (billing untouched). Screenshots: `.playwright-mcp/m50-bandwidth-{12h,7d}-partial-data.png`. The sanity check also exposed two pre-existing defects, filed as `w1/034`/`w1/035` and **fixed by w1/m51**: (a) the hourly egress rollup recorded no `egress_bytes` rows on prod at all because any unhealthy source deferred the hour forever — replaced by per-source hour gating (see § Successful zeroes above); (b) `core.TraefikRouterNamesForIngress` missed Traefik's TLS sibling router, so every bandwidth/egress consumer counted only the 80→443 redirect bytes. Ground truth (w1/m51/t001, Traefik v3.7.5, chart 41.0.0, prod label inventory 2026-07-18): with entrypoints `web`/`websecure` and no entrypoint annotation, an Ingress rule yields the base router `<ns>-<name>-<host>-<path>@kubernetes` plus, for every `spec.tls`-covered host, the sibling `websecure-<same>@kubernetes` — exactly two shapes exist on prod, and the sibling carries essentially all real bytes. The resolver now emits both (sibling derived from the final, collision-hashed key), so the chart, `monthToDateBandwidth`, the http health source, and the billing meter all inherit HTTPS coverage through the one shared matcher. Bandwidth/egress history before 2026-07-18 undercounts (redirect-only); it is noted, not rewritten.

**w1/m51 prod verification (2026-07-18, after the `fdbfb173` roll):** the reference service's 1h interactive bandwidth read integrated to **1424.3 MiB** against a trusted reset-free Prometheus `increase()` of **1418.3 MiB** over the same routers (0.4% apart — rate-vs-increase boundary extrapolation), where the pre-fix chart averaged ~2 KB/s; month-to-date jumped 83.1 MB → 84.6 GB (the long-window figure carries documented reset inflation and its `degradedSources` annotation). The rollup cursor — frozen at 2026-07-16 09:00 with zero non-zero rows platform-wide — caught up to the last closed hour on the first post-deploy pass, writing **44 non-zero hours (69.75 GB)** for the reference service alone; the per-source gate recorded quiet hours and skipped reset hours' http contribution. The pre-existing frozen row and all instance/storage totals were byte-identical after the roll. Screenshot: `.playwright-mcp/m51-bandwidth-https-12h.png`.

**Companion platform decision (w1/m50/t005) — accept the meter's deploy rolls.** The egress-meter DaemonSet ships in the shared bex-operator image, so every image pin rolls it (the generation-67 amplifier above). A separately pinned meter tag was considered and rejected: the image is the one multi-entrypoint artifact by design (`lego/` layout), the meter's counters are node-persistent across pod restarts (pinned BPF map + `/var/lib/bex-egress-meter` checkpoint — a roll's only cost is the scrape gap), and post-split that gap degrades to a "Partial data" annotation instead of a blank panel. Scheduling coverage is likewise deliberate and unchanged: the DS carries no tolerations because production tenant Apps can only land on the untainted tenant pool (`config/egress-meter/daemonset.yaml`'s comment) — platform/control-plane nodes need no meter. Revisit only if tenant placement ever widens.

## Pre-declared drift from Render

Render's public REST API and GraphQL surface have no usage or billing endpoints (billing is dashboard-only; verified against Render's OpenAPI spec 2026-07-09 — no `/usage`, `/billing`, or equivalent exists). This surface is therefore a bex extension, and its **API names are stable**: REST `/v1/usage`, GraphQL `usage`, MCP `get_usage`. The **dashboard page**, originally `/usage`, was deliberately renamed to `/billing` in w5/m70 (2026-08-17) to match Render's workspace IA once the page carried payment onboarding, collection lifecycle, and credits alongside metered usage — a front-of-house rename only; `/usage` remains a redirect and the wire surface is untouched. Render's Postgres storage rate is `$0.30/GB-month`, prorated by the second, but applies to provisioned capacity; bex applies a 30%-lower `$0.21/GB-month` estimate to actual used bytes. Render has no independent Key Value storage charge; bex applying the same transparent used-storage rate to Valkey is a deliberate extension. See [docs/ADR018-render-parity.md](ADR018-render-parity.md) § bex ahead of Render.
