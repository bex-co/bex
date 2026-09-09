# ADR088 — Platform observability UI: Grafana at obs.bex.co behind the ops-workspace gate

**Status:** Accepted (2026-09-07). Source: platform-monitoring discussion 2026-09-07 (w5). Composes with [ADR001](ADR001-go-and-gitops.md) (product ≠ GitOps), [ADR010](ADR010-observability.md) (customer-facing logs/metrics), [ADR012](ADR012-auth.md) (Ory Kratos + Hydra), [ADR024](ADR024-members.md) (workspace members & roles), and the [ADR072](ADR072-security-review-round7.md) §1 `onbex.co` cookie-isolation disposition.

## Context

The platform GitOps baseline already runs the monitoring backends, all in the `monitoring` namespace that tenant pods are denied from reaching (`network-policies.yaml`): Prometheus scrapes Traefik and platform services and evaluates the alert rules Alertmanager emails out (`deploy/gitops/base/prometheus.yaml`), Loki stores durable log history (`loki.yaml`), and an Alloy DaemonSet ships App pod logs into it (`log-shipper.yaml`). What is missing is a human UI: today an operator reads these backends through kubectl port-forwards and raw PromQL/LogQL. [ADR010](ADR010-observability.md)'s logs and metrics are the **customer** product surface — per-service, tenant-scoped, served by bex-api — and are deliberately not a platform-operations view.

Three questions are decided here: where the UI runs, what hostname it gets, and who can log in.

## Decision

### 1. Grafana as platform GitOps infrastructure

Grafana is deployed by an Argo CD Application in `deploy/gitops/base/` (per-env differences in the overlays, like every sibling), into the `monitoring` namespace alongside the backends it reads. Datasources (Prometheus, Loki) and dashboards are **provisioned as code** — committed JSON, no click-ops state to lose. The initial dashboard set targets platform availability, bex-api errors/latency, operator reconciliation, builds, databases (CNPG), and cluster capacity; it may start small and grow, but every dashboard lives in git.

Explicit non-choices:

- **Not a bex-hosted customer App.** Our operations tooling must not depend on the product it exists to observe.
- **Not folded into the product dashboard.** Customer service metrics remain ADR010's surface; platform operations is a different audience with different data.
- **Not an outage-proof monitoring plane.** Grafana in the production cluster goes down with the cluster. The external uptime check remains the whole-cluster-outage detector; moving the observability stack (or a telemetry copy) outside the production cluster is deferred until blast-radius goals demand it.

### 2. Hostname: `obs.bex.co`

The existing platform hosts are role-named, never tool-named — `api.`, `auth.`, `oauth.`, `dashboard.bex.co`, not `kratos.` or `hydra.` — so the observability portal is **`obs.bex.co`** (letsencrypt-prod TLS, prod overlay). Rejected:

- `metrics.bex.co` — the UI fronts logs (and eventually traces) too, and "metrics" reads as a scrape/ingest endpoint, not a human portal.
- `grafana.bex.co` — welds today's tool choice into DNS, certificates, bookmarks, and the OAuth redirect URI.
- anything under `onbex.co` — ADR072 §1: `onbex.co` is not on the PSL, so every tenant origin shares its registrable-domain cookie scope; an admin-authenticated UI must stay on the control-plane domain, out of tenant cookie-tossing range.

### 3. AuthN: an ordinary Hydra OIDC client

Grafana authenticates against the existing platform issuer (`https://oauth.bex.co`) as one more first-party client — no new identity system:

- Registered idempotently by `scripts/auth-bootstrap-client.sh` beside `bex-mobile`: confidential client, `authorization_code` + `refresh_token`, exact redirect `https://obs.bex.co/login/generic_oauth`, `skip_consent: true`.
- Scopes are identity-only (`openid profile email`) with **no access-token audience** — a Grafana token carries zero bex-api authority, so this adds no surface to the product API.
- Grafana's generic-OAuth config enables PKCE S256 — the consent gate (`hydra-consent.ts` `pkceSatisfied`) refuses any authorization-code flow without it.
- Login rides the existing Kratos-native bridge; an operator with a live dashboard session reaches Grafana with no consent screen (the trusted headless path).
- The client secret and Grafana's break-glass local-admin password live in SealedSecrets ([ADR016](ADR016-sealed-secrets.md)); names-only in `.env.example`.

### 4. AuthZ: the ops-workspace gate

Kratos is the **customer** identity pool, so authentication alone would admit every bex user. The gate is membership in a designated ops workspace — an ordinary `tea-*` workspace pinned by deployment config, whose membership (ADR024 tuples, invites, members UI) becomes the Grafana ACL:

- **bex-api** learns the id via `BEX_OPS_WORKSPACE` and exposes a server-only internal verb answering "what is subject S's role in the ops workspace" (one OpenFGA read), guarded by a static bearer (`BEX_OPS_ROLE_TOKEN`) and reachable only in-cluster.
- **The consent acceptor** (`dashboard/src/common/server-fn/hydra-consent.ts`) gains an ops-gated client class (`OAUTH_OPS_CLIENTS`, today just Grafana's client id): before **any** accept — including the trusted/skip headless path — it resolves `consent.subject`'s role through that verb. `admin` → GrafanaAdmin, `developer` → Editor, `viewer` → Viewer; `contributor`, `billing`, and non-members are rejected with `access_denied`. On accept it stamps `email`, `name`, and `ops_role` id_token claims. The gate runs before either path's accept call, so the "headless and human paths grant identically" invariant survives.
- **Grafana** maps `ops_role` via `role_attribute_path` with `role_attribute_strict` — defense in depth behind the server-side gate, not the gate itself. Token/session lifetimes stay short: role changes and removals take effect at next login (plus the 1h consent remember window), not mid-session.

The pinned workspace gets two guards: workspace deletion/suspension refuse it outright, and invite-time seat/plan gating (ADR024) exempts it — onboarding an operator must never be silently blocked by a seat cap, and deleting the workspace must never lock every operator out at once.

**Alternative considered:** granting operators tuples on `workspace:default` (the platform object ADR012 reserves for bootstrap) — no product-lifecycle semantics to guard, but no members UI or invite flow either; operator management would be out-of-band tuple writes. Rejected for now: the members surface is exactly the operator-management UX we want, and the consent-side gate is identical under either model, so switching later is cheap.

### 5. Environments and verification

The local overlay runs Grafana with only the local admin — no OIDC client, no public host — the same reduced shape as its other components; prod carries the full OIDC config. Verification extends the auth e2e family (`scripts/auth-oauth21-e2e.sh` pattern): an ops-workspace member completes the code flow and lands with the mapped role; a customer identity outside the ops workspace is denied at consent.

### 6. Key metrics / SLI baseline

Division of record: **this section says what we watch and why; the committed dashboard JSON says how it is drawn.** Two standing principles:

- **Every alert gets a panel.** Prometheus evaluates 48 platform alert rules today (`deploy/gitops/base/prometheus.yaml`); an alert only says "broken", so each rule's backing series must appear on a dashboard that answers _how bad, since when, trending which way_. A new alert rule lands with (or names) its panel; a panel-less alert is review feedback. `scripts/obs-coverage-check.sh` generates the audit and fails CI, so this is enforced rather than remembered.
- **Every tenant-facing surface gets a falsifiable signal** — an alert rule, a scheduled probe, or a written waiver. The per-surface ledger is the [coverage table](#tenant-facing-surface-coverage-w3m83-t001) below.
- **Bounded labels only.** Platform dashboards query the same bounded label sets the alert rules do — never per-path/per-host/per-tenant-unbounded dimensions (the ADR010 cardinality rule applies to dashboards too).

Per-dashboard SLIs and core series (all series names verified against the scrape/alert config and the `bex_*` registries; third-party exporters named by their real series):

| dashboard | SLI / question | core series |
| --- | --- | --- |
| **Platform availability** (landing page) | edge availability = 1 − 5xx ratio (5m), tied to `TraefikHigh5xxRate`'s threshold; edge latency p50/p95/p99; "is the platform itself up" | `traefik_service_requests_total`, Traefik service duration histogram; `kube_deployment_status_replicas_available` vs `kube_deployment_spec_replicas` (platform ns); `kube_pod_container_status_waiting_reason` (crashloops); `certmanager_certificate_expiration_timestamp_seconds` + `certmanager_certificate_ready_status`; `vault_core_unsealed` |
| **bex-api + gateways** | API error/latency from BOTH sides — Traefik's edge view per service and bex-api's own origin view per route pattern, surface, GraphQL operation, and MCP tool (w3/m84); agent-session SLOs (w5/m81 baseline, corrected w5/m88); gateway saturation; outbound eventing delivery (webhooks + push) | Traefik series filtered to the api service; `bex_api_http_request_duration_seconds{surface,route,method,status}`, `bex_api_http_requests_total{…}`, `bex_api_http_in_flight_requests{surface}`, `bex_api_graphql_operation_duration_seconds{operation,type,outcome}`, `bex_api_mcp_tool_duration_seconds{tool,outcome}`; `bex_agent_session_turn_outcomes_total{outcome}`, `bex_agent_session_turn_duration_seconds{outcome}` (running-only; started_at→terminal), `bex_agent_session_provision_seconds{outcome}`, `bex_agent_session_terminal_convergences_total`; `bex_ssh_gateway_active_sessions`/`_active_channels`/`_limit_rejections_total`/`_git_proxy_upstream_failures_total`; `bex_webhooks_delivery_admissions_total{result}`, `bex_webhooks_delivery_attempts_total{origin,result}`; `bex_push_last_success_timestamp_seconds`, `bex_push_queue_rows{state}`, `bex_push_enabled` |
| **Builds** | queue starvation (oldest wait), queue-time distribution, build success ratio, builder health | `bex_builds_active`/`bex_builds_queued`, `bex_build_queue_oldest_seconds`, `bex_build_queue_seconds_bucket`, `bex_build_run_seconds`, `bex_build_outcomes_total`, `bex_build_infra_failures_total`, `bex_build_clusterbuilder_ready`/`_present`/`_image_resolved_timestamp_seconds` |
| **Data plane** | tenant datastore readiness + PITR safety (WAL archiving must be 1); replication/backup freshness; public SNI front doors | `bex_datastore_ready`, `bex_datastore_wal_archiving`, `bex_datastore_age_seconds`, `bex_datastore_observe_errors_total`; `cnpg_backends_total`, `cnpg_pg_replication_*`, `cnpg_pg_stat_archiver_last_archived_time`; `kube_cronjob_status_last_successful_time` (backup CronJobs); `bex_pg_proxy_healthy`, `bex_kv_proxy_healthy` |
| **Billing + metering** | money-path freshness (outbox age), export integrity, meter integrity (a broken meter is silent revenue loss) | `bex_billing_outbox_oldest_pending_age_seconds`, `bex_billing_export_rejected_rows`/`_ambiguous_rows`, `bex_billing_webhook_last_success_timestamp_seconds`, `bex_billing_operations_total`, `bex_billing_enabled`; `bex_egress_meter_healthy`/`_counter_loss_events_total`/`_resource_map_pressure_ratio`, `bex_websocket_meter_healthy`, `bex_app_direct_egress_bytes_total` + pg/kv/websocket egress bytes |
| **Cluster capacity** | node headroom, PV fill (feeds `PersistentVolumeFillingUp` + the disk autoscaler), registry fill | `container_cpu_usage_seconds_total`, `container_memory_working_set_bytes` (cadvisor); `kube_node_status_condition`, `kube_node_info`/`kube_node_role`; `kubelet_volume_stats_used_bytes`/`_available_bytes`/`_capacity_bytes` (incl. the Zot PVC behind the `ZotRegistry*` alerts) |

#### Tenant-facing surface coverage (w3/m83 t001)

"Every alert gets a panel" is a rule about the alerts that exist. It says nothing about the surfaces that have **no** signal at all — and every silent outage in this repo's history has been of that second kind: a pipeline that returned `200` with no rows (`w3/m36` app logs, `w6/m131` request logs), an edge that answered TCP but never spoke SSH (`w6/m132`), a metric that stopped arriving (`w3/m110`). So the standing rule extends: **every tenant-facing surface gets a falsifiable signal**, and the form is chosen by where the failure is visible.

The choice is not a preference. A **Prometheus rule** is right when the failure is already visible in a series bex emits, because it costs nothing per evaluation and fires in minutes. A **scheduled probe** is the only option when the failure is visible only from the tenant's side — a query that returns empty, a URL that 404s, an invariant that needs a fixture to test — because no series exists to alert on. A surface at **none** is a surface where the next regression is found by a human.

Each "covered" cell below names the rule (`deploy/gitops/base/prometheus.yaml`) or the workflow job (`.github/workflows/*.yml`) that provides it, so the claim is checkable rather than asserted.

| tenant-facing surface | alert rule | scheduled probe | form + owner |
| --- | --- | --- | --- |
| App logs (`type=app`) | — | `request-logs-liveness` job, 6h (`STREAM_TYPES` includes `app`) | **probe** — w3/m83 t002. A dark pipeline is a `200 {"logs":[]}`, indistinguishable per-resource from a quiet service; only the fleet-wide label index decides it |
| Request logs (`type=request`) | — | `request-logs-liveness` job, 6h | **probe** — shipped w6/m131 t004 |
| Build logs (`type=build`) | — | — | **waived** — a build stream only exists while a build runs, so "no stream in 6h" is a true statement about a quiet fleet, not a fault. Its producer is covered by the build alerts (`BuildInfraSuccessBelowSLO`, `BuildQueuedTooLong`): a build that never ran cannot have logs |
| Datastore logs (`type=postgres`/`keyvalue`) | — | — | **waived** — same shipper DaemonSet and Loki as `type=app`, selected by an operator-stamped pod label; the app-log probe fails first and for the same reason. Revisit if a datastore-only pipeline defect is ever seen |
| Resource metrics (cpu/memory/instances) | — | `tenant-view-liveness` job, 6h | **probe** — w3/m83 t003. cAdvisor's own absence is what fails, and only a real App's series answers it |
| Request metrics | `TraefikHigh5xxRate` (the source series, not the tenant read) | `tenant-view-liveness` job, 6h | **probe** — w3/m83 t003 |
| Usage metering | `EgressMeterTargetMissing`, `EgressMeterUnhealthy`, `EgressMeterCounterLoss`, `EgressMeterMapPressure`, `WebSocketEgressMeterUnavailable`, `PostgresEgressProxyUnavailable`, `KeyValueEgressProxyUnavailable` | — | **covered by alerts** — every meter fails loudly rather than metering zero (ADR023) |
| Events feed | — | `tenant-view-liveness` job, 6h | **probe** — w3/m83 t003. The feed is a read VIEW over Postgres; its failure shape is an empty page |
| Outbound webhooks | `WebhookDeliveryAdmissionPressure`, `WebhookDeliveryFailing` | — | **covered by alerts** — w3/m83 t006 added the wire-failure ratio beside the queue-bound one |
| Push delivery | `PushDeliveryStale` | — | **alert** — w3/m83 t006 |
| Deploy email | — | — | **waived** — one SMTP relay, and `KratosCourierNotReady` already pages when it stalls (sign-up verification shares it, so a dead relay is caught at the higher-stakes surface first) |
| Deploy from git → Ready → URL | — | `deploy-canary` job, weekly | **probe** — w3/m83 t004. The Render promise itself; no series says "a push became a running URL" |
| Static-site serving + teardown | — | `deploy-canary` job (static variant), weekly | **probe** — w3/m83 t004. Teardown is the half that fails silently: a deleted site that keeps serving is invisible to every metric |
| Custom domains / TLS | `CertificateNotReady`, `CertificateExpiringSoon`, `TenantCustomDomainCertNotReady` (info) | `onbex-default-tls-verify` step, 6h | **covered** — cert-manager's own series plus the public wildcard-fallback synthetic |
| SSH edge | — | `ssh-kexinit-probe` step, 6h | **probe** — shipped w6/m132 t004. The failure was pre-authentication, so no in-cluster series saw it |
| Web Shell | — | `shell-ws-probe` step, 6h | **probe** — shipped w2/m90 (alive-but-refusing 401 is the healthy shape) |
| Agent sessions — provision | `AgentSessionProvisionFailing` | — | **alert** — w3/m83 t006 |
| Agent sessions — turn | — | — | **waived** — a turn's duration is dominated by the model and the task, so no threshold separates "slow platform" from "hard problem". `bex_agent_session_turn_duration_seconds` and `bex_agent_session_terminal_convergences_total` are paneled for the human read; provisioning is the half bex owns and it is alerted |
| Sandboxes | — | — | **none (accepted for now)** — sandbox creation outside an agent session has no first-party series and no probe. The agent-session provisioning alert covers the same OpenSandbox substrate through its busiest caller, so a substrate outage is caught; a `/v1/sandboxes`-specific regression is not. Owner: unfiled |
| Tenant / sandbox isolation | — | `isolation-matrix` job, weekly | **probe** — w3/m83 t005. An invariant, not a series: only an actual denied connection proves it |
| Datastore provisioning | `DatastoreStuckProvisioning`, `DatastoreObservationFailing` | — | **covered by alerts** — absence is the symptom, which is why the observation-error rule exists beside it |
| Datastore backups | `DatabaseNotArchivingWAL`, `BackupCronJobStale`, `PlatformDatabaseBackupStale` | — | **covered by alerts** |
| Billing | `BillingExportBacklog`, `BillingPermanentReject`, `BillingExportAmbiguity`, `BillingLocalStampFailure`, `BillingProviderDuplicate`, `BillingInvoiceReadDegraded`, `BillingWebhookDrift`, `BillingProvisioningFailure` | — | **covered by alerts** |

Two properties of this table are load-bearing:

- **A probe must be falsifiable.** A probe that cannot fail is worse than no probe, because it reports safety. Every row above that says "probe" asserts a property that was actually false during a real outage — tenant-namespace attribution, a KEXINIT byte, a 401 refusal shape — not merely that a call returned 200.
- **`scripts/obs-coverage-check.sh` guards the alert column only.** Its waiver list is about panels for alerts; the "none"/"waived" rows here are a different ledger, kept in this table, and each one states the reason in place rather than pointing at a waiver file.

#### Scheduled probes and the canary fixture (w3/m83)

An alert rule fires on a series that moved. It cannot fire on a series that is **silently empty**, and that is the failure this platform has hit repeatedly: `type=request` dark for every tenant for a month (w6/m131), the SSH edge never completing a handshake (w6/m132), metrics series that existed while bex-api's query named them wrong (w6/m110). Each was a 200 with no rows — indistinguishable, to any threshold, from a quiet platform. So dashboards and alerts are only half the coverage; the other half is a probe that **generates** the signal it then demands to read back. Falsifiable green: a probe that cannot go red is not coverage.

| probe | cadence | asks | red opens |
| --- | --- | --- | --- |
| `ssh-kexinit-probe.sh` | 6h | does `ssh.bex.co` complete a version exchange | `ssh-edge-down` |
| `onbex-default-tls-verify.sh` | 6h | trusted wildcard TLS + the intentional 404 | `onbex-fallback-tls-down` |
| `shell-ws-probe.sh` | 6h | does the Web Shell edge refuse ticketlessly with 401 | `shell-ws-edge-down` |
| `request-logs-liveness.sh` | 6h | do `type=request` streams exist for a **tenant** namespace | `request-logs-down` |
| `tenant-view-liveness.sh` | 6h | with a tenant's key: does the canary serve, and does bex-api report that request back through logs, metrics, and events | `tenant-view-down` |
| `deploy-canary.sh` | weekly | does a repo become a running HTTPS URL, and does deleting it converge on every read surface | `deploy-canary-down` |
| `verify-tenant-isolation.sh` | weekly | ADR043 reachability matrix on the real substrate | `tenant-isolation-down` |
| `verify-sandbox-isolation-live.sh` | weekly | ADR042 / w3/m35 sandbox boundary matrix | `sandbox-isolation-down` |

The first four are credential-free platform reads. The last four need a **canary fixture**: a first-party `bex-canary` workspace (`billing_excluded` per [ADR040](ADR040-billing-metronome.md) §7) holding one free web service built from `examples/hello-go`, plus one workspace-scoped API key. Free-tier hibernation is wanted rather than tolerated — the probe's own request wakes the service, which exercises the activator path and puts a `service_woken` event in the feed that the probe's last stage reads.

Configuration is split by secrecy, not convenience: the key is a repository **secret** (`BEX_CANARY_API_KEY`, custodied per [ADR019](ADR019-infra-credentials.md)), while the fixture's ids are repository **variables**, so a wrong value is legible in the run log instead of masked.

| setting | kind | value |
| --- | --- | --- |
| `BEX_CANARY_API_KEY` | secret | _owed_ — `<key-id>:<key-secret>` from `POST /v1/api-keys` in the canary workspace |
| `BEX_CANARY_WORKSPACE_ID` | variable | _owed_ — the `tea-…` id of `bex-canary` |
| `BEX_CANARY_SERVICE_ID` | variable | _owed_ — the `srv-…` id of the canary web service |
| `BEX_CANARY_URL` | variable | _owed_ — that service's assigned `https://….onbex.co` host |
| `BEX_CANARY_STATIC_REPO` | variable | _owed, optional_ — a public no-build static-site repo; unset leaves the deploy canary's static leg skipped |

**Owed operator steps** (they provision real first-party production resources, so they are an authorized human action, not something a scheduled job or an agent may do): create the `bex-canary` workspace and mark it `billing_excluded`; deploy the `examples/hello-go` web service into it on the free plan; mint one workspace API key; record the four ids as repository variables; put the key in `.env` and run `scripts/gh-secrets.sh`; dispatch each workflow once and record the run ids. **No placeholder id is committed** — a fabricated `srv-…` that reads as live is worse than an absent one, so until these exist every affected job soft-skips with a `::notice::` naming what is missing. A soft-skip is deliberately not a failure: a red run on these workflows must always mean production is broken.

Two probes stay narrower than their scripts allow, on purpose. The deploy canary's static leg needs a public no-build static-site repository that does not exist yet, which is what keeps the `w3/m46` t008 and `w3/m81` t004 static legs owed. And the sandbox matrix's model-key check ([`scripts/verify-sandbox-isolation.sh`](../scripts/verify-sandbox-isolation.sh) `BEX_VERIFY_AGENT_DRIVER=1` + `BEX_VERIFY_AGENT_MODEL=1`, both default 0) costs real model tokens per run while adding nothing to the admission-regression class the weekly run exists to catch; it remains a manual invocation. Enabling it later is two flags plus `BEX_LIVE_AGENT_MODEL_API_KEY` on the job — no script change.

#### Origin-side API telemetry (w3/m84 — closes the former §6 gap)

The gap this section used to record — "bex-api has no first-party per-route request histogram, its error/latency view is Traefik's edge perspective" — is closed. bex-api now exports its own request telemetry on the registry `/metrics` already serves (`lego/backend/internal/api/httpmetrics.go`), which is what makes **edge-versus-origin attribution** possible: an origin-side 2xx under an edge-side error puts the fault between Traefik and the pod (LB or idle-timeout race), an origin-side 5xx puts it in bex-api or a dependency it calls.

| series | labels | what it answers |
| --- | --- | --- |
| `bex_api_http_request_duration_seconds` | `surface`, `route`, `method`, `status` | origin p50/p95/p99 per surface, slowest route patterns |
| `bex_api_http_requests_total` | same | origin error ratio per surface and per route (includes streams) |
| `bex_api_http_in_flight_requests` | `surface` | origin saturation, including long-lived streams |
| `bex_api_graphql_operation_duration_seconds` | `operation`, `type`, `outcome` | whether a mutation actually worked — GraphQL answers 200 with an `errors` array, so no HTTP metric can say |
| `bex_api_mcp_tool_duration_seconds` | `tool`, `outcome` | per-tool latency and denial rate for agent traffic (ADR008) |

The **label contract** is the load-bearing part, and it is enforced by test (`TestOriginMetricsRouteLabelIsAPatternNeverAnId`), not by convention:

- `surface` is the closed set `rest` / `graphql` / `mcp` / `auth` / `internal`. `internal` is the cluster-internal `:8091` listener (projection, mints, ops-role); it is excluded from both alert rules because its callers are bex's own components.
- `route` is the **registered mux pattern** the request matched (`GET /v1/services/{serviceId}`), never a raw path, so no `srv-…`/`dpg-…`/`tea-…` id can become a series. Unrouted requests — 404s, scanners — all fold into `route="unmatched"`.
- `method` folds to `other` outside the methods bex registers; GraphQL `operation` folds to `other` unless the schema's own operation table knows it; an unregistered MCP `tool` creates no series at all.
- Streaming responses (SSE tails, MCP SSE, upgrades) are counted but **not** observed into the duration histogram: an hour-long healthy tail would own every bucket and hide the p95 the SLI is about. Their question — did it start and stay up — is the counter plus the in-flight gauge.

Two alert rules read these, both per surface with a traffic floor so a quiet surface's single failure never pages:

- **`BexApiOriginHighErrorRate`** — 5xx ratio > 5% over 5m above 0.1 req/s, for 10m. Deliberately the same shape as `TraefikHigh5xxRate` so the pair is read together.
- **`BexApiOriginLatencyHigh`** — p95 > **2.5s** over 10m above 0.1 req/s, for 10m. That threshold is the recorded baseline: roughly an order of magnitude above bex-api's normal p95, so load never pages and a real stall does.

Both land with panels on **bex-api + gateways** (origin p50/p95/p99 by surface, origin 5xx ratio + request rate, 5xx rate by route, slowest routes p95, GraphQL mutation fault ratio + p95, MCP tool p95 + outcomes, in-flight by surface) and with promtool fire/no-fire tests in `deploy/gitops/base/rules/alerts_test.yml`. The families reach Prometheus only because the bex-api scrape job's keep-list admits `bex_api_.*` — the m86 death-layer lesson: a rule that evaluates a never-scraped series is worse than no rule.

Remaining, still recorded: per §1, whole-cluster outage detection stays with the external uptime check — this stack must never be its own only witness.

## Consequences

- New env vars enter the cascading inventories: `BEX_OPS_WORKSPACE` + `BEX_OPS_ROLE_TOKEN` (backend, [lego/backend/CLAUDE.md](../lego/backend/CLAUDE.md)); `OAUTH_OPS_CLIENTS` + the role-verb URL/token (dashboard SSR, [dashboard/CLAUDE.md](../dashboard/CLAUDE.md)).
- The consent acceptor acquires its first identity-conditional client class; every other client's behavior is byte-identical.
- The dashboard SSR runtime gains a server-to-server call into bex-api — new coupling, kept to one internal verb.
- Publishing the manifests exposes `obs.bex.co`'s existence and our dashboards' shape. Consistent with the repo's posture: alert rules and platform hostnames are already public, hostnames appear in CT logs regardless, and secrets never enter git.
