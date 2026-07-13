# ADR: managed PostgreSQL for tenants (CNPG, plans, internal/external URLs)

**Status:** accepted and implemented — the `Database` CRD (`lego/types/v1alpha1/database_types.go`), the CNPG-projecting controller (`lego/operator/internal/controller/database_controller.go`), the `postgres` tier family (`lego/types/tiers/tiers.yaml`), and the bex-api surface (`lego/backend/internal/postgres/`, [ADR006-bex-api.md](ADR006-bex-api.md#managed-postgres-render-v1postgres-compatible)) all ship. The CloudNativePG (CNPG) operator and the control-plane's own DB `bex-db` run on the cluster. Grounded in a live study of Render Postgres (create → connect → delete driven through the dashboard).

## Context

bex is an open-source Render. Render sells **managed Postgres** as a first-class product, and we want parity. Two Postgres concerns must not be conflated:

1. **The control plane's own DB (`bex-db`, exists)** — platform infra, the source of truth for tenants/apps/domains. Internally multi-tenant (a `tenant_id` column). Not a tenant resource.
2. **Tenant databases (this ADR)** — a Render-style "add a Postgres" that a tenant provisions, connects to, and deletes.

What Render actually does (confirmed by creating a real Free instance and connecting to it): a Postgres is a **standalone resource** with a create form (name, database, user, region, **version 13–18**, plan Free/Basic/Pro/Accelerated, storage, HA), and its **Connections** panel exposes two URLs that are the whole product:

- **Internal:** `postgresql://user:pass@dpg-<id>-a/db` — private network, same region, no SSL. For services _inside_ the platform.
- **External:** `postgresql://user:pass@dpg-<id>-a.<region>-postgres.render.com/db` — public FQDN, **SSL required**, plus an IP allowlist. For anything _outside_.

## Decision

### 1. Operator: CloudNativePG (CNPG)

Already deployed, and validated over the alternatives:

- **Governance** — Apache 2.0, CNCF. Rules out KubeDB (freemium) and StackGres (AGPL).
- **Architecture** — native HA via the Kubernetes API (no Patroni/StatefulSet), less to operate than Zalando/Crunchy.
- **Fit** — every axis of the plan design maps to a built-in CNPG feature (below). When the product spec maps 1:1 onto the operator's CRDs, it's the right operator.

The operator is only the _executor_. Plans, per-tenant provisioning, connection-URL assembly, lifecycle, and metering live in **bex's control plane** on top of CNPG — that is the actual build, and it's the same for any operator.

### 2. A standalone `Database` resource → a CNPG `Cluster` in the tenant namespace

Render Postgres is standalone (created independently; multiple services connect to it; it outlives any one app). So model it as a standalone **`Database`** (control-plane row + CR), _not_ `App.spec.database`. The control plane projects it to a CNPG `Cluster` in the **tenant's namespace**, so it inherits that namespace's isolation for free — NetworkPolicy, ResourceQuota, RBAC, and namespace-scoped credential Secret.

```mermaid
flowchart LR
  db["Database resource<br/>(control plane · plan/version/storage)"] --> cnpg["CNPG Cluster<br/>(tenant namespace)"]
  cnpg --> svc["cluster-rw Service<br/>(ClusterIP, in-cluster)"]
  cnpg --> sec["Secret<br/>(user + password)"]
  svc --> appx["tenant App<br/>uses INTERNAL url"]
  svc --> tcp["Traefik TCP router<br/>SNI + TLS · id.db.bex.co:5432"]
  tcp --> ext["external client<br/>uses EXTERNAL url · sslmode=require"]
```

### 3. Two URLs = two network paths (the core of the product)

|  | Render | bex (CNPG) |
| --- | --- | --- |
| **Internal URL** | `dpg-<id>-a` private host | the CNPG **`<cluster>-rw` ClusterIP Service**: `postgresql://user:pass@<cluster>-rw.<tenant-ns>.svc:5432/db` — in-cluster only, for the tenant's Apps. Free; CNPG creates the Service. |
| **External URL** | `<id>.<region>-postgres.render.com` + SSL | a **Traefik TCP router with SNI + TLS passthrough** on a shared `:5432` entrypoint, routing `<db-id>.db.bex.co` → that DB's Service (Postgres speaks its own TLS, so `sslmode=require`). One wildcard `*.db.bex.co` + entrypoint fans out to every DB — no per-DB LoadBalancer. Opt-in per DB via `spec.public`. |

The external route is created by the operator only when `spec.public: true` and `BEX_DB_DOMAIN` is set (private by default). Two constraints are load-bearing and easy to miss:

- **DNS-only (gray-cloud) wildcard.** `*.db.bex.co` must be a plain A record to the node IP, **not** a Cloudflare-proxied one — Cloudflare's proxy is HTTP(S) only; it cannot carry raw TCP `:5432` (that needs Spectrum, an enterprise feature). This differs from the App/API domains, which are proxied.
- **The Postgres SSLRequest preamble vs SNI.** libpq's default negotiation sends a _cleartext_ `SSLRequest` **before** the TLS `ClientHello`, so Traefik (which reads SNI from the first-bytes ClientHello) cannot extract the SNI and route by host. SNI passthrough therefore works for clients using **direct TLS** — PostgreSQL 17+ with `sslnegotiation=direct` — but not for older preamble-mode clients. Broad-client compatibility needs a **Postgres-aware SNI proxy** (the `pg_sni_proxy` pattern) in front, which is deferred. For the MVP the external endpoint targets direct-TLS clients; the internal URL (what tenant Apps use) is unaffected.

### 4. Naming convention (copy Render's reasoning, not its strings)

Render's URL is engineered for three constraints at once; bex should satisfy the same:

- **Normalized db/role name** — Postgres unquoted identifiers allow only `[a-z0-9_]`, lowercased. Render turns `my-db` into `my_db` so the name works **unquoted everywhere** (queries, `pg_hba`, ORMs). Do the same.
- **`<name>_user` role** — one owner role per DB, distinct from the db name, greppable and predictable.
- **Typed opaque ID in the SNI hostname** — the immutable ID (not the mutable name) lives in `<id>.db.bex.co`, so a **rename never breaks a connection string**, the ID is unique/non-guessable, and one wildcard endpoint routes by SNI to the right backend. (Render: `dpg-<id>-a` where `-a` is the primary-instance address.)
- **Brand domain** — external endpoint on `db.bex.co` (the brand), not the app-hosting `onbex.co` — same rule as `api.bex.co` vs `onrender.com`.

### 5. Plans — three axes, not one size knob

Reading Render's pricing carefully: Free and Basic-256mb are _identical specs_ ($0 vs $6 — the difference is backups/persistence), and Pro-4gb ($55, 1 CPU) is cheaper than Basic-4gb ($75, 2 CPU) — because **Pro sells availability + PITR, not raw compute**. So a plan decomposes into three axes, each a distinct CNPG field:

| axis | CNPG field | Free | Basic | Pro |
| --- | --- | --- | --- | --- |
| compute (RAM/CPU) | `spec.resources.requests==limits` | 256Mi/0.1 | 256Mi–4Gi / 0.1–2 | 4–16Gi / 1–4 |
| availability | `spec.instances` (+ sync replica, anti-affinity) | 1 | 1 | **3, failover** |
| durability | `ScheduledBackup` / `spec.backup.barmanObjectStore` | none, expires | daily | **WAL → PITR** |
| storage | `spec.storage.size` (expand-only) | 1Gi bundled | bundled | metered |
| version | `spec.imageName` / `ImageCatalog` (per-cluster) | any of 13–18 |  |  |
| tuning | `spec.postgresql.parameters` (shared_buffers ~25% RAM) | per plan |  |  |

Every Render Postgres feature (per-tenant version, daily backup, PITR, HA, pooling, hibernation) maps to a CNPG mechanism — so CNPG is sufficient for Render parity, and there is no case for switching to Crunchy (pgBackRest only wins at extreme scale) or a multi-engine strategy (Redis etc. is a _separate operator_, not a CNPG gap).

### 6. Lifecycle (create → URLs → connect → delete)

1. **Create** `Database{name, plan, version, storage}`.
2. **Provision** → CNPG `Cluster` in the tenant namespace from the plan catalog; CNPG generates the `<name>_user` role + password into a namespace Secret.
3. **Surface connection info** → the control plane assembles both URLs + host/port/db/user/password/psql-command from the Secret and the `-rw` Service (the Connections panel).
4. **Delete** → deleting the `Database` deletes the Cluster + Service + Secret + PVC and drops the external SNI route. (Optionally a final backup first.)

### 7. Security floor (not optional, even in MVP)

- **Internal path** — the tenant's default-deny NetworkPolicy already isolates the `-rw` Service to the tenant's own pods.
- **External path** — a public Postgres port. **Require `sslmode`** and a per-DB **IP allowlist** (Render exposes exactly this). Without it you're publishing an open Postgres.

## MVP scope

Ship only what fits the current single `cx33` (8 GB) node — single-instance plans differing by resources + backup:

- **free** — 256Mi/0.1CPU/1Gi, `instances:1`, no backup, **30-day TTL** (a control-plane sweeper — else "free forever" eats the cluster, which is why Render expires them).
- **basic-256mb** — 256Mi/0.1CPU, `instances:1`, daily `ScheduledBackup`.
- **basic-1gb** — 1Gi/0.5CPU, `instances:1`, daily backup.

Smallest possible first cut: internal-URL-only (connect from an in-cluster App), then add the Traefik-SNI external route as step 2 — but since the external URL _is_ the Render experience, the MVP should include its basic form.

## Alternatives considered

- **Operators** — Zalando (older Patroni/StatefulSet architecture), Crunchy PGO (pgBackRest, heavier, only wins at extreme backup scale), StackGres (AGPL + heavy), KubeDB (freemium). CNPG chosen; see §1.
- **Isolation model** — shared cluster + row-level (weakest; that's how the _control plane_ stores its own tenants, not tenant DBs) and database-per-tenant on a shared cluster (noisy neighbor, shared blast radius) both rejected in favor of **instance-per-tenant-DB** (matches the namespace compute model).
- **Attach DB to App (`App.spec.database`)** — rejected; a DB shouldn't die with one app or be un-shareable. Standalone `Database` is Render-faithful.
- **Per-DB LoadBalancer for the external endpoint** — rejected; a Hetzner LB per DB is costly and doesn't scale. **Traefik TCP + SNI** on one wildcard endpoint is how Render does it (region proxy routes by SNI hostname).

## Advanced: data protection, lifecycle, access (w1/m17)

Built on the same CNPG-is-the-executor principle — every advanced capability maps to a CNPG mechanism the operator projects and bex-api drives (three adapters: REST/GraphQL/MCP, see [ADR006-bex-api.md](ADR006-bex-api.md#managed-postgres-render-v1postgres-compatible)).

### Backups + point-in-time recovery

- **Durability is a plan axis** (`lego/types/tiers` `postgres[].backup`): Free is ephemeral (no backups, as on Render); `basic-*` opt in. When a backed-up plan runs and the operator's backup store is configured (`BEX_DB_BACKUP_DESTINATION` / `BEX_DB_BACKUP_ENDPOINT` / `BEX_DB_BACKUP_S3_SECRET`), the controller projects `spec.backup.barmanObjectStore` (continuous WAL archiving → object storage, gzip, `retentionPolicy: 30d`) plus a daily `ScheduledBackup`. WAL archiving is what makes PITR possible; the base backup pins the window. The object store is the **same Wasabi/S3 bucket + credential pattern** as the etcd/OpenBao runbooks ([ADR011-etcd-backup-restore.md](ADR011-etcd-backup-restore.md)) — a per-namespace Secret with `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`. CNPG scopes each cluster's backups under `serverName` (the cluster name), so one bucket fans out to every tenant DB.
- **`recovery-info`** reports whether recovery is available (`Status.BackupsEnabled`), the restorable window (earliest = CNPG `firstRecoverabilityPoint`, latest ≈ now via the continuous WAL), and the backup list. A no-backup plan returns `{enabled:false}`, never an error.
- **`recover` always restores to a NEW `Database`** (never in place — matching Render): bex-api creates a Database with `spec.recovery{sourceDatabase, targetTime}`, which the controller bootstraps via CNPG `bootstrap.recovery` + an `externalClusters` entry reading the source's `serverName` from the shared store. The source instance is untouched.
- **`exports`** trigger/list on-demand snapshots. bex's export is a **physical CNPG on-demand `Backup`** (a restorable base-backup snapshot to object storage), a conscious divergence from Render's logical `pg_dump` — honest, restorable, and reusing the same store rather than a bespoke dump pipeline.

### Lifecycle (suspend / resume / restart)

Mirrors the compute and KeyValue lifecycle verbs, writing intent to the `Database` CR the operator converges (see [ADR007-restart-suspend-and-resume.md](ADR007-restart-suspend-and-resume.md)):

- **suspend / resume** → `spec.suspended`, mapped to CNPG **hibernation** (`cnpg.io/hibernation=on`/removed). Postgres can't scale-to-zero, but hibernation stops compute and keeps the PVC (data), credentials, and any external route, preserving the sleep=free promise; a suspended DB settles `Ready`/`Suspended` immediately rather than waiting on ready instances.
- **restart** → `spec.restartedAt` (verb-as-timestamp), stamped onto CNPG's `kubectl.kubernetes.io/restartedAt` annotation for a rolling restart of the primary.

### Access & connection

- **IP allowlist** (`spec.ipAllowList`) gates the **external** SNI route only: the controller projects a Traefik `ipAllowList` `MiddlewareTCP` (source-range) referenced by the route, so only listed CIDRs reach the public endpoint. Empty ⇒ open to all source IPs; the internal `-rw` path is never gated. This closes the "public was an on/off toggle with no source restriction" gap (§7).
- **PgBouncer pooler** (`spec.pooler`) projects a CNPG `Pooler` (transaction mode) whose `<name>-pooler` Service backs the pooled connection strings `connection-info` now returns (internal always; external when `public`, via a `<name>-pool.<domain>` SNI route). This fills the previously-stubbed `internalConnectionPoolString`/`externalConnectionPoolString`.
- **Postgres users** (`spec.users`) are additional managed login roles projected to CNPG `spec.managed.roles`; bex-api generates each role's password into a per-user basic-auth Secret (`<db>-user-<role>`, referenced by CNPG's `passwordSecret`) and reveals it once on creation — never logged. The owner role (`<db>_user`) stays CNPG-managed.

## HA · failover · read replicas (w1/m22)

Shipped 2026-07-12. All three Render fields verified against the live API ([render-artifacts/postgres-ha.md](render-artifacts/postgres-ha.md)):

### High availability

- `spec.highAvailability` (Render's `enableHighAvailability`) provisions a replicated CNPG cluster. When `true`, the operator raises `spec.instances` to `max(plan.Instances, 2)` and adds pod anti-affinity (`enablePodAntiAffinity: true, topologyKey: kubernetes.io/hostname`) so primary and standby land on different nodes. The ready-gate waits for both instances before reporting `Ready`.
- `status.highAvailabilityEnabled` (Render's read field) is `true` only when HA is on _and_ ≥2 instances are actually ready — it reflects the operator's observed state, not just the spec intent.
- Independent of `readReplicas`: a DB can have read replicas without HA and vice versa (Render's documented independence).

### Failover

- `spec.failoverAt` (verb-as-timestamp, like `restartedAt`) triggers a CNPG **planned switchover**: the operator reads the cluster's `status.instanceNames`, finds a non-primary instance, and patches `cluster.status.targetPrimary` to it. CNPG then fences the old primary and promotes the target. The operator records the timestamp in `status.lastFailoverAt` so it only acts once per request.
- API surfaces: REST `POST /v1/postgres/{id}/failover → 202` (no body, matching Render exactly); GraphQL `failoverDatabase(id) → Boolean`; MCP `failover_postgres`. All three write `spec.failoverAt` through `Failover()` in `lifecycle.go`.
- Requires `// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=clusters/status,verbs=patch`.

### Named read replicas

- `spec.readReplicas: [{name}]` (Render's `readReplicas` create field) declares named read-only replica endpoints. Each entry gets its own Traefik `IngressRouteTCP` routing to the CNPG `-ro` service (which load-balances across standbys). Naming is `<db>-ro-<name>.<BEX_DB_DOMAIN>` for the external SNI hostname; the internal host is the shared CNPG `-ro` service.
- `status.readReplicaStatuses: [{name, internalHost, externalHost}]` tracks the resolved hosts. Route cleanup: when a replica is removed from spec, the operator finds it in the previous `status.readReplicaStatuses` and deletes its Traefik route.
- Connection strings (with password) are in `connection-info` as `readReplicaConnectionStrings: [{name, internalConnectionString, externalConnectionString}]` — host-only info (without password) is also in the view.

## Consequences

- Deferred: storage autoscaling, the Accelerated tier, metering/billing. Backups + PITR + lifecycle + access shipped in **w1/m17**; HA/replicas/failover in **w1/m22**.
- **`bex-db` (the control plane's own DB) should go `instances:3` + backups before you depend on it** — today it's `instances:1`, 5Gi, no backup config; losing it loses every tenant mapping. Higher priority than any single tenant DB.
- The external-endpoint IP allowlist and pooler routes ride the same `*.db.bex.co` wildcard SNI entrypoint (§3) — no per-DB LoadBalancer; the pooled endpoint adds a `<name>-pool.<domain>` SNI hostname.

## Verification

- **Target UX proven** (this ADR's research): created a real Render Free DB, connected to its **external URL** over TLS (PostgreSQL 18.4, ran `create table`/`insert`/`select`), then deleted it and confirmed the connection died.
- **bex MVP verification (when built):** apply a `Database` CR → CNPG `Cluster` reaches healthy → connect via the **internal** URL from an in-cluster pod and via the **external** URL (`<id>.db.bex.co`, `sslmode=require`) with `psql` → delete the `Database` → Cluster/Service/Secret/PVC gone and the external route removed.
