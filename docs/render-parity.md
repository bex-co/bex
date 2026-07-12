# Render parity matrix — REST · GraphQL · MCP · UI

A single, evidence-based map of how far bex actually matches [render.com](https://render.com), one row per Render capability across the four surfaces bex mirrors. It replaces scattered "Render-compatible" assertions with a checkable ledger: what has parity, what diverges, what is missing, and what is a deliberate non-goal — each cell backed by a pointer to code (bex side) or Render's own spec/docs (Render side).

**Method.** Four passes, 2026-07-08, re-verified against Render's live API/docs 2026-07-09: bex's public REST vs Render's OpenAPI spec; bex's GraphQL vs Render's captured dashboard operations; bex's MCP tools vs `render-oss/render-mcp-server` v0.3.0 (24 tools — incl. three non-functional `update_{web_service,static_site,cron_job}` stubs that return "use the dashboard/API"; Render's only functional MCP write is the single env-var tool `update_environment_variables`); bex's dashboard IA vs Render's (via render.com/docs — the live dashboard needs a login). The design record for bex's side is [bex-api.md](bex-api.md) ("one Core, three adapters"); this doc is the parity ledger over it.

**Legend.** ✅ parity (evidence pointer) · ◐ partial (divergence documented) · ✖ missing (gap — see backlog) · — deliberate non-goal (rationale inline). "Render capability" = a noun/verb Render exposes on _any_ of its surfaces; a `—` on the REST column can still be a `✅` elsewhere (Render itself splits work across surfaces — e.g. read-only SQL is MCP-only on both sides).

> **Headline.** bex has real parity on the **service lifecycle, env vars, environment groups, secret files, custom domains, managed Postgres, managed Key Value, logs, metrics, and autoscaling config** core across all four surfaces, and is _ahead_ of Render on three AI-native verbs (API-key management over the API, deploy-from-chat, an inbound push webhook — [§ bex ahead of Render](#bex-ahead-of-render)). The open frontier is **deploys as first-class objects, static sites, and advanced Postgres data-protection** — all mapped to owning milestones/notes in the [§ Gap backlog](#gap-backlog). (Background-worker and cron-job service types shipped in w1/m15; environment groups + secret files in w1/m16; managed Key Value landed on REST/GraphQL/MCP in w2/m7 and the dashboard in w5/m12; autoscaling config shipped in w1/m20.)

---

## Services & lifecycle

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| List services | ✅ | ✅ | ✅ | ✅ | `apps/rest.go` `GET /v1/services`; GraphQL `services`; MCP `list_services`; dashboard `routes/index.tsx`. Omits Render `includePreviews` (no preview services). |
| Get service | ✅ | ✅ | ✅ | ✅ | `GET /v1/services/{id}`; GraphQL `server`/`service`; MCP `get_service`; Overview tab. bex extras `phase`/`replicas`/`revision` (superset Render clients ignore). |
| Create service (web / private) | ◐ | ◐ | ◐ | ✖ | `POST /v1/services` upsert; GraphQL `createService`; MCP `create_web_service` + `deploy`. Verified field-by-field vs Render's OpenAPI (w2/m4/t001): `type`/`name`/`repo`/`branch`/`image{imagePath}`/`autoDeploy`/`envVars`/`serviceDetails.{plan,numInstances,healthCheckPath}` honored; `autoDeploy` sets `spec.autoDeploy` (gates the push-to-deploy webhook). Omits Render `region`/`runtime`/`buildCommand`/`startCommand` (Dockerfile/CNB auto-detect) + `ownerId` (single workspace). **Divergence:** returns the bare service object, not Render's create-only `{service, deployId}` envelope (deploy history is its own endpoint; no faked `deployId`) — [bex-api.md](bex-api.md). No dashboard create wizard (API-first; deploy-from-chat is the path — w2/m2). |
| Root Directory (build from a monorepo subdirectory) | ✅ | ✅ | ✅ | ✅ | `App.spec.rootDir` (`app_types.go`) scopes the BuildKit git context to `:<rootDir>` (`build.go` `gitContext`) and scopes the git-push auto-deploy webhook to paths under it (`webhook.go` `rootDirMatches`); settable on create (REST `rootDir`, GraphQL `createService(rootDir:)`, MCP `create_web_service`/`create_cron_job`) and after create (REST `PATCH .../{id}` `rootDir`, GraphQL `setRootDir`, MCP `set_root_directory` — `w1/m18`); readable back on all three, including `repo`/`branch` (`AppView`/`renderService`/GraphQL `Service`). Dockerfile builder only — CNB (`spec.builder: buildpack`) is still not in-cluster, so `rootDir` has no effect there. Dashboard Settings → Build & Deploy section (Source/Branch read-only, Root Directory editable) — `w5/m13`. |
| Health checks (path → readiness gating) | ◐ | ✖ | ✖ | ✖ | `serviceDetails.healthCheckPath` is accepted into `spec.healthCheckPath` (`apps/deploy.go`, `apps/rest.go`) but the operator never wires it to a ReadinessProbe — Running gates on replica-readiness only. Not on GraphQL/MCP create or the dashboard. → **w1/005**. |
| Change instance plan / type | ✅ | ✅ | ✅ | ✅ | `PATCH /v1/services/{id}` (plan); GraphQL `updateServicePlan`; MCP `update_service_plan`; Plan-picker page. `rootDir` (w5/m13) and `autoDeploy` (w2/m9 — `PATCH autoDeploy`, GraphQL `setAutoDeploy`, MCP `set_auto_deploy`, Build & Deploy toggle) are also editable. Remaining `PATCH` fields (name, buildFilter) not editable — ◐, low. |
| Suspend / Resume | ✅ | ✅ | ✅ | ✅ | `POST …/suspend` (202) · `…/resume` (202); GraphQL `suspendService`/`resumeService`; MCP `suspend_service`/`resume_service`; row + header actions. Render parity verified in [bex-api.md](bex-api.md). |
| Restart | ✅ | ✅ | ✅ | ✅ | `POST …/restart` (200); GraphQL `restartServer`; MCP `restart_service`; header action. Render's official MCP omits these — bex adds them (named after Render's REST verbs). |
| Manual scale (instance count) | ✅ | ✅ | ✅ | ✖ | `POST …/scale`; GraphQL `scaleService`; MCP `scale_service` (backend shipped w2/m12). Dashboard stepper → **w5/004**. |
| Autoscaling config (min/max + CPU/mem target) | ✅ | ✅ | ✅ | ✅ | Render `GET`/`PUT`/`DELETE /services/{id}/autoscaling`. bex: REST + GraphQL (`autoscalingConfig` query, `setAutoscaling` / `disableAutoscaling` mutations) + MCP (`get_autoscaling` / `set_autoscaling` / `disable_autoscaling` tools) + Dashboard Scaling tab. Operator reconciler: HPA-style metrics-server loop at 30 s, 5-min scale-down stabilization window. → **w1/m20**. |
| Delete service | ✅ | ✅ | ✅ | ✅ | `DELETE /v1/services/{id}` (204, empty body); GraphQL `deleteService` (success boolean, like `deleteCustomDomain`); MCP `delete_service` (`{deleted:true}`, bex extension — Render's official MCP ships no delete tool). One `Service.Delete` verb (`can_create` scope): store-managed → delete the apps row first (projector removes the CR, resync-safe) then the CR directly for immediate convergence; store-less → delete the CR directly. Operator ownerRefs cascade Deployment/Service/Ingress/CronJob/NetworkPolicy; the cert-manager TLS `Secret` is the one documented orphan (w2/m4). Dashboard: Settings-tab danger zone → **Delete Service** button opens a type-to-confirm dialog (exact service name), evicts the deleted `Service` from the Apollo cache, and redirects to the services list with a success toast (`delete-service-card.tsx`/`use-delete-service.ts`, w5/m14). |
| Service events / activity feed | ✖ | ✖ | ✖ | ✖ | Render `GET /services/{id}/events`. bex has no event objects. → **w2/m5** (deploy objects) + **w4/m10** (audit log). |
| Cache purge | — | — | — | — | Render `POST …/cache/purge` (static-site CDN). bex has no build CDN cache — non-goal. |
| Background worker (no HTTP port) | ✅ | ✅ | ✅ | ✅ | `spec.type=background_worker` → Deployment only, no Service/Ingress/URL (`app_controller.go`); create over `POST /v1/services`, GraphQL `createService(type:)`, MCP `create_web_service(type:)`; dashboard type badge + no-URL. (w1/m15) |
| Cron job (schedule + command + run history) | ✅ | ✅ | ✅ | ✅ | `spec.type=cron_job` + `spec.schedule` + `spec.command` (entrypoint override, applied via a shell in `cronPodSpec`) → k8s CronJob, `status.runs` (`app_controller.go`); run trigger `POST /v1/cron-jobs/{id}/runs` / GraphQL `runCronJob` / MCP `run_cron_job`; create/update thread both fields on REST (`schedule`/`command`, top-level or nested under `serviceDetails`), GraphQL (`createService(schedule:, command:)`), MCP (`create_cron_job`). Settings page (w5/m11): a cron job's Settings tab shows a **Deploy** section (Schedule + Command, read-only — write path is a follow-on) instead of Custom Domains/Idle timeout, neither of which applies to a service with no HTTP traffic. Deferred vs Render's cron Deploy section: Build Command, Auto Deploy toggle, Log Stream, Notifications (◐ — tracked here, not a separate row). (w1/m15, w5/m11) |
| Static site | ✖ | ✖ | ✖ | ✖ | Render `static_site` type (build → CDN with redirects/rewrites/headers). A larger build→CDN effort than the compute types. → **w1/012**. |
| One-off jobs (run a command) | — | — | — | — | Render `/services/{id}/jobs` runs an arbitrary command in the service context — an execution surface, off-roadmap (`DO_NOT_DO` §pillar 5), the same call as Shell/SSH below. (Scheduled cron jobs are a service type, tracked separately → w1/m15.) |
| Shell / SSH into a running instance | — | — | — | — | Render Shell tab / `render ssh`. No exec surface — hosted execution is off-roadmap (DO_NOT_DO §pillar 5). Non-goal for now. |
| PR preview environments | ✖ | ✖ | ✖ | ✖ | Render `POST …/previews` (plural) + Previews tab. The git-integration prerequisite is now met (GitHub App connection + private clones + signed webhook, w2/m8–m9), so this is unblocked — still low priority, untracked. |

## Deploys

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Trigger a deploy | ◐ | ◐ | ✅ | ✖ | No dedicated endpoint: `POST /v1/services` upsert re-applies the spec (rebuild) and the HMAC webhook redeploys on push; MCP `deploy` (bexYaml) + `create_web_service`. Render `POST …/deploys` → **w2/m5**. See [deploy-from-chat.md](deploy-from-chat.md). |
| List / get deploy objects | ✖ | ✖ | ✖ | ✖ | Render `list_deploys`/`get_deploy`, `GET …/deploys`. bex tracks only `status.revision` on the App, no deploy objects. → **w2/m5**. |
| Cancel deploy | ✖ | ✖ | ✖ | ✖ | Render `POST /services/{serviceId}/deploys/{deployId}/cancel`. → **w2/m5**. |
| Rollback | ✖ | ✖ | ✖ | ✖ | Render `POST …/rollback {deployId}`. Depends on deploy objects → extends **w2/m5**. |

## Environment & config

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Env vars (list · get · replace · set · delete) | ✅ | ✅ | ✅ | ✅ | All 5 REST endpoints (`secrets/rest.go`, verified vs Render OpenAPI); GraphQL dashboard shape (`envVarKeys`/`envVar` nested, `setEnvVars`/`setEnvVar`/`deleteEnvVar`); MCP 5 tools; Environment tab (w4/m6.5). **Divergence:** writes roll pods immediately; omits pagination + `generateValue` — see [bex-api.md](bex-api.md). |
| Secret files (mounted at `/etc/secrets`) | ✅ | ✅ | ✅ | ✅ | `secrets/files.go` `/v1/services/{id}/secret-files` (list · get · upsert · delete); GraphQL `service{ secretFileNames / secretFile(name) }` + `setSecretFile`/`deleteSecretFile`; MCP `list_/get_/set_/delete_secret_file`; Environment tab Secret Files section. Materialized into `<name>-files`, projected into a read-only `/etc/secrets` volume ([secrets.md](secrets.md)). **Divergence:** list is names-only (contents fetched per file, like env-var "Show secret"); writes roll pods immediately. (w1/m16) |
| Environment groups (+ link / unlink) | ✅ | ✅ | ✅ | ✅ | `internal/envgroups` `/v1/env-groups` (CRUD + `/env-vars` + `/secret-files/{name}` + `/services/{serviceId}` link/unlink); GraphQL `envGroups`/`envGroup` + `createEnvGroup`/`deleteEnvGroup`/`setEnvGroupVars`/`setEnvGroupSecretFile`/`deleteEnvGroupSecretFile`/`linkEnvGroup`/`unlinkEnvGroup`; MCP `list_/get_/create_/delete_env_group` + `update_env_group_vars` + `set_/delete_env_group_secret_file` + `link_/unlink_env_group`; dashboard Environment tab Env-Groups section. A group materializes to `<evg-id>-env` + `<evg-id>-files`; linking appends them to the service's `spec.envFromSecrets`/`spec.filesFromSecrets` ([secrets.md](secrets.md)). **Divergence:** reads are keys/names-only (values revealed per var/file); a group-var change rolls every linked service. (w1/m16) |

## Custom domains

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Custom domains (list · add · get · delete) | ✅ | ✅ | ✅ | ✅ | `apps/rest.go` `/v1/services/{id}/custom-domains` (w1/m11); GraphQL `customDomains`/`addCustomDomain`/`deleteCustomDomain`; MCP 4 tools; Settings section (w1/m11.5). [custom-domain.md](custom-domain.md). |
| Verify / DNS instructions | ✅ | ✅ | ✅ | ✅ | Per-domain `dnsRecord{type,name,value}` (CNAME → platform host for subdomains; ALIAS `@` for apex) + verify verb (`POST …/custom-domains/{name}/verify`, GraphQL `verifyCustomDomain`, MCP `verify_custom_domain`) re-check status now; verification stays automatic via cert-manager (verify is an idempotent re-read). Dashboard DNS-instructions panel + copy + re-check (w5/m10). [custom-domain.md](custom-domain.md), [render-artifacts/custom-domain-dns-instructions.md](render-artifacts/custom-domain-dns-instructions.md). |

## Managed Postgres

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Postgres CRUD + connection-info | ✅ | ✅ | ✅ | ✅ | `postgres/rest.go` `/v1/postgres` (+`/v1/databases` alias); GraphQL `databases`/`database`/`databaseConnectionInfo`/`createDatabase`/`deleteDatabase`; MCP `list_postgres_instances`/`get_postgres`/`create_postgres`; Databases pages (w5/m8). No `PATCH` update yet (◐, low). [postgresql-management.md](postgresql-management.md). |
| Read-only SQL query | — | — | ✅ | ✖ | MCP `query_render_postgres` (read-only envelope) — MCP-only on **both** sides (Render exposes no REST/GraphQL equivalent). Dashboard SQL console: none (low). |
| Lifecycle (suspend / resume / restart) | ✅ | ✅ | ✅ | ✅ | `POST /v1/postgres/{id}/{suspend,resume,restart}` (202/202/200); GraphQL `suspendDatabase`/`resumeDatabase`/`restartDatabase`; MCP `suspend_postgres`/`resume_postgres`/`restart_postgres`; detail-page lifecycle actions. suspend ⇒ CNPG hibernation (compute stops, PVC kept), restart ⇒ rolling restart (w1/m17). [postgresql-management.md](postgresql-management.md). |
| Backups · PITR / recovery | ✅ | ✅ | ✅ | ✅ | `recovery-info`/`recover`/`exports` over REST + GraphQL (`databaseRecoveryInfo`/`databaseExports`/`recoverDatabase`/`createDatabaseExport`) + MCP; Recovery section on the detail page. `recover` restores to a **new** Database via CNPG `bootstrap.recovery` (source untouched, matching Render). Divergence: bex `exports` are physical base-backup snapshots (CNPG on-demand `Backup`), not Render's logical `pg_dump` — documented, restorable. Backups gated on plan durability + `BEX_DB_BACKUP_*` (w1/m17). |
| HA · failover · read replicas | ✖ | ✖ | ✖ | ✖ | Render `failover`/`promote`/`replication`. `highAvailabilityEnabled` is reported `false` today. → **w1/013** (deferred from m17). |
| Access control (IP allowlist) · users · pooler | ✅ | ✅ | ✅ | ✅ | `ip-allow-list` (Traefik `ipAllowList` middleware gating the external SNI route), `/users` (CNPG managed roles, password revealed once), and PgBouncer pooler strings (CNPG `Pooler`) in `connection-info` — over REST + GraphQL (`databaseIpAllowList`/`databaseUsers`/`setDatabaseIpAllowList`/`createDatabaseUser`/`deleteDatabaseUser`) + MCP; access-control panel on the detail page (w1/m17). |
| Postgres observability (live queries · top-queries · sizes · table-scans · param overrides) | ✖ | ✖ | ✖ | ✖ | Render `GET /postgres/{id}/{processes,top-queries,sizes,table-scans}` + `parameter-overrides`. Runtime introspection over `pg_stat_activity`/`pg_stat_statements`. bex has none. Untracked, low (extends **w1/m17**). |

## Other datastores & storage

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Key Value (Valkey / Redis) | ✅ | ✅ | ✅ | ✅ | REST `keyvalue/rest.go` `/v1/key-value` (list/get/create/delete/`connection-info` + `suspend`/`resume`); GraphQL `keyValue*` (`keyValues`/`keyValue`/`keyValueConnectionInfo`/`keyValueInstanceTypes`, `createKeyValue`/`deleteKeyValue`/`suspendKeyValue`/`resumeKeyValue`); MCP `list_key_value_instances`/`get_key_value`/`create_key_value` (Render's exact 3-tool set — Render's MCP server has no KV delete/suspend tools, so bex mirrors that: those verbs are REST+GraphQL only, a deliberate match not a gap). Mechanism (w1/m14): a `KeyValue` CR → single-instance Valkey + internal Service DNS + credentials Secret, optional public Traefik TCP/SNI route; suspend scales the StatefulSet to zero (PVC/Secret kept, data + password survive). Plans `free`/`starter`/`standard` (the web-service vocabulary, **not** Postgres `basic-*`); connection-info 3 keys — `internalConnectionString` (`redis://`), `externalConnectionString` (`rediss://` TLS, when public), `cliCommand` (`redis-cli -u …`). **Divergences (conscious):** the id is the KeyValue name (name-as-id, the same documented deviation managed Postgres takes — [identifiers.md](identifiers.md) § Known deviations); the internal URL always mints a password (Render leaves it unauthenticated by default — bex is stricter, a safe superset); create/update fields `maxmemoryPolicy`/`persistenceMode`/`ipAllowList` are omitted (the CR can't back them yet) rather than faked → follow-up. **Dashboard (UI ✅, w5/m12):** `/keyvalue` sidebar entry + list (stat cards, status chips, empty state), `/keyvalue/new` full-page create form (name, tier cards from `keyValueInstanceTypes`, Valkey version, public toggle — a single page, matching Render's own `/new/redis` layout captured live rather than m8's dialog pattern, [docs/render-artifacts/key-value.md](render-artifacts/key-value.md)), `/keyvalue/$id` detail (status/plan metadata, on-demand connection-info reveal with internal/external/CLI fields + copy, suspend-with-confirm/resume, typed-name delete confirm). UI omissions matching the API's own gaps: no maxmemoryPolicy/persistenceMode fields in the create form, no per-instance IP-allowlist UI (Render's "Networking" section), no metrics tab (filed as a w5/m12 follow-up, not this milestone's scope) — omitted, not faked. |
| Persistent disks | — | — | — | — | Render `/disks` + Disks tab. Deliberate: bex is **stateless-first** (managed Postgres for state); disks disable multi-instance + zero-downtime deploys, which fights bex's dense bin-pack + free-tier-sleep economics. Non-goal. |

## Deployment sources & IaC

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Blueprint / `render.yaml` IaC | ◐ | ✖ | ◐ | ✖ | bex consumes a `render.yaml`-shaped `bex.yml` via `deploy` (MCP) + `scripts/app-apply.sh`, but exposes no `/blueprints` resource (validate/list/sync). → extends **w2/m2**; resource untracked, low. |
| Projects & environments (grouping) | ✖ | ✖ | ✖ | ✖ | Render `/projects`, `/environments`, protected environments. bex is flat apps in one workspace. Belongs to the tenancy line → nearest **w1/m9**; low. |
| Registry credentials (private images) | ✖ | ✖ | ✖ | ✖ | Render `/registrycredentials`. bex pulls from its own zot registry; external private registries unsupported. Low, untracked. |
| Git connections (GitHub / GitLab app) | ✅ | ✅ | ✅ | ✅ | **GitHub App** across all four surfaces (w2/m8 connect+list, w2/m9 private deploy + push): connect (install → callback) + repo list; private-repo deploys clone with a 1h installation token bex-api mints into a `<app>-clone` Secret (`spec.cloneSecret` → BuildKit `GIT_AUTH_TOKEN`); **zero-config push-to-deploy** — the app's app-wide webhook is a second accepted HMAC key on `POST /v1/webhooks/git` (no per-repo config); Auto-Deploy toggle (`setAutoDeploy`) + dashboard Connect-GitHub card & Build & Deploy toggle. GitLab/Bitbucket providers remain ◐ (out of scope). `GET /v1/repos` + MCP `list_repos`/`get_git_connection` are bex supersets (§ bex ahead). [github-integration.md](github-integration.md). GitHub OAuth **login** is separate → w4/003. |
| Header rules · redirects / rewrites | — | — | — | — | Render's static-site-only edge rules (`/headers`, `/routes`). bex serves web/private services, which have no such rules — non-goal for those types; revisit only if w1/012 adds static sites. |

## Logs

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Application logs (query + live tail) | ✅ | ✅ | ✅ | ✅ | `GET /v1/logs` + `/v1/logs/subscribe`; GraphQL `logs`; MCP `list_logs`; Logs tab (w3/m1, w5/m6). **Durable history (w3/m5):** with `BEX_LOKI_URL` set the query reads a Loki store (log-shipper DaemonSet) so logs survive pod restarts and time-range is a real bounded search; 7-day retention = Render's Hobby window (Render tiers 7/14/30 by plan). Pure `QueryLogs` backend swap — shapes/limits identical, byte-identical fallback when unset. **Divergence:** live tail is **SSE** where Render upgrades to WebSocket (and always reads pod logs, not Loki). [observability.md](observability.md). |
| Request / HTTP logs + structured filters | ✖ | ✖ | ✖ | ✖ | Render filters `level`/`statusCode`/`method`/`path`/`instance`/`host`/`direction` + MCP `list_log_label_values`. bex sources application logs only (accepted-but-empty for `type=request`/`build`). → **w3/002**. |
| Log streams (external drains) | — | — | — | — | Render `owner-log-stream`/`resource-log-streams`. External log-drain integration — non-goal. |

## Metrics

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Core metrics (CPU · memory · instance-count · HTTP requests · latency · bandwidth) | ✅ | ✅ | ✅ | ✅ | 6 REST endpoints under `/v1/metrics/*`; GraphQL `metrics(…)` + dashboard companions (`monthToDateBandwidth`, `metricsFilters`); MCP `get_metrics`; Metrics tab (w3/m2–m4.5). CPU/mem need metrics-server/Prometheus (503 otherwise). [observability.md](observability.md). |
| Extended metrics (cpu/mem limit & target · disk · active-connections · replication-lag) | ◐ | ◐ | ◐ | ◐ | Render exposes autoscale-target, disk, and DB-connection series. bex has CPU/mem as % of limit but not the target/disk/connection series → follows **w1/008** (autoscaling) + **w1/m17** (Postgres). |
| Metric streams (external) | — | — | — | — | Render `owner-metrics-stream`. External metrics-drain — non-goal. |

## Identity, workspaces & account

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| API-key management (create · list · revoke · metadata) | ✅ | ✅ | ✅ | ✅ | `apikeys/rest.go` `/v1/api-keys`; GraphQL `apiKeys`/`createApiKey`/`revokeApiKey`; MCP `create/list/revoke_api_key`; dashboard mint/list/revoke (**w4/m8**). **bex is ahead of Render** — Render has _no_ REST API-key surface (dashboard-only), so the comparison target is bex's own cross-surface consistency, not a Render shape. **w4/m13** added hygiene metadata — `createdBy` + `lastUsedAt` (last-used recorded off the request path, throttled) — carried identically by all four surfaces, plus a deliberate access-token TTL ([auth.md §8](auth.md)). |
| Login · sessions · account settings | — | — | — | ✅ | bex uses Ory Kratos (not a bex-api resource): auth pages + `/settings`. [auth.md](auth.md). |
| Email recovery / verification | — | — | — | ◐ | Dashboard forgot/reset pages shipped; live SMTP courier → **w4/m7**. |
| MFA (TOTP / passkeys) | — | — | — | ✅ | Kratos-native, no bex-api resource (like Login/sessions above — REST/GraphQL/MCP N/A, the surface is the dashboard). **w4/m11** enables `totp` + `webauthn` (**second factor**, `passwordless: false`) + `lookup_secret` recovery codes with a `highest_available` AAL policy ([kratos.values.yaml](../deploy/gitops/base/values/kratos.values.yaml)); Ory Elements renders enroll/unenroll in `/settings` and the login page's aal2 challenge (`use-ory-flow.ts` mints the `aal=aal2` step-up flow). Matches Render's 2FA (authenticator-app TOTP in account settings, challenge at login, recovery codes) — [Render 2FA docs](https://render.com/docs/2fa). **Superset:** bex adds WebAuthn security keys as a second factor. TOTP + recovery-code lifecycle scripted ([auth-mfa-e2e.sh](../scripts/auth-mfa-e2e.sh)); WebAuthn is a manual browser check. See [auth.md §9](auth.md). |
| Workspace members & roles (list · invite · change role · remove) | ✅ | ✅ | ✅ | ✅ | `members/rest.go` `/v1/workspaces/{id}/members` + `/invites`; GraphQL `workspaceMembers`/`workspaceInvites` + `inviteWorkspaceMember`/`changeWorkspaceMemberRole`/`removeWorkspaceMember`/`revokeWorkspaceInvite`; MCP 6 tools; dashboard Settings → Team (**w4/m12**). Writes `tenant_members` + OpenFGA role tuples together; the docs/auth.md role matrix is enforced (invited viewer reads but can't mutate); last admin can't self-demote/remove. Roles are Render's UPPERCASE enum (`render-artifacts/team-members.graphql`). **Divergence (◐ shape):** members are keyed by identity subject, not `user{email,name}` (no per-member profile store yet); bex flattens Render's `owner.team.members` nesting into workspace-scoped queries (bex has no polymorphic `owner`). See [members.md](members.md). |
| Audit logs | ◐ | ◐ | — | ◐ | `internal/audit` (`w4/m10`): `GET /v1/owners/{ownerId}/audit-logs` (Render's workspace path — `/organizations/{id}/audit-logs`, the org-level surface, is out of scope: bex has no org layer above workspace) + GraphQL `auditLogs(ownerId, startTime, endTime, cursor, limit)`, both delegating to one `Service.List` verb (`can_manage`-scoped, 503 store-less) — proven identical by `TestAuditSurfaceParity`. Query params (`startTime`/`endTime`/`cursor`/`limit`) match Render's documented vocabulary; `direction` is accepted and ignored (bex always returns newest-first). **Divergence (◐ shape):** Render's exact JSON response schema wasn't resolvable from public docs at authoring time (api-docs.render.com's reference page documents only the request parameters) — bex's fields (`id`/`timestamp`/`actor`/`actorMethod`/`action`/`status`/`resource`) are a best-effort rendering of Render's documented dashboard columns (Timestamp/Actor/Event/Status/Metadata), and the list envelope follows bex's own established `{object, cursor}` per-item shape (deploys/env-vars) rather than an unverified Render envelope. `status` is `"success"`/`"denied"` (bex's outcome is a binary authorize allow/deny, not Render's success/error). Only **write**-tier verbs are recorded (`can_operate`/`can_create`/`can_manage_keys`/`can_manage`); read-verb denials are a documented future extension (same event shape, no migration needed). MCP intentionally out of scope (Render's own MCP server has no audit-log tool either). **UI (w4/m14):** dashboard Settings → Audit Log card (admin-only via the same `can_manage` gate — a 403 hides the card rather than erroring; a store-less 503 shows an explanatory state), newest-first table + "Load more" cursor paging. **Divergence (◐, IA):** the milestone's own premise ("Render places Audit Log next to Team Members") didn't hold on re-check (t007, 2026-07-11) — Render's dashboard has **no in-app browsable table at all**, only a date-range CSV-export button under Workspace Settings → Compliance, not beside Team/Members; bex's placement and interactive-table treatment is a deliberate superset, not a mirror (tracked in [§ bex ahead of Render](#bex-ahead-of-render); follow-up filed as `w4/007`). See [bex-api.md § Audit log](bex-api.md#audit-log-w4m10). |
| SSO / SAML · SCIM | — | — | — | — | Enterprise; Ory can add later. Non-goal for now. |
| SSH keys | — | — | — | — | User SSH keys serve the Shell/SSH surface, which is off-roadmap. Non-goal. |

## API contract

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Rate limits (429 + Retry-After) | ✅ | ✅ | ✅ | — | Render: 500 req/min per API key, HTTP 429 + `Retry-After` (api-docs.render.com/reference/rate-limiting; OpenAPI `429` per endpoint). bex: per-caller token-bucket middleware at the shared mux — same 500/min default, same `{"id":"rate_limited","message":"…"}` REST body, `{"data":null,"errors":[{"message":"…","extensions":{"code":"RATE_LIMITED"}}]}` GraphQL envelope, HTTP 429 for MCP. `Retry-After` on all three. Webhook (`/v1/webhooks/git`) and healthz exempt. Env-tunable: `BEX_RATE_LIMIT` (fill rate, 0=disabled), `BEX_RATE_BURST`. Companion caps: body size 2 MiB (`BEX_MAX_BODY_BYTES`), query window 720h (`BEX_MAX_QUERY_HOURS`), SSE connections 100 (`BEX_MAX_SSE_CONNS`). **Single-replica caveat**: effective per-caller budget = BEX_RATE_LIMIT × replicas; distributed counter is the multi-replica follow-up. (w7/m3) |

## Platform events & integrations

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Notifications (deploy/failure alerts) | ✖ | ✖ | ✖ | ✖ | Render `/notification-settings` + Integrations. Untracked, low — depends on first-class deploy/events (**w2/m5**). |
| Outbound event webhooks | ✖ | ✖ | ✖ | ✖ | Render `/webhooks` (Render → you). bex has the _inbound_ git push webhook instead ([§ bex ahead](#bex-ahead-of-render)). Low, untracked. |
| Maintenance runs | — | — | — | — | Render `/maintenance`. Managed-infra scheduling — non-goal. |
| Dedicated outbound IPs | — | — | — | — | Render `/dedicated-ips`. Infra/enterprise — non-goal. |
| Workflows & tasks (Beta) | — | — | — | — | Render's orchestration product (`/workflows`, `/tasks`, `/task-runs` + SSE) — now a full surface (≥14 endpoints), not just a Beta. Non-goal for bex (off-roadmap orchestration, not a hosting primitive). |

---

## bex ahead of Render

Verbs where bex's AI-native posture exposes _more_ than Render does — tracked here so they read as deliberate supersets, not accidental drift:

- **API-key management over the API** — `POST/GET/DELETE /v1/api-keys` + GraphQL + MCP. Render mints keys in the dashboard only (account-scoped, non-expiring); bex makes them a first-class, workspace-scoped, revocable OAuth2-client resource an agent can rotate itself ([auth.md](auth.md)).
- **Deploy-from-chat** — MCP `deploy {repo, bexYaml}` rides the same `Create` verb (no bespoke endpoint): one call takes a repo + `bex.yml` to a live URL ([deploy-from-chat.md](deploy-from-chat.md)).
- **Inbound push-to-deploy webhook** — `POST /v1/webhooks/git` (HMAC-SHA256, outside the OAuth gate). Render's public `/webhooks` are _outbound_; bex additionally accepts the git host's push.
- **Usage metering** — `GET /v1/usage` + GraphQL `usage` query + MCP `get_usage` tool return month-to-date instance-seconds (per tier), egress bytes, and build seconds for the caller's workspace. Render's billing surface is dashboard-only (no REST/GraphQL/MCP billing endpoints — verified against Render's OpenAPI spec 2026-07-09). See [docs/usage-metering.md](usage-metering.md).
- **In-app Audit Log viewer** — dashboard Settings → Audit Log (`w4/m14`) is a live, paginated, in-app table. Render's dashboard has no in-app audit-log table at all — only a date-range CSV export under Workspace Settings → Compliance (render.com/docs/audit-logs, checked 2026-07-11).
- **GitHub repo listing & connection status over the API** — `GET /v1/repos` + MCP `list_repos`/`get_git_connection` return the connected installation's repositories (private included) and the connection's account/install URL, so "which of my repos can you deploy?" is one agent call. Render lists repos only through its private dashboard API and its MCP has no repo tools (w2/m8, [github-integration.md](github-integration.md)).

Read-only SQL over MCP (`query_render_postgres`) is parity, not a superset — Render is MCP-only there too.

## Gap backlog

Every `✖`/`◐` worth doing, mapped to its owning milestone or inbox note (nothing here is filed twice; new notes were opened only where no owner existed):

| Gap | Owner | Status |
| --- | --- | --- |
| Delete service | `w2/m4` · `w5/m14` | done 2026-07-09 (REST/GraphQL/MCP); dashboard danger-zone UI done 2026-07-11 (`w5/m14`) |
| Deploy objects (list/get/trigger/cancel) + rollback | `w2/m5` | todo |
| Manual-scaling control in dashboard | `w5/004` | todo (blocked) |
| Custom-domain DNS/CNAME instructions in dashboard | `w5/006` | done (w5/m10) |
| Key Value (Valkey/Redis) store | `w1/m14` · `w2/m7` · `w5/m12` | done — mechanism (CR + reconciler, live in prod 2026-07-09); REST/GraphQL/MCP (w2/m7); dashboard (w5/m12, 2026-07-09) |
| API keys in the dashboard | `w4/m8` | done 2026-07-08; key metadata (created-by/last-used) + token TTL → `w4/m13` done 2026-07-09 |
| Workspace members & roles | `w4/m12` | done 2026-07-09 — invite/list/change-role/remove across all four surfaces + invite-accept-on-login ([members.md](members.md)) |
| Audit logs | `w4/m10` + `w4/m14` | done 2026-07-11 (REST + GraphQL, admin-scoped, read-only, `w4/m10`; dashboard Settings → Audit Log card, `w4/m14`) — MCP still out of scope (Render's own MCP has no audit-log tool); IA-placement drift filed as `w4/007` |
| Git connections (GitHub App): connect + repo list + private deploy + zero-config push | `w2/m8` · `w2/m9` | done 2026-07-11 (all four surfaces; GitLab/Bitbucket providers remain ◐, untracked) |
| Health-check path → readiness probe | `w1/005` | todo |
| Env groups + secret files | `w1/m16` | todo |
| Per-service autoscaling config | **`w1/m20`** | done 2026-07-11 (HPA-style reconciler + REST/GraphQL/MCP surfaces + dashboard Scaling tab; CRD `AutoscalingSpec`, 5-min scale-down stabilization window) |
| Postgres advanced lifecycle & data protection | `w1/m17` (HA → `w1/013`) | done 2026-07-09 (backups+PITR, suspend/resume/restart, IP allowlist + PgBouncer pooler + users, across REST/GraphQL/MCP + dashboard; HA/replicas → `w1/013`) |
| Additional service types: background worker + cron job | `w1/m15` | done 2026-07-09 (static site split → `w1/012`) |
| Request/HTTP logs + structured filters | **`w3/002`** (new) | todo |
| Projects & environments; registry creds; notifications; outbound webhooks; PR previews; blueprint resource | untracked (low) | — (rationale inline above) |

Deliberate non-goals (marked `—`): persistent disks, shell/SSH & one-off exec, static-site edge rules, SSO/SAML/SCIM, log/metric streams, maintenance, dedicated IPs, workflows — see the rationale in each row and [DO_NOT_DO.md](../.pm/DO_NOT_DO.md).

## Evidence sources

- **bex side** — [bex-api.md](bex-api.md) (design + verified-parity notes); `lego/backend/internal/{apps,logs,metrics,apikeys,postgres,secrets}/{rest,graphql,mcp}.go`; `dashboard/src/routes/` + `features/`.
- **Render REST** — OpenAPI spec `https://api-docs.render.com/openapi/render-public-api-1.json`; reference `https://api-docs.render.com/reference/introduction`; endpoint index `https://api-docs.render.com/llms.txt`.
- **Render MCP** — `render-oss/render-mcp-server` v0.3.0 (24 tools): `cmd/server.go` + `pkg/*/tools.go`; docs `https://render.com/docs/mcp-server`.
- **Render dashboard IA** — `render.com/docs` (service-metrics, logging, deploys, scaling, disks, jobs, ssh, custom-domains, postgresql\*, configure-environment-variables, team-members, notifications, audit-logs).
- **Render GraphQL** — captured live in [render-artifacts/](render-artifacts/) (`team-members.graphql`); the dashboard operation names bex mirrors are noted in [bex-api.md](bex-api.md).
