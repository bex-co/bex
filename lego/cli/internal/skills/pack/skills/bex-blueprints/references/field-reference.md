# bex Blueprint YAML Field Reference

Authoritative detail for `render.yaml` fields. Pair with `wiring-patterns.md`, `common-mistakes.md`, and `preview-environments.md`.

## Conventions

- **`name`**: Stable identifier for the resource within the Blueprint; used by `fromService`, `fromDatabase`, and the Dashboard.
- **Required vs optional** vary by `type` and `runtime`; validate with the JSON Schema (`https://render.com/schema/render.yaml.json`) and `bex blueprints validate`.

---

## Common service fields (most types)

Applies broadly to `web`, `pserv`, `worker`, `cron`, and often `keyvalue` where relevant.

| Field | Notes |
|-------|--------|
| `name` | Unique among services in scope. |
| `type` | `web`, `pserv`, `worker`, `cron`, `keyvalue` (alias `redis`). **Immutable after create.** |
| `runtime` | `node`, `python`, `go`, `ruby`, `rust`, `elixir`, `docker`, `image`, `static`. **Immutable after create.** |
| `region` | Deploy region. |
| `plan` | Instance/plan slug. |
| `branch` | Git branch to deploy. |
| `rootDir` | Subdirectory for repo context. |
| `buildCommand` | Build step(s). |
| `startCommand` | Process to run (not used the same way for `cron`—see below). |
| `preDeployCommand` | Command before deploy rollout. |
| `autoDeployTrigger` | `commit`, `checksPass`, or `off`. |
| `maxShutdownDelaySeconds` | 1–300; default **30**. |
| `healthCheckPath` | HTTP path for health checks (web services only). |
| `domains` | Custom domains list. |
| `envVars` | List of env var objects (see [Env vars patterns](#env-vars-patterns)). |
| `buildFilter` | Limit builds to path changes (see [buildFilter](#buildfilter)). |
| `disk` | Persistent disk attachment (see [disk](#disk)). |
| `scaling` | Autoscaling settings (see [scaling](#scaling)). |
| `numInstances` | Manual instance count; interacts with `scaling`. |
| `registryCredential` | For private base images in a `runtime: docker` build. |
| `image` | For `runtime: image`: `url` plus optional `creds.fromRegistryCreds`. |
| `dockerfilePath` | Path to Dockerfile (`docker` runtime). |
| `dockerContext` | Docker build context path. |
| `dockerCommand` | Override container command. |
| `previews` | Mixed preview settings: `generation` configures independent service previews; `plan` and `numInstances` configure the service in preview environments. |

**Cron-specific**

| Field | Notes |
|-------|--------|
| `schedule` | Cron expression. |
| `buildCommand` / `startCommand` | Build + command invoked on schedule. |

**Key Value (`keyvalue` / `redis`)**

| Field | Notes |
|-------|--------|
| `maxmemoryPolicy` | Eviction policy for Key Value. |
| `ipAllowList` | **Typically required** for access control; mistakes around this are common. |

**Private service (`pserv`)**

| Field | Notes |
|-------|--------|
| (common fields) | Internal hostname/port wiring via `fromService` for consumers. |

**Web + `runtime: static` (static sites)**

| Field | Notes |
|-------|--------|
| `staticPublishPath` | Directory of built assets to publish. |
| `headers` | Custom response headers (rules/objects per schema). |
| `routes` | SPA / routing rules (e.g. fallback to `index.html`). |

---

## Database fields (`databases`)

| Field | Notes |
|-------|--------|
| `name` | Blueprint identifier; **immutable** after creation. |
| `plan` | Database plan. |
| `previewPlan` | Plan for preview DB instances. |
| `previewDiskSizeGB` | Disk sizing for preview databases. |
| `postgresMajorVersion` | Major version; **immutable**. |
| `databaseName` | **Immutable.** |
| `user` | **Immutable.** |
| `region` | **Immutable.** |
| `diskSizeGB` | Storage size. |
| `storageAutoscalingEnabled` | Autoscale storage when supported. |
| `readReplicas` | Replica list; **empty list removes all replicas** (destructive). |
| `highAvailability` | HA enabled/disabled per plan support. |
| `ipAllowList` | CIDR restrictions where applicable. |

---

## envVarGroups

| Field | Notes |
|-------|--------|
| `name` | Group identifier for `fromGroup`. |
| `envVars` | Variables may use `value` or `generateValue`; they cannot reference services, databases, or other groups. |

**Constraints:** `sync: false`, `fromDatabase`, `fromService`, and `fromGroup` are **not valid** in env var groups.

---

## projects / environments

Projects group one or more **environments** (e.g. production, staging) under a single bex project. Use this pattern for multi-service apps.

### Project fields

| Field | Notes |
|-------|--------|
| `name` | Project name (appears in Dashboard). |
| `environments` | List of environment blocks. |

### Environment fields

Each environment is a self-contained set of resources:

| Field | Notes |
|-------|--------|
| `name` | Environment key (e.g. `production`, `staging`). |
| `services` | Services belonging to this environment (same schema as top-level `services`). |
| `databases` | Databases belonging to this environment (same schema as top-level `databases`). |
| `envVarGroups` | Env var groups scoped to this environment. |

### Structural rules

- **No duplication:** Define each resource in **one** place: either at the root level **or** inside a single environment. Never both.
- **Cross-environment wiring:** `fromDatabase` and `fromService` resolve within the same environment. Referencing resources across environments is not supported in Blueprint YAML.
- **Environment isolation:** With a Pro workspace or higher, environments can be isolated so services in different environments cannot reach each other over the private network.
- **When to use projects:** Multiple services that belong together, staging/production parity, environment-scoped env groups. Single-service apps can use flat top-level lists instead.

---

## previews (top-level)

| Field | Notes |
|-------|--------|
| `generation` | `off` (default), `manual`, or `automatic`. |
| `expireAfterDays` | Delete preview environments after N days. |

Top-level `previews.generation` controls preview environments. Service-level `previews.generation` instead controls independent service previews, accepts only `manual` or `automatic`, and does not override the top-level setting. Omit it to disable service previews.

---

## Env vars patterns

Each entry is typically keyed by `key` plus **one** of:

| Pattern | Purpose |
|---------|---------|
| `value` | Inline literal. |
| `generateValue` | bex generates a random secret. |
| `sync` | `false` = do not sync from Blueprint after initial setup (secrets); see limitations in preview/group docs. |
| `fromDatabase` | Pull DB host, port, user, password, database, `connectionString`, or `connectionPoolString`. |
| `fromService` | Pull host/port/connection info from another service. |
| `fromGroup` | Import vars from an `envVarGroup` by name. |

---

## scaling

| Field | Notes |
|-------|--------|
| `numInstances` | Fixed instance count when not using autoscaling (or as baseline depending on config). |
| `scaling.minInstances` | Floor for autoscaled services. |
| `scaling.maxInstances` | Ceiling for autoscaled services. |
| `scaling.targetCPUPercent` | CPU target for autoscaling. |
| `scaling.targetMemoryPercent` | Memory target for autoscaling. |

**Previews:** Autoscaling is disabled; previews use **minInstances** behavior (see `preview-environments.md`).

---

## disk

Before configuring an attached service disk, read [persistent-disks.md](persistent-disks.md) for its architectural constraints and confirm the current schema in the Blueprint specification.

| Field | Notes |
|-------|-------|
| `name` | Disk identifier. |
| `mountPath` | Absolute mount path in the instance filesystem. |
| `sizeGB` | Provisioned disk capacity in GB. |

---

## buildFilter

Limit which Git changes trigger builds:

| Field | Notes |
|-------|--------|
| `paths` | Include paths (glob / path rules per schema). |
| `ignoredPaths` | Excluded paths. |

**Sync behavior:** Omitting `paths` or `ignoredPaths` when syncing can **replace** existing filters with empty lists—always send the full desired filter set.

---

## static site fields (`type: web`, `runtime: static`)

| Field | Notes |
|-------|--------|
| `buildCommand` | Build static assets. |
| `staticPublishPath` | Output directory for publishing. |
| `headers` | Header rules for responses. |
| `routes` | URL routing / SPA fallback configuration. |
| `buildFilter` | Path-based build triggers. |

---

## Key Value fields (`type: keyvalue`)

| Field | Notes |
|-------|--------|
| `name`, `region`, `plan` | Standard resource fields. |
| `previewPlan` | Plan for preview Key Value instances. |
| `maxmemoryPolicy` | Eviction policy string. |
| `ipAllowList` | **Required** for proper network restriction in typical setups—do not omit unless you explicitly accept open access per product rules. |

---

## Schema

Use `https://render.com/schema/render.yaml.json` as the source of truth for required fields and enum values; bex may add fields over time.
