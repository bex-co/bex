# Render parity matrix — REST · GraphQL · MCP · UI

A single, evidence-based map of how far bex actually matches [render.com](https://render.com), one row per Render capability across the four surfaces bex mirrors. It replaces scattered "Render-compatible" assertions with a checkable ledger: what has parity, what diverges, what is missing, and what is a deliberate non-goal — each cell backed by a pointer to code (bex side) or Render's own spec/docs (Render side).

**Method.** Four passes, 2026-07-08, re-verified against Render's live API/docs 2026-07-09: bex's public REST vs Render's OpenAPI spec; bex's GraphQL vs Render's captured dashboard operations; bex's MCP tools vs `render-oss/render-mcp-server` v0.3.0 (24 tools — incl. three non-functional `update_{web_service,static_site,cron_job}` stubs that return "use the dashboard/API"; Render's only functional MCP write is the single env-var tool `update_environment_variables`); bex's dashboard IA vs Render's (via render.com/docs — the live dashboard needs a login). The design record for bex's side is [bex-api.md](bex-api.md) ("one Core, three adapters"); this doc is the parity ledger over it.

**Legend.** ✅ parity (evidence pointer) · ◐ partial (divergence documented) · ✖ missing (gap — see backlog) · — deliberate non-goal (rationale inline). "Render capability" = a noun/verb Render exposes on _any_ of its surfaces; a `—` on the REST column can still be a `✅` elsewhere (Render itself splits work across surfaces — e.g. read-only SQL is MCP-only on both sides).

> **Headline.** bex has real parity on the **service lifecycle, env vars, custom domains, managed Postgres, logs, and metrics** core across all four surfaces, and is _ahead_ of Render on three AI-native verbs (API-key management over the API, deploy-from-chat, an inbound push webhook — [§ bex ahead of Render](#bex-ahead-of-render)). The open frontier is **deploys as first-class objects, config surfaces beyond plain env vars (env groups, secret files), autoscaling config, additional resource types (Key Value, static sites), and advanced Postgres data-protection** — all mapped to owning milestones/notes in the [§ Gap backlog](#gap-backlog). (Background-worker and cron-job service types shipped in w1/m15.)

---

## Services & lifecycle

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| List services | ✅ | ✅ | ✅ | ✅ | `apps/rest.go` `GET /v1/services`; GraphQL `services`; MCP `list_services`; dashboard `routes/index.tsx`. Omits Render `includePreviews` (no preview services). |
| Get service | ✅ | ✅ | ✅ | ✅ | `GET /v1/services/{id}`; GraphQL `server`/`service`; MCP `get_service`; Overview tab. bex extras `phase`/`replicas`/`revision` (superset Render clients ignore). |
| Create service (web / private) | ◐ | ◐ | ◐ | ✖ | `POST /v1/services` upsert; GraphQL `createService`; MCP `create_web_service` + `deploy`. Omits Render `region`/`runtime`/`buildCommand`/`startCommand` (Dockerfile/CNB auto-detect); no dashboard create wizard (API-first; deploy-from-chat is the path — w2/m2). |
| Health checks (path → readiness gating) | ◐ | ✖ | ✖ | ✖ | `serviceDetails.healthCheckPath` is accepted into `spec.healthCheckPath` (`apps/deploy.go`, `apps/rest.go`) but the operator never wires it to a ReadinessProbe — Running gates on replica-readiness only. Not on GraphQL/MCP create or the dashboard. → **w1/005**. |
| Change instance plan / type | ✅ | ✅ | ✅ | ✅ | `PATCH /v1/services/{id}` (plan); GraphQL `updateServicePlan`; MCP `update_service_plan`; Plan-picker page. Broader `PATCH` fields (name, autoDeploy, rootDir, buildFilter) not editable — ◐, low. |
| Suspend / Resume | ✅ | ✅ | ✅ | ✅ | `POST …/suspend` (202) · `…/resume` (202); GraphQL `suspendService`/`resumeService`; MCP `suspend_service`/`resume_service`; row + header actions. Render parity verified in [bex-api.md](bex-api.md). |
| Restart | ✅ | ✅ | ✅ | ✅ | `POST …/restart` (200); GraphQL `restartServer`; MCP `restart_service`; header action. Render's official MCP omits these — bex adds them (named after Render's REST verbs). |
| Manual scale (instance count) | ✅ | ✅ | ✅ | ✖ | `POST …/scale`; GraphQL `scaleService`; MCP `scale_service` (backend shipped w2/m12). Dashboard stepper → **w5/004**. |
| Autoscaling config (min/max + CPU/mem target) | ✖ | ✖ | ✖ | ✖ | Render `PUT`/`DELETE /services/{id}/autoscaling`. bex has no per-service autoscaler. → **w1/008** (mechanism leans on w1/m3 bin-pack/autoscale). |
| Delete service | ✖ | ✖ | ✖ | ✖ | Render `DELETE /services/{id}`. Not built (scope note in [bex-api.md](bex-api.md)). → **w2/m4**. |
| Service events / activity feed | ✖ | ✖ | ✖ | ✖ | Render `GET /services/{id}/events`. bex has no event objects. → **w2/m5** (deploy objects) + **w4/m10** (audit log). |
| Cache purge | — | — | — | — | Render `POST …/cache/purge` (static-site CDN). bex has no build CDN cache — non-goal. |
| Background worker (no HTTP port) | ✅ | ✅ | ✅ | ✅ | `spec.type=background_worker` → Deployment only, no Service/Ingress/URL (`app_controller.go`); create over `POST /v1/services`, GraphQL `createService(type:)`, MCP `create_web_service(type:)`; dashboard type badge + no-URL. (w1/m15) |
| Cron job (schedule + run history) | ✅ | ✅ | ✅ | ✅ | `spec.type=cron_job` + `spec.schedule` → k8s CronJob, `status.runs` (`app_controller.go`); run trigger `POST /v1/cron-jobs/{id}/runs` / GraphQL `runCronJob` / MCP `run_cron_job`; create MCP `create_cron_job` (tracks Render's tool); dashboard shows schedule + recent runs. (w1/m15) |
| Static site | ✖ | ✖ | ✖ | ✖ | Render `static_site` type (build → CDN with redirects/rewrites/headers). A larger build→CDN effort than the compute types. → **w1/012**. |
| One-off jobs (run a command) | — | — | — | — | Render `/services/{id}/jobs` runs an arbitrary command in the service context — an execution surface, off-roadmap (`DO_NOT_DO` §pillar 5), the same call as Shell/SSH below. (Scheduled cron jobs are a service type, tracked separately → w1/m15.) |
| Shell / SSH into a running instance | — | — | — | — | Render Shell tab / `render ssh`. No exec surface — hosted execution is off-roadmap (DO_NOT_DO §pillar 5). Non-goal for now. |
| PR preview environments | ✖ | ✖ | ✖ | ✖ | Render `POST …/previews` (plural) + Previews tab. Ties to git integration + deploys; low priority, untracked. |

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
| Secret files (mounted at `/etc/secrets`) | ✖ | ✖ | ✖ | ✖ | Render `/services/{id}/secret-files`. bex omits (safe-superset note in [bex-api.md](bex-api.md)). → **w1/m16**. |
| Environment groups (+ link / unlink) | ✖ | ✖ | ✖ | ✖ | Render `/env-groups` + link/unlink. bex has no shared env-var sets. → **w1/m16**. |

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
| Lifecycle (suspend / resume / restart) | ✖ | ✖ | ✖ | ✖ | Render `POST /postgres/{id}/{suspend,resume,restart}`. Deferred in [bex-api.md](bex-api.md). → **w1/m17**. |
| Backups · PITR / recovery | ✖ | ✖ | ✖ | ✖ | Render `recovery-info`/`recover`/`exports` + Recovery tab. Needs CNPG backup wiring. → **w1/m17**. |
| HA · failover · read replicas | ✖ | ✖ | ✖ | ✖ | Render `failover`/`promote`/`replication`. `highAvailabilityEnabled` is reported `false` today. → **w1/013** (deferred from m17). |
| Access control (IP allowlist) · users · pooler | ✖ | ✖ | ✖ | ✖ | Render `ipAllowList`, `/users`, PgBouncer pooler strings. bex has a `public` toggle only. → **w1/m17**. |
| Postgres observability (live queries · top-queries · sizes · table-scans · param overrides) | ✖ | ✖ | ✖ | ✖ | Render `GET /postgres/{id}/{processes,top-queries,sizes,table-scans}` + `parameter-overrides`. Runtime introspection over `pg_stat_activity`/`pg_stat_statements`. bex has none. Untracked, low (extends **w1/m17**). |

## Other datastores & storage

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Key Value (Valkey / Redis) | ✖ | ✖ | ✖ | ✖ | Render `/key-value` (full CRUD + `connection-info` + `suspend`/`resume`, 8 endpoints) + MCP `list/get/create_key_value` + dashboard type ("Key Value", Valkey 8). Mechanism shipped (w1/m14): a `KeyValue` CR → single-instance Valkey + internal Service DNS + credentials Secret, optional public Traefik TCP/SNI route. The four surfaces (REST/GraphQL/MCP/UI) are still unbuilt → **w2/m7** (REST/GraphQL/MCP) + **w5/m12** (dashboard). Surface contract to mirror: plans `free`/`starter`/`standard`/… (the web-service vocabulary, **not** Postgres `basic-*`); connection-info 3 keys — `internalConnectionString` (`redis://`), `externalConnectionString` (`rediss://` TLS, opt-in), `cliCommand`; create/update fields Render has that the CR lacks today — `maxmemoryPolicy`, `persistenceMode`, `ipAllowList` (CIDR). Internal URL is unauthenticated by default in Render; bex's mechanism always mints a password. |
| Persistent disks | — | — | — | — | Render `/disks` + Disks tab. Deliberate: bex is **stateless-first** (managed Postgres for state); disks disable multi-instance + zero-downtime deploys, which fights bex's dense bin-pack + free-tier-sleep economics. Non-goal. |

## Deployment sources & IaC

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Blueprint / `render.yaml` IaC | ◐ | ✖ | ◐ | ✖ | bex consumes a `render.yaml`-shaped `bex.yml` via `deploy` (MCP) + `scripts/app-apply.sh`, but exposes no `/blueprints` resource (validate/list/sync). → extends **w2/m2**; resource untracked, low. |
| Projects & environments (grouping) | ✖ | ✖ | ✖ | ✖ | Render `/projects`, `/environments`, protected environments. bex is flat apps in one workspace. Belongs to the tenancy line → nearest **w1/m9**; low. |
| Registry credentials (private images) | ✖ | ✖ | ✖ | ✖ | Render `/registrycredentials`. bex pulls from its own zot registry; external private registries unsupported. Low, untracked. |
| Git connections (GitHub / GitLab app) | ◐ | ✖ | ✖ | ✖ | Repo URL + HMAC push webhook works; no managed OAuth git-app connection. GitHub OAuth **login** → w4/003. Low. |
| Header rules · redirects / rewrites | — | — | — | — | Render's static-site-only edge rules (`/headers`, `/routes`). bex serves web/private services, which have no such rules — non-goal for those types; revisit only if w1/012 adds static sites. |

## Logs

| Render capability | REST | GraphQL | MCP | UI | Evidence / divergence |
| --- | :-: | :-: | :-: | :-: | --- |
| Application logs (query + live tail) | ✅ | ✅ | ✅ | ✅ | `GET /v1/logs` + `/v1/logs/subscribe`; GraphQL `logs`; MCP `list_logs`; Logs tab (w3/m1, w5/m6). **Divergence:** live tail is **SSE** where Render upgrades to WebSocket. [observability.md](observability.md). |
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
| API-key management (create · list · revoke) | ✅ | ✅ | ✅ | ✖ | `apikeys/rest.go` `/v1/api-keys`; GraphQL `apiKeys`/`createApiKey`/`revokeApiKey`; MCP `create/list/revoke_api_key`. **bex is ahead of Render** — Render has _no_ REST API-key surface (dashboard-only). Dashboard mint/list/revoke → **w4/m8**. |
| Login · sessions · account settings | — | — | — | ✅ | bex uses Ory Kratos (not a bex-api resource): auth pages + `/settings`. [auth.md](auth.md). |
| Email recovery / verification | — | — | — | ◐ | Dashboard forgot/reset pages shipped; live SMTP courier → **w4/m7**. |
| MFA (TOTP / passkeys) | — | — | — | ✖ | Kratos-native → **w4/m11**. |
| Workspace members & roles | ✖ | ✖ | ✖ | ✖ | Render `/owners/{id}/members` + Team page. Roles already modelled in OpenFGA (`model.fga`); captured contract in [render-artifacts/team-members.graphql](render-artifacts/team-members.graphql). → **w4/m12** (gated on w1/m9). |
| Audit logs | ✖ | ✖ | ✖ | ✖ | Render `/owners/{id}/audit-logs` (workspace) **and** `/organizations/{id}/audit-logs` (org). → **w4/m10**. |
| SSO / SAML · SCIM | — | — | — | — | Enterprise; Ory can add later. Non-goal for now. |
| SSH keys | — | — | — | — | User SSH keys serve the Shell/SSH surface, which is off-roadmap. Non-goal. |

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

Three verbs where bex's AI-native posture exposes _more_ than Render does — tracked here so they read as deliberate supersets, not accidental drift:

- **API-key management over the API** — `POST/GET/DELETE /v1/api-keys` + GraphQL + MCP. Render mints keys in the dashboard only (account-scoped, non-expiring); bex makes them a first-class, workspace-scoped, revocable OAuth2-client resource an agent can rotate itself ([auth.md](auth.md)).
- **Deploy-from-chat** — MCP `deploy {repo, bexYaml}` rides the same `Create` verb (no bespoke endpoint): one call takes a repo + `bex.yml` to a live URL ([deploy-from-chat.md](deploy-from-chat.md)).
- **Inbound push-to-deploy webhook** — `POST /v1/webhooks/git` (HMAC-SHA256, outside the OAuth gate). Render's public `/webhooks` are _outbound_; bex additionally accepts the git host's push.

Read-only SQL over MCP (`query_render_postgres`) is parity, not a superset — Render is MCP-only there too.

## Gap backlog

Every `✖`/`◐` worth doing, mapped to its owning milestone or inbox note (nothing here is filed twice; new notes were opened only where no owner existed):

| Gap | Owner | Status |
| --- | --- | --- |
| Delete service | `w2/m4` | todo |
| Deploy objects (list/get/trigger/cancel) + rollback | `w2/m5` | todo |
| Manual-scaling control in dashboard | `w5/004` | todo (blocked) |
| Custom-domain DNS/CNAME instructions in dashboard | `w5/006` | done (w5/m10) |
| Key Value (Valkey/Redis) store | `w1/m14` | mechanism done (CR + reconciler, live in prod 2026-07-09); surface → `w2/m7` (API) + `w5/m12` (dashboard) |
| API keys in the dashboard | `w4/m8` | done 2026-07-08 (key metadata follow-up → `w4/m13`) |
| Workspace members & roles | `w4/m12` | todo (gated on w1/m9) |
| Audit logs | `w4/m10` | todo |
| Health-check path → readiness probe | `w1/005` | todo |
| Env groups + secret files | `w1/m16` | todo |
| Per-service autoscaling config | **`w1/008`** (new) | todo |
| Postgres advanced lifecycle & data protection | `w1/m17` (HA → `w1/013`) | todo |
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
