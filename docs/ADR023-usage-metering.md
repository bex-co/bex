# Usage metering

bex records month-to-date resource consumption per workspace and exposes it over REST, GraphQL, and MCP so any client — curl, the dashboard, or an MCP agent — can read the same numbers (pillar 1: API-first; pillar 3: agents as operators).

## Five meters

| Meter | Unit | Source |
| --- | --- | --- |
| `instance_seconds` | seconds (per tier) | cAdvisor container-presence signal via Prometheus — per-pod running time: 15 s steps at which each pod had reported within 59 s (containers collapsed per pod), × 15 s |
| `egress_bytes` | bytes | loss-detecting sum of exact App HTTP + WebSocket + direct-public sources, or the public datastore proxy response source |
| `build_seconds` | seconds | build and pre-deploy runs that ended in the window — Jobs and kpack Builds, failed ones included — from pod placement to their end (§ `build_seconds` against Render's pipeline minutes) |
| `storage_gb_seconds` | decimal GB-seconds | average `kubelet_volume_stats_used_bytes` over the window × window seconds, summed across a datastore's PVCs |
| `sandbox_memory_gb_seconds` | GiB-seconds | **Shadow (w22/m313): recorded, not billed.** Allocated GiB of the sandbox shape × `running` seconds, from the same phase cursor as `sandbox_compute_seconds`; billing switches to it with the CPU meter (ADR104 D8, w22/m314) |
| `sandbox_snapshot_gb_seconds` | decimal GB-seconds | **Event-driven (w22/m313):** compressed image bytes of a suspended sandbox's retained rootfs image × the seconds it is retained; § Sandbox snapshot storage |
| `sandbox_compute_seconds` | milli-vCPU-equivalent seconds | OpenSandbox lifecycle phase × shape; CPU millicores plus memory GiB folded at the AgentCore `$0.00945/GB-hour ÷ $0.0895/vCPU-hour` reference ratio; only `running` intervals accrue |
| `agent_model_{input,output,cache_read,cache_write}_tokens` | tokens (tier = model id) | platform model route only (w10/m14, ADR090): the gateway model proxy reads Anthropic usage from each platform-key exchange and reports it once per request id; `resource_kind` `agent_session`, `service_id` the session |
| `agent_model_cost_microusd` | micro-USD (customer price) | platform model route ([ADR091](ADR091-agent-model-pricing.md) D3): provider cost per request (OpenRouter-reported, else computed from its price table) × `agentModel.markup` (1.30), rounded half-up per request; the one billed agent-model meter |

The first three meters match Render's compute, bandwidth, and pipeline-minute dimensions. Storage adds the separately-priced Postgres dimension while remaining API-first. Render charges provisioned Postgres capacity; bex meters actual used PVC bytes, so the rate is comparable but the usage basis is deliberately more usage-sensitive. Render has no separate Key Value storage line; bex exposes the same storage meter for Valkey because it also owns a persistent volume.

### `build_seconds` against Render's pipeline minutes (2026-11-10, w6/m307)

Render counts every pipeline task against pipeline minutes: the build command, a Dockerfile image build, and the pre-deploy command ([build pipeline](https://render.com/docs/build-pipeline)). [ADR030 §13](ADR030-pricing.md) brings in Render's included minutes, which makes the meter's basis matter. `queryBuildSeconds` (`internal/usage/build_seconds.go`) bills a run in the hour it ended:

- **Every build that ran, failed ones included.** A Job ends at its `completionTime`, which Kubernetes sets on success only, or else at its `Failed` condition. A build that failed for a platform reason bills nothing. That is the operator's verdict when it stamped one on the Job (`app.bex.co/build-fault`, which also covers its overrides, such as a toolchain bex supplies failing to arrive), else the Job's own failure reason (`BuildJobFault` in `lego/types`: used-up retries are the platform's). A tenant failure or a timeout bills like a success.
- **From when the build pod was placed.** The operator stamps the first placement on the finished Job (`app.bex.co/build-placed-at`). The Job's own `startTime` comes earlier by the wait for a build node, which is the platform's time; it is the fallback for a Job the stamp missed.
- **Pre-deploy runs.** The App's pre-deploy Jobs (`app.bex.co/predeploy`, in the App's own namespace) count too, from their `startTime`. A failed one always bills: it has no pod failure policy, so its failure reason says nothing about fault. They carry the release revision (`app.bex.co/build-revision`), so a platform-opened release's pre-deploy run is exempt like its build (w1/m184).
- **Buildpack builds.** A `builder: buildpack` release builds through kpack, which runs Builds rather than Jobs and copies the operator's labels from the release's Image onto each Build and its pod. Each of the App's Builds counts from its pod's start (its creation, once the pod is gone) to its terminal `Succeeded` condition. bex-api may read kpack Builds in the build namespace for this (`build-boundary.yaml`).

Residuals: time that leaves no object behind bills nothing. A build canceled while running is deleted with its Job; a kpack Build goes with its Image when the next release's build is stored; and a finished Job lives an hour (its TTL), so a rollup that runs later than that misses it.

### Complete outbound-bandwidth contract (w8/m15)

`egress_bytes` is a composition, not a synonym for one edge metric. An App row sums every applicable App source; a public Database or Key Value row reads its metered public proxy. Private datastores have no public-egress source and persist a successful zero. The collector never returns a partial sum: required-source absence, health loss, or an ambiguous counter decrease leaves the hour absent and holds that meter's cursor for retry.

The HTTP component resolves a control-plane App row to its projected CR through the immutable `bex.co/app-id` label, reads that CR's actual operator-owned Ingress, and reproduces the router-key algorithm from the production-pinned Traefik v3.7.5. Prometheus selectors are anchored to the complete `router@kubernetes` labels; display-name substrings and backend Service names are never used for bandwidth attribution. This keeps ordinary services, custom domains, and shared-backend static sites separate. An App without public hosts has no Ingress and records a successful zero: a private App, or an exposed one whose subdomain is disabled and whose last custom domain was deleted (`AppSpec.HasPublicHosts`, w5/112). A missing projected CR, missing expected public Ingress, unsupported Ingress shape, ambiguous projection or cross-Ingress router label, Kubernetes failure, or Prometheus failure writes no row and is retried by the per-meter cursor.

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
| `build_seconds` | ✅ build and pre-deploy runs (Jobs, kpack Builds), placement → end | — no build step | — no build step | — no build step |
| `storage_gb_seconds` | — stateless App storage is not a supported product surface | ✅ CNPG PVCs (`<name>-<n>`) | ✅ Valkey PVC (`data-<name>-<n>`) | — a different dimension: a suspended sandbox's image is `sandbox_snapshot_gb_seconds` |
| `sandbox_compute_seconds` | — | — | — | ✅ durable OpenSandbox phase cursor × weighted shape |
| `sandbox_memory_gb_seconds` | — | — | — | ✅ shadow: allocated GiB × `running` seconds, same cursor |
| `sandbox_snapshot_gb_seconds` | — | — | — | ✅ retained rootfs image bytes × seconds suspended |

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

**Instance running time (w4/m173).** Until this correction, `instance_seconds` was `count(avg_over_time(container_memory_working_set_bytes{…}[1h])) × 3600`. That counted every container series seen at any moment of the hour and billed each one a full hour. A pod alive five minutes read 3600, a Key Value's Valkey plus exporter sidecar read 7200, and each rollout's replacement pod added another 3600. The meter now evaluates `sum(count_over_time((max by (pod) (count_over_time(…[59s])))[3599s:15s])) × 15`, which is each pod's own presence at the cAdvisor scrape step, so one pod is one instance.

- **Liveness (w5/m111).** A step counts a pod only if it reported within the previous 59 s.
  - cAdvisor samples carry their own timestamps, so Prometheus writes no staleness marker when a pod is deleted (w4/m110).
  - A bare instant selector would have carried the last sample forward for the 5-minute lookback. That billed every terminated pod about 285 s more than it ran: every rollout, scale-down and pod replacement.
  - The samples are stamped at kubelet housekeeping (10 s, jittered up to 2×). Production measured a 44.3 s maximum over 53,039 live gaps (2026-10-27), which broke the first 44 s bound, so the bound is four scrape intervals less one second.
  - Any bound from 45 to 59 s credits a terminated pod the same three steps, so 59 s costs nothing over a tighter one.
  - 59 s sits off the 15 s grid, so Prometheus 2.x's closed and 3.x's left-open ranges agree.
- **Accuracy.** Within three steps per pod.
  - A deleted pod is still counted for up to 45 s (three steps) after its last sample.
  - A gap in stored samples longer than 59 s (Prometheus down) under-counts the excess.
  - The 3599 s range avoids the aligned left endpoint Prometheus 2.x includes, so consecutive hours partition as (start, end].
- **Verified** in Prometheus's engine (`TestInstanceSecondsQueryOnPrometheus`, v2.54.1 and v3.14.0), with series ending the way cAdvisor's do (no staleness marker):
  - 5 minutes reads 330;
  - a sidecar pod reads 3600;
  - two replicas read 7200;
  - a rollout with a 2-minute overlap reads 3765;
  - gaps of up to three missed scrapes lose nothing, while four lose one step;
  - a restarted container's overlapping new series continues its pod once.
- **Cut-over.** Hours metered after each deploy use that deploy's query, so `usage_hourly` holds two over-counted cohorts. Rows from before w4/m173 billed every container series a full hour. Rows from the w4/m173 query (live from pin `f837cec46294` until the deploy that ships w5/m111) billed each terminated pod about 285 s of lookback. Whether to recompute paid-tier hours, and to repair Stripe meter events already sent for them through ADR040's reconcile/repair path, is an open decision (w4/m173 t002). Paid exposure must first be quantified with a read-only production query.

Successful zero egress/build rows are coverage anchors in `usage_hourly`; the period aggregation omits their all-zero groups so REST/GraphQL/MCP response semantics remain unchanged. `instance_seconds` retains zero groups for tiered suspended services, as before. Any analysis that needs to prove collection completeness must query raw hourly rows from the deployment time of this corrected contract; older positive-only rows are consumption evidence, not coverage evidence.

The loop iterates `ListApps` (App services) and then all `Database` and `KeyValue` CRs in the operator's namespace — both using the `bex.co/tenant` label to identify the owning workspace. Database/KeyValue CRs use their immutable typed CR metadata name (`dpg-…` / `red-…`) as `service_id`; the former name-keyed fleet shapes were normalized and retired by w1/m56 ([docs/ADR020-identifiers.md](ADR020-identifiers.md)). For each closed datastore window, the collector averages the kubelet used-byte gauge across the hour, sums all matching PVCs (including CNPG replicas), converts bytes to decimal GB, and multiplies by elapsed seconds. Public datastore egress is collected from the exact proxy counter in the same window; private datastore egress is successful zero. Per-meter catch-up cursors let storage and egress backfill independently for up to 48 hours and idempotent upserts prevent double-counting when a window is retried.

**Deleted resources keep their final hours (w4/m173 t007).** Those lists hold only what still exists, so a resource deleted before its hour was rolled up was never metered: every short-lived service or datastore read as no row at all, and every deleted resource lost its final partial hour. Migration `0151_usage_retired_resources` keeps `usage_retired_resources` for this.

- **What fills it.** An `AFTER DELETE` trigger on `apps` writes a row on every App deletion path: the delete verb or a workspace cascade. The table has no foreign key, so that cascade cannot fail. `RollbackAppCreation` discards the row, because a create that never completed owes nothing. `DeletePostgres` and `DeleteKeyValue` record the CR's identity, namespace, display name, public flag, plan and creation time **before** deleting it. If the record cannot be written they refuse the delete as unavailable.
- **How it is metered.** Each rollup and catch-up meters these rows through the hour of deletion, using the same per-meter cursors and the same 48-hour floor from the current window.
  - **Apps** are metered for `instance_seconds` and `build_seconds` by their derived pod identity.
  - **Apps' `egress_bytes` too (w6/m333).** Their final hours' bandwidth is billed, as on Render ([outbound bandwidth](https://render.com/docs/outbound-bandwidth)). The HTTP counter is keyed by Traefik router names that exist only while the CR and its Ingresses do.
    - So each catch-up pass resolves an App's egress identity once and uses it for every window. The identity is the router names and the direct-egress flag.
    - The pass records the identity in `usage_egress_identities`, migration `0233_usage_retired_egress_identity`. These are observed cluster state, so they live beside the `apps` row, not on it; an unchanged identity writes nothing.
    - The retired pass reads the identity with the retired record and meters egress with the same per-source health gating as a live App. Pruning the record removes the identity.
    - The direct and WebSocket counters are keyed by the App's id. The router names are covered by the same-name skip below.
    - **Deletion records it too (w6/121).** The delete verb resolves the routers while the CR and Ingresses still exist and records them, so an App created and deleted between two hourly rollups is still metered. This is best-effort: a failed resolution or write never blocks the delete. A workspace deletion's cascade does not go through the verb, so its Apps rely on the hourly record. An App with no identity from either path keeps its final-hour egress unmetered, as before.
  - **A same-name App** created by the deletion hour (one that is live, or deleted later) is already billing those same pods under its own ID, so the deleted App is skipped.
  - **Datastores** go through the live datastore pass, so their display names are retained and all three meters run.
- **Coverage.** Each retired resource's coverage streams close at the end of its deletion hour.
- **Creation floor.** Datastore catch-up now also starts no earlier than the creation hour, like Apps, so a new or short-lived datastore no longer back-fills pre-creation zero rows.
- **When rows go.** Retention compaction drops a row once it is past the catch-up window or its workspace is gone. Compaction runs with or without Prometheus.
- **Limit.** Workspace purges delete datastores without a record. Their usage rows go with the workspace anyway.

With `BEX_CP_DB_URI` set but `BEX_PROM_URL` absent the service is wired (the month-to-date read still works) but the metering side of the loop is skipped — only the retention compaction below runs.

## Sandbox snapshot storage (w22/m313, ADR104 D8)

Render bills a suspended sandbox for its snapshot storage only. bex used to absorb the retained image for up to 30 days. `sandbox_snapshot_gb_seconds` is that charge: decimal GB (10⁹ bytes) × seconds, the same unit as `storage_gb_seconds`.

**What is metered.** One object per suspended sandbox: the rootfs image its pause committed. The object is open while the sandbox's phase cursor reads `suspended` and closes when the cursor leaves it: a resume, a terminate, or the controller's expiry at pause + 30 days (the sandbox then drops out of the complete tenant list and the missing-cursor path terminates it). A resumed, running sandbox pays compute and no storage, so nothing is charged twice for one sandbox. Snapshot objects (w32/m285) use the same table and the same `start`/`end` calls with `kind = 'snapshot'`.

**Byte source (measured 2026-11-10).** The OCI image manifest in Zot, read by tag: `config.size + Σ layers[].size`, compressed blob bytes, which is what the registry stores. The controller's `SandboxSnapshot` status carries `imageUri` and `imageDigest` and no size; the tag is `snap-gen<N>` for a sandbox-owned snapshot and the repository is `snapshots/<ws>-sandbox/<sandbox>-<container>` (the carried patch and the pinned controller source). Zot's search extension, which would also report a size, is not enabled by the operator's config (`extensions` holds only `scrub`) and answered 404 on a local Zot v2.1.22 run with that config; the distribution API needs no config change. A local run (Zot v2.1.22, bex's storage and retention policy, three pushed images) read: generation 1 = 25,071,309 B (config 877 + layers 4,092,319 and 20,978,113), generation 2 = 30,345,793 B (the same two layers, plus 5,244,676 and a 30 MB zero file that compresses to 29,310), and the same generation 1 pushed under a second sandbox's repository = the same digest and 25,071,309 B. The 30 MB of zeros added 29,310 B: the figure is compressed bytes, not the rootfs size a customer sees in `df`.

**Which tag.** The highest `snap-gen<N>`: the image a resume would pull. Zot retention keeps the five most recent tags per repository (and unreferenced layers until garbage collection), so older tags exist; they are bex's retention overhead, bounded by D11 and not billed.

**What is double-counted, and why.** Billing sums each object's own manifest bytes. In the local run the three tags sum to 80,488,411 B against 30,348,627 B of unique stored content, 2.65×: layers shared between a sandbox's successive commits are counted once per billed object because only the latest tag is billed (gen 2 = 30,345,793, gen 1 not), and the base layer shared between two sandboxes' images is counted in each, because each customer pays for the size of what they kept, as on Render, and bex's cross-repository dedupe is its own saving. That is a stated choice, not an accident.

**Event contract.** A durable row per `(workspace_id, object_id)`: kind, the registry ref, bytes (NULL until sized), `started_at`, `accrued_through`, remainder, `ended_at`, and whether its image waits for the reaper.

- `start(object, at)`: idempotent. An existing open row, or one whose `started_at` is not older than `at`, is left alone; a closed row older than `at` reopens (a sandbox that was resumed and suspended again). The phase cursor drives it: the transition into `suspended` calls it in the same transaction that advances the cursor, so a replayed observation changes nothing.
- `end(object, at)`: idempotent; closes an open row at `at`.
- `SetBytes(object, started_at, bytes)`: fills the size once it is read from Zot, and never changes it afterwards. `started_at` is the first observation of `suspended` (the poll's 30 s granularity, the same attribution error as compute); accrual runs from `started_at` as soon as the size is known, so a Zot outage at suspend time delays the charge and does not lose or invent seconds. A row that closes still unsized is counted by `bex_sandbox_storage_unsized_total` and never billed as zero silently.
- Accrual advances `accrued_through` to `min(now, ended_at)` in one transaction with the `usage_hourly` upserts, split at UTC hour boundaries, with the sub-unit remainder carried in byte-seconds so the 30 s cadence cannot round usage down. Replaying a poll cannot add a second. The unsized queue rotates on the time each object was last tried, so an image whose size cannot be read does not hold the head of the queue; a round's registry calls are bounded to 20 s and end after three consecutive failures, so a hung registry cannot delay the next poll of every workspace. Accrual runs per object every 15 minutes (a closed object settles at the next pass), and rows settled for 35 days are pruned, so the table follows the objects with work left and every meter read is served by a partial index.

**Left out.** Agent-session sandboxes (ADR047) are not metered for storage: their cost is the session's, priced on compute (D6), and they are torn down by the agent-sessions feature, not idle-suspended. Whether a session's retained image should be billed is a separate decision, filed as w22/005.

**Bounds and deletion (ADR104 D11).** A workspace's retained bytes are capped (`BEX_SANDBOX_MAX_STORAGE_GB_PER_WORKSPACE`, default 500 decimal GB, 0 disables): past it a suspend, explicit or idle, answers 429 `SANDBOX_STORAGE_LIMIT` and the sandbox keeps running. The cap reads the open, sized objects, so a workspace can pass it by one image, and a failed read never refuses a suspend. The cap is a divergence from Render recorded in ADR018, awaiting sign-off. Zot's retention keeps a repository's five newest tags forever, so a terminated or expired sandbox's images would stay in the registry; an object that closes because its sandbox is gone (terminated, or dropped from the complete tenant list at pause + 30 days) is queued (`reap_pending`, including a sandbox that was resumed and later terminated, whose old tags are still there) and a reaper deletes its `snap-gen<N>` tags by tag, never by digest (a digest delete removes every tag on the manifest; checked on Zot v2.1.22), after which Zot's own retention and blob GC reclaim them. A resume does not queue its image: the pod may still be pulling it. The reaper needs a credential with the delete action on the snapshot prefix (`BEX_SANDBOX_SNAPSHOT_REGISTRY_GC_USER`); without one the images stay and `SandboxStorageReapBacklog` says so. The meter's own health is `bex_sandbox_storage_objects{state}`, `bex_sandbox_storage_bytes` and two failure counters, with four alerts (`SandboxStorageUnsized`, `SandboxStorageUnsizedClosed`, `SandboxStorageReapBacklog`, `SandboxStorageGrowth`) and two panels on the platform dashboard.

**Reading Zot, and who holds the credential (w22/m321).** bex-api reads manifests with a read-only Zot user and deletes a gone sandbox's tags with a separate read-and-delete user, both on `snapshots/**` and nothing else; neither is `bex-builder`, which holds Zot's admin policy. The operator mints them (`BEX_SANDBOX_METER_CREDS`: `bex-sandbox-meter` with `read`, `bex-sandbox-reaper` with `read` and `delete`, a per-user policy on the shared pattern so neither replaces the other) into the Secret `bex-sandbox-registry` **in the registry namespace**, where it already manages Zot's Secrets. That placement is deliberate: its manager caches Secrets only in the apps namespace and its ClusterRole has no Secret access in `bex-system` on purpose, so minting there would have widened it. bex-api, in `bex-system`, cannot reference a Secret across namespaces, so it reads this one Secret by name through a Role limited to `get` on that name (`deploy/gitops/base/bex-api-sandbox-registry-rbac.yaml`, guarded in `scripts/gitops-validate.sh`), at most every five minutes, so a rotated password is picked up and a brief API outage serves the last good one. Deleting follows the Secret: with it bex-api deletes a gone sandbox's tags with the reaper user (there is no separate switch; checked 2026-11-11 on production, where the reader can neither delete nor read outside `snapshots/**` and the reaper can delete there and nowhere else); the user and password variables remain as an override for a registry the operator does not manage. With no registry URL nothing is sized and nothing is billed: the feature is off, not wrong.

**Checked on production (2026-11-11, after the deploy of `7ca0227aa`).** The Secret `bex-registry/bex-sandbox-registry` holds the reader and reaper users. Through a `port-forward`: the reader lists the catalog (200), cannot delete (403) and cannot read a repository outside `snapshots/**` (403); the reaper lists it, deletes inside `snapshots/**` (404 for a tag that does not exist, so the delete is authorized) and cannot read outside it (403); an anonymous catalog read is refused (401). The repository layout is `snapshots/<ws>-sandbox/<sandbox id>-sandbox` with `snap-gen<N>` tags, and `config.size + Σ layers[].size` of the newest tag gives 3.8 to 4.9 MB for the eight existing repositories (two layers each): a suspended sandbox's image is its changed layers, not the base image, so a typical charge is a fraction of a cent a month. Those eight predate the meter, so no row tracks them and the reaper never deletes them. **That check missed a gap, and the first real reap found it.** Zot applies only the longest matching pattern, and production holds 71 per-workspace patterns `snapshots/<ws>-sandbox/**` (the namespace's own read-only `snap-<ws>-sandbox`, made by the operator for resume pulls), each of which shadows `snapshots/**`: for those workspaces the reader and the reaper were refused (403) on `tags/list`, so nothing in them was sized or deleted, and the eight repositories above were readable only because their workspaces have no pattern of their own. The six QA objects were retried every poll and failed with 403 until this was fixed. The operator now puts the two users on every `snapshots/<ws>-sandbox/**` pattern beside the namespace's own user, both when it creates one (`EnsureSnapshotCreds`) and for the existing ones (`EnsureSandboxMeterCreds`, which re-ensures every ten minutes), and both writers converge on the same three policies so neither strips what the other adds. Confirmed on a real Zot `v2.1.22` with both shapes side by side: a pattern with only the namespace user answered the reader and the reaper 403, including a delete; with the two users added the reader got 200 on tags and manifest and 403 on delete, the reaper got 200 and 202 and the tag was gone, and one namespace's user still got 403 on the other's repository. The meter's own rows that day were six QA sandboxes suspended for 7 seconds to 8 minutes, none with an image in the registry, which is why the unsized-closed count ignores an object open for less than `StorageUnsizedGrace` (30 minutes).

## Retention: hourly detail compacts into monthly aggregates

`usage_hourly` accrues one row per resource × meter kind × tier × hour forever, so the same loop bounds its growth (w8/m4): once a calendar month falls out of the **hot window**, a daily compaction pass folds its hourly rows into the `usage_monthly` table (migration `0007_usage_monthly.up.sql`) — one row per `(resource_kind, service_id, kind, tier, month)` — and purges the compacted hourly detail.

`BEX_USAGE_RETENTION_MONTHS` sets the hot window: how many calendar months (current month included) keep full hourly detail. Default **3** (this month + the prior two — the common historical-comparison range), minimum 1; a very large value effectively disables compaction.

Properties of the compaction pass:

- **Lossless for totals.** Compaction is a plain `SUM` per calendar month, not a sample: `period=` queries return exactly the same totals before and after a month is compacted. Only sub-month (hourly) resolution is lost, and only for months older than the hot window.
- **Transparent to the API.** The period query sums `usage_hourly` and `usage_monthly` together, so REST/GraphQL/MCP answers are correct whether a month is still hot, already compacted, or (never yet) compacted — correctness never depends on the compaction having run.
- **Atomic and idempotent.** The aggregate-insert and hourly-purge are a single statement, so a crash can never leave the two tables inconsistent, and re-running over the same window is a no-op. Straggler hourly rows compacted by a later pass _add_ to their month's aggregate.
- **Never races the rollup.** The compaction boundary is clamped to 48 hours ago (the rollup's restart catch-up limit), so a window the metering loop might still rewrite is never compacted underneath it.
- **The agent model request ledger stays longer.** `agent_model_usage_events` (one row per platform-route model request, with its cost, cost source and price) is not purged with the hourly rows it fed. The same pass deletes events older than **13 months** before the compaction boundary, so per-turn session cost and the invoice dispute trace survive the invoice period plus a dispute window ([ADR091](ADR091-agent-model-pricing.md) D6, D10.7; w10/m14 t013; migration 0199 adds `started_at`, the request's validated start, and indexes `(session_id, recorded_at)`). The deletion cutoff is computed in UTC calendar months.

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

`tier` is omitted on the JSON response when it is the empty string (non-compute meters). `resourceKind` identifies the resource type (`"service"`, `"postgres"`, `"key_value"`, `"sandbox"`). `services` is always a JSON array (never `null`). `serviceName` uses the current name when available and otherwise the retained name; genuinely lost historical names fall back to `serviceId` in presenters.

**Attribution after deletion (w4/m135).** Services retain names at create/rename, and datastore collection retains names before writing usage. A billing-page visit is therefore unnecessary to preserve a metered datastore's name. If retention fails, that workspace's datastore collection records unavailable coverage and retries through its existing catch-up cursors; other workspaces continue. Read-time capture still refreshes names. `deleted` is independent of whether a name exists: a successful inventory showing an absent service/datastore proves deletion, while a failed read or an unwired Kubernetes client does not. A sandbox's historical session label is attribution, not proof of existence; its durable meter's explicit `terminated` phase supplies deletion evidence. Other phases reflect the last observation and may lag the poller. Where no session or retained label exists, a known tier supplies a descriptive sandbox label. Old rows with neither lifecycle evidence nor a recoverable name remain unknown; no audit-log guess or fabricated historical name is added.

REST, GraphQL, and MCP share this resolution. The existing Charges card renders a deleted marker for either a retained name or a fallback ID. This changes attribution metadata, not usage quantities, prices, or invoice calculations. Render documents accrued monthly charges in its [dashboard billing view](https://render.com/docs/render-dashboard); its public documentation does not specify this historical name-retention contract, so no claim of exact deleted-row UI parity is made.

For the current calendar month, `coverage` reports the evidence behind the totals rather than changing or hiding them. `complete` means every expected hourly source stream is explicitly healthy through the latest closed UTC hour. `partial` means a source is degraded/unavailable, begins late, contains an internal gap, or has not caught up; `through` is the end of the common contiguous _metered_ prefix (a degraded hour was metered and counts, and so does a hole too old to be metered any more; a missing or unavailable hour the rollup can still fill stops it — amended 2026-11-11 for w4/496) and `degradedSources` is a sorted, bounded public vocabulary (`instance`, `build`, `storage`, `http`, `websocket`, `direct`, `postgres`, `key_value`, `sandbox`, with `other` as the overflow sentinel). `unknown` means no post-contract source evidence exists; legacy usage rows are never promoted to healthy evidence. An empty totals result with `complete` is therefore a proved healthy empty result, while a store/query failure remains unavailable. Historical periods always report `unknown` because compacted or legacy consumption cannot establish hourly completeness.

**Degraded hours and the watermark (w4/496, 2026-11-11).** Until this amendment `through` stopped at the first degraded hour, so one skipped egress reading on November 1 showed every workspace's Billing page "complete only through 2026-11-01" for the whole month, although every later hour was metered. A degraded hour is now traversed: it was recorded (the meter moved on), and its reduced confidence stays visible as `degradedSources` and `partial`; only an hour with no evidence or an `unavailable` read that the rollup can still fill — inside the catch-up window (`store.UsageCatchupWindow`, 48 h, the same constant `usage` meters by, plus the disk meter's one extra hour) — stops `through`. An older such hole will never be metered: a deleted App's last egress hours (the deleted-resource pass meters only instance and build seconds), a missed rollup hour. It counts as skipped, like a degraded hour: `through` passes it and its source is reported degraded. The Billing caveat reads "Metered through {date}". Two causes of degradation were found in production: an App that never served (no `ActiveRevision` — first build failed or was canceled, first rollout never ready) had its honest zero instance-seconds marked degraded every hour (four Apps all month); `expectsRunningPods` now treats it as having no pods, and its November rows (1,017 across the four Apps and one probe) were set to healthy on the production database on 2026-11-11 after the fix deployed. An egress source failing its scrape-health gate is skipped for that hour (about a dozen direct-source hours a day, plus two `http`/`websocket` incidents on November 2 and 5). Those hours are not re-metered: the health gate failed because the counters in that window cannot be trusted, and re-reading them later reads the same samples — undercount, never invent. **Guarded since w6/m334.** An hour that stays unmetered is no longer skipped silently.

- After every pass, and once before the startup catch-up, bex-api publishes `bex_usage_oldest_unmetered_hour_timestamp_seconds{source}`. For each source it is the start of the oldest hour, fleet-wide, that the rollup can still meter but has not: the hour the coverage watermark stops at (`store.UnmeteredUsageHoles`). It is 0 when there is none. A deleted App's hole counts only for what is still metered for it: its `store.RetiredAppKinds`, plus its disk, which the disk meter bills from the closed disk period. Its egress without a recorded identity is an undercount that coverage reports, not a hole an operator can fill. **Coverage uses the same predicate (w4/637, 2026-11-14):** `store.RetiredUsageDebts.Unfillable` decides both what the gauge leaves out and which holes the coverage watermark passes at once, rather than holding `through` back for 48 h until the hole settles; such a stream is still reported degraded. A hole a meter still owes (a live App's, or a deleted App's instance, build or disk hour) keeps stopping `through` inside the catch-up window. On production on 2026-11-12 there were 25 holes, all from deletions before w6/m333. 24 were that egress kind. The 25th was a disk hole that should not have existed: a deletion's `CloseUsageSourceStreams` had pushed an already-ended disk stream back out to the deletion hour. A close now never extends a stream that ended earlier.
- `UsageHourNearlyUnmeterable` (critical) fires when that hour closed more than 24 h ago for 10 m. That is half the catch-up window, a day before the hour would be skipped for good, so an outage is fixed while its hours can still be metered.
- The gauge is a start time rather than an age, so the alert keeps ageing from `time()` even if bex-api stops publishing. Its panel is on the billing-metering dashboard. The bex-api scrape job keeps `bex_usage_*` (`deploy/gitops/base/prometheus.yaml`); the first deploy lacked that entry, so the series was never stored.
- A deleted App whose name a new App took over within its last hour owes those hours to nobody: the new App bills the pods. So its coverage now ends an hour after its own last reading (`CloseUsageSourceStreamsAtLastReading`) rather than at its deletion, and those hours never read as a hole.

The evidence is durable in `usage_source_streams` plus `usage_source_health`. Successful quantity and health writes are one transaction, including explicit zeroes; a failed multi-source attempt records its full applicable source set atomically as unavailable and a successful retry replaces those states. Each stream starts from the resource's real creation hour, never its first observation. Deleted-resource streams receive a finite expected end and stop constraining the live watermark once their own expected range is evidenced or settled (w4/496). Source-health rows age out with the same hot-window compaction boundary as hourly usage. Sandbox compute currently uses its separate durable lifecycle cursor; any current-month sandbox total conservatively forces otherwise-known coverage to `partial` with source `sandbox`.

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

**Month-to-date reads are monotonic (w4/468).** The Outbound Bandwidth footer and `monthToDateBandwidth` no longer read the whole month as one `increase()`, whose extrapolation over a window that grows with every read made a flat counter's total shrink for an App created this month. They sum the whole UTC hours that have ended (a `sum_over_time` subquery stepping on absolute hour boundaries, evaluated on the hour, so each hour's term is fixed) and add the current partial hour as an exact counter delta (the counter now minus its value at the hour start, a series that did not exist yet counting from zero; w4/477), because `increase()` still extrapolated over a window that grows with every read and made a flat counter dip within the hour; an App's first hour is the same delta over `[created, now]`. Bytes counted before a counter's first scrape are still not seen. This changes neither billing reads nor the chart.

**Direct source scoped to its own meters (w4/442, 2026-11-01).** The direct source no longer takes the fleet-wide `up` and scrape-density terms. Each node meter holds a persistent, monotonic per-app counter, so a missed scrape loses no bytes: `increase()` across the gap is exact, and only a loss or decrease misstates it. Those are still rejected through `resets()` and the counter-loss time. One missed scrape on any of the fleet's meters had been skipping every app's direct egress for the hour, which billed it as zero. In production that was 21 of 168 hours, with `bex-canary`'s streams degraded in 40–55% of October's hours. The direct gate now reads the meters that exported this app's counter in the window. It requires that their `bex_egress_meter_healthy` stayed 1 with at least 80% of its samples, and that they recorded no counter loss in the window. An app no meter has counted yet falls back to the fleet's health and loss, so a broken meter never passes as an app with no traffic. Replayed read-only on production for 2026-10-31: the example service's healthy hours went from 15 to 20 of 24, and the remaining four are hours when a meter holding its counter reported itself unhealthy. Every scoped term takes its minimum over a subset of the fleet, so no hour the fleet-wide gate accepted is now rejected. Already-recorded degraded hours are not re-metered.

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
