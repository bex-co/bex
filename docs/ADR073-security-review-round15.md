# ADR073: Security review round 15 — XvLnJG disposition

- **Status**: Accepted (2026-08-18)
- **Scan**: codex-security `XvLnJG`, repository revision `96f58943` (2026-08-18), 8 findings (2 high, 3 medium, 3 low)
- **Lineage**: fifteenth pass in the ADR028 → ADR045 → ADR055 → ADR056 → ADR057 → ADR072 → ADR061 → ADR063 → ADR064 → ADR066 → ADR067 → ADR068 → ADR069 lineage

## Summary

Five findings are fixed in place with regression tests; one is a partial fix with a recorded residual; two are the standing PSL and digest-pinning residuals (tenth and eighth reports). Nothing was rejected.

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Autoscaling replica counts unbounded (int32) | high | **Fixed** — platform ceiling `100` on API, CRD, and operator projection |
| 2 | Unbounded webhook URLs reloaded every 2s | high | **Fixed** — 2048-byte cap + CHECK + catalog query drops URL |
| 3 | GitHub callback skips current `can_manage` | medium | **Fixed** — fresh check on the initiator against the transaction workspace before upsert |
| 4 | Cached authz on blueprint manifests / Postgres insights | medium | **Fixed** — `AuthorizeFresh` / `AuthorizeDatabaseFresh` before Git fetch, Manifest reveal, and DB dial |
| 5 | Live log streams survive revocation | medium | **Fixed** — watchdog shipped as w4/034; this round extracts it to `core.WithRevalidation` shared with SSH/agent transports |
| 6 | Shared onbex.co tenant hosts permit sibling cookie injection | low | **Accepted residual** — onbex.co PSL submission (tenth report), blocked on operator action |
| 7 | Snapshot PUT in argv; restore skips digest | low | **Partial fix** — restore verifies SHA-256 + size before extract; argv PUT remains a residual |
| 8 | Mutable images beside datastore secrets | low | **Deferred** — digest-pinning inventory (eighth report; ADR060 §D7) |

## Finding 1 (high) — autoscaling cannot outrun the replica ceiling

`SetAutoscaling` (and the Blueprint `scaling` block that shares `autoscalingSpec`) enforced sign and ordering but not `store.MaxReplicas`. The CRD allowed any int32, the autoscaler treated stored min/max as authoritative, and `Deployment.spec.replicas` received the result. A workspace operator could therefore ask for a billion replicas and keep the shared scheduler busy.

**Fix — one ceiling, three enforcement points.** `types.MaxReplicas = 100` is the source of truth (`lego/types/v1alpha1`); `store.MaxReplicas` aliases it so the API and operator cannot drift:

- **API.** `autoscalingSpec` refuses `minInstances`/`maxInstances` above the ceiling (inclusive 100 is accepted). Create/scale already compared against the same constant.
- **CRD.** `spec.replicas`, `spec.autoscaling.minReplicas`, and `spec.autoscaling.maxReplicas` carry `+kubebuilder:validation:Maximum=100`.
- **Operator.** `autoscaleDesired` clamps both bounds; `clampReplicas` is the last bound on every Deployment replica target, including a CR that predates the schema.

Tests: `SetAutoscaling` and Blueprint parse refuse 101 and accept 100; the autoscaler and Deployment projection cap an oversized stored spec.

## Finding 2 (high) — webhook destinations are length-bounded and off the catalog poll

The shared worker reloads every enabled endpoint every two seconds. Destination URLs were bounded only by the 2 MiB request-body cap, so one workspace's catalog row was a persistent memory and query-byte multiplier across every replica.

**Fix.** `MaxWebhookURLBytes = 2048` is enforced at parse (REST/GraphQL/MCP share `parseDestination`), at `CreateWebhookEndpoint`/`UpdateWebhookEndpoint`, and by migration `0088` (`CHECK (octet_length(url) <= 2048)`; legacy over-limit rows are rewritten to a non-routable sentinel and disabled). `ListEnabledWebhookEndpoints` now selects only `id, tenant_id, event_types, created_at` — dispatch does not need the URL; send loads it per due delivery.

Tests: Create refuses an HTTPS URL over the cap.

## Finding 3 (medium) — GitHub install callback re-checks current administration

`connectFromCallback` consumed the nonce, proved the caller was the initiator, proved GitHub installation administration, then upserted `git_connections` with **no** current workspace authorization. `StartConnect` had checked `can_manage`, but GitHub's install UI can take minutes; a demotion inside that window still bound the installation.

**Fix.** After the installation-admin proof and before `connectWithWorkspace`, `AuthorizeFreshOn(txn.Subject, RelCanManage, workspace:txn.TenantID)`. Nil `Authz` still allows (local/dev). The nonce stays single-use: a fresh deny cannot be retried.

Tests: a stale-positive checker refuses the callback and leaves `git_connections` empty; a current admin still completes.

## Finding 4 (medium) — sensitive reads use the fresh seam

`RelCanViewSensitive` is a read relation, so `Authorize`/`Can` ride the 30s positive cache. Two disclosure sinks used it:

- **Blueprint preview** fetched private repo contents after a cached allow. Get/list used `Can` to decide whether to serve the stored Manifest (the same private file, plus possible literal env values).
- **Postgres Processes/TopQueries** (and every other insight that dials the tenant database) loaded the connection Secret through `loadAppSecret` → cached `AuthorizeDatabase`.

**Fix.** `PreviewBlueprint` calls `AuthorizeFresh` before `discoverBlueprintFile`. Get/list blank Manifest unless `AuthorizeFresh(RelCanViewSensitive)` succeeds (metadata stays viewer-readable). `runInsight` is `AuthorizeDatabase` → `AuthorizeDatabaseFresh` → `databaseSecret`, matching the SQL-console path. `TopQueries` already rethrows `ErrForbidden` / `ErrAuthzUnavailable`.

Tests: preview does not call `GitFetcher` on a stale positive; get/list blank Manifest while keeping the name; Processes and TopQueries return `ErrForbidden` on the same checker.

## Finding 5 (medium) — live log tails are a lease

`FollowLogs` authorized once at subscribe and then pumped pod logs until disconnect. Native SSH, web shell, agent attach, and sandbox-exec already re-ran fresh authorization on `BEX_SSH_REVALIDATE_INTERVAL` (default 1m). A revoked member could keep reading app/build stdout for the life of the SSE slot.

**Fix.** w4/034 already landed the logs watchdog (`BEX_LOG_STREAM_REVALIDATE_INTERVAL`, App re-fetch + `AuthorizeAppFresh`, SSE/WebSocket/NDJSON). This round extracts the loop to `core.WithRevalidation` so logs, SSH, and agent transports share one helper (SSH and logs keep thin aliases). Postgres/Key Value live tails stay short-circuit 400s and are not wrapped.

Tests: `TestLogStreamRevalidation*` (stale-allow + fresh-deny, checker outage, App deletion, healthy tail, WebSocket, negative-disable).

## Finding 6 (low) — onbex.co PSL (tenth report)

Unchanged. `hostingdomain.ValidateSharedSuffix` still identifies the browser trust-boundary failure; the manager still continues with a warning because failing closed would disable platform hosting. Operator action: `.pm/w1/050.md`. Close when browsers classify `onbex.co` as a public suffix.

## Finding 7 (low) — restore verifies the recorded object; PUT-in-argv stays residual

Hibernation still embeds a 15-minute presigned **PUT** in `/bin/sh -c` argv (`hibernateScript`). The gateway sandbox-exec path has no stdin, so moving the URL to env on that hop is not a drop-in; treating argv as a credential is the standing residual (create-only upload / isolated helper is the real fix).

Restore already used env (`BEX_AGENT_RESTORE_URL`), not argv, but extracted without comparing the object to `SnapshotSHA` / `SnapshotBytes`. That half is closed: rehydrate stamps `BEX_AGENT_RESTORE_SHA` and `BEX_AGENT_RESTORE_BYTES`; the driver downloads to a temp file, verifies size and SHA-256, then `tar xzf`. Mismatch is fatal before extract. Empty SHA / zero bytes stay optional so rows that predate digest recording still restore.

Tests: resume stamps the digest/size env; the driver refuses a wrong SHA and a wrong size without extracting.

## Finding 8 (low) — digest-pinning inventory (eighth report)

Unchanged in substance, and wider than last round: `oliver006/redis_exporter:alpine` (holds `REDIS_PASSWORD`), tenant-chosen Valkey version tags on serving + snapshot, `apk add age` at backup runtime, and CloudNativePG version tags on export Jobs. The answer remains ADR060 §D7's internally-built signed toolchain images, not more one-off pins.

## Deferred and follow-up

Carried forward from this round:

- **onbex.co PSL submission** (finding 6, tenth report): operator action, `.pm/w1/050.md`.
- **Digest-pinning inventory** (finding 8, eighth report): tenant-version Valkey, redis_exporter, CNPG export tags, `apk add age`, and the wider Dockerfile/kpack/barman inventory from ADR061 #1. ADR060 §D7.
- **Hibernate PUT capability in argv** (finding 7 residual): create-only upload broker or isolated helper; do not place presigned URLs in same-UID process arguments.

The scan's open questions that are unchanged repeats of standing migration-gated items (ADR055 F2/F3 registry/S3-prefix identity) are not re-litigated here.
