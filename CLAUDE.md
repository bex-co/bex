# CLAUDE.md

bex is the open-source Render alternative — AI-native ([docs/ADR008-vision.md](docs/ADR008-vision.md)). A Go Kubernetes operator reconciles `App` CRs (`app.bex.co/v1alpha1`, namespace `bex-system`) into running services. All Go lives in **`lego/`** (Latin _legō_, "I assemble" — bex assembles bought bricks; the modules are the bricks): one image with multiple entrypoints from a Go workspace (`lego/go.work`) of three modules. **lego/types/** — the `App`/`Database` CRD contract (leaf; imports nothing); **lego/operator/** — the **manager** (`cmd/manager`), a mechanism-only, DB-free reconciler; **lego/backend/** — **bex-api** (`cmd/api`, the Render-compatible REST/GraphQL/MCP surface on :8090 + OpenFGA authz + API keys + metrics) and the isolated **SSH gateway** (`cmd/ssh-gateway`). Dependency arrows point one way: `operator → types ← backend`; the operator never imports the backend.

## Repo map

- `lego/` — **the product: all Go**, self-contained (workspace `go.work`, `Dockerfile` + `.dockerignore`; build context is `lego/`). Overview: [`lego/README.md`](lego/README.md).
  - `lego/types/` — the `App`/`Database` CRD types (`app.bex.co/v1alpha1`); leaf, imports nothing.
  - `lego/operator/` — the operator (kubebuilder): manager reconciles CRs → Deployment/Service/Ingress (mechanism, no DB). Owns codegen, `config/`, `hack/`. See `lego/operator/CLAUDE.md`.
  - `lego/backend/` — the business-logic services: **bex-api** (Render REST/GraphQL/MCP + authz + API keys + metrics) and the isolated SSH gateway. Imports `types/`, never `operator/`. See `lego/backend/CLAUDE.md`.
- `dashboard/` — the human-facing dashboard (TanStack Start + Apollo + shadcn), client of `bex-api`'s GraphQL. See `dashboard/CLAUDE.md` before editing.
- `infra/` — day-0 provisioning: Terraform + Cluster API; overlays `local-capd` (Docker) ⇄ `hetzner-caph`. The operator never references `infra/`.
- `deploy/gitops/` — day-1+: what Argo CD reconciles into the cluster (zot registry, opensandbox controller, CAPI, autoscaler, bex itself). GitOps is for platform infra, not user deploys.
- `examples/` — `whoami-app.yaml` (prebuilt image), `hello-go/` (build-from-git sample).
- `scripts/` — local-cluster and app helpers (below). `up.sh` / `start-opensandbox*.sh` are the legacy single-host path.
- `docs/` — the real documentation; one file per topic, indexed below.
- `.pm/` — internal PM notes and milestone logs. Not documentation; may be stale or aspirational.

## Commands (run from `lego/operator/`)

All Go is a workspace under `lego/` (`lego/go.work` over `types/` `operator/` `backend/`). The `make` targets live in **`lego/operator/`** (codegen, manager build, image build with context `lego/`, deploy). Build/test bex-api from `lego/backend/` (`cd lego/backend && go build ./... && go test ./...`); the CRD types are in `lego/types/`. Run `make` below **from `lego/operator/`**:

- `make test` — unit + envtest; auto-runs manifests/generate/fmt/vet first. First run downloads envtest binaries to `bin/`. (codegen reads CRD/deepcopy markers from `../types`, RBAC from `./...`.) **CI-enforced** on every push/PR touching `lego/operator/**` or `lego/types/**` (`.github/workflows/operator-test.yml`).
- `cd lego/backend && go test ./...` — backend unit + integration tests; integration tests gated on `BEX_TEST_DB_URI`/`BEX_TEST_OPENFGA_URL` are **not** skipped in CI — real ephemeral Postgres + OpenFGA containers run them (`.github/workflows/backend-test.yml`). **CI-enforced** on every push/PR touching `lego/backend/**` or `lego/types/**`.
- `make lint` / `make lint-fix` — golangci-lint over **both** modules (operator + backend); `make lint-backend` runs the backend module alone (its `.golangci.yml` depguard-enforces the id convention, [docs/ADR020-identifiers.md](docs/ADR020-identifiers.md)).
- `make build` — build the manager binary.
- Dev inner loop against a cluster: `make install && BEX_RUNTIME=kubernetes make run` (runs the operator from the host).
- `make docker-build IMG=…` / `make deploy IMG=…` — image build / kustomize deploy to the current kubeconfig.
- ⚠️ `make test-e2e` creates and deletes a kind cluster (`control-plane-test-e2e`) — slow, CI territory; don't run casually.
- All three test suites (`make test`, `go test ./...`, `yarn test`) **must pass before `deploy.yml` builds or pushes any image**: `build-and-deploy` `needs:` all three test jobs.

## Local cluster workflow

- `bash scripts/mock-cluster.sh` — full local mock (kind infra cluster + Cluster API + CAPD app cluster). Writes kubeconfig to `infra/local/bex.kubeconfig` (gitignored credential — never commit or print it).
- `bash scripts/mock-cluster.sh scale N` — add/remove worker machines.
- `scripts/app-apply.sh <bex.yml>` — apply a `render.yaml`-shaped `bex.yml` as an App CR (`DRY_RUN=1` to preview).
- `scripts/deploy-sample.sh` — deploy the hello-go sample.
- `kubectl get apps.app.bex.co` — App status: phase / revision / url.

## Environment variables

| Component | Variable | Meaning |
| --- | --- | --- |
| operator | `BEX_RUNTIME` | `kubernetes` (Deployments) or `opensandbox` (host sandboxes) |
| operator | `BEX_REGISTRY`, `BEX_KPACK_REGISTRY`, `BEX_CNB_BUILDER` | canonical image registry (Zot, e.g. `zot.bex-registry.svc:5000`), optional kpack push alias (local HTTP default `zot.local:5000`; omit to use the canonical registry), and the CNB builder imported by the platform `ClusterBuilder` |
| operator | `BEX_REGISTRY_PUSH_SECRET`, `BEX_REGISTRY_PULL_SECRET` | registry authn (w7/m8, docs/ADR022-tenant-isolation.md § Registry access control): Zot denies anonymous catalog/pull/push. `PUSH_SECRET` names a docker-config Secret (build namespace, key `config.json`) the build Job mounts to authenticate its push — in the buildkitd/cosign container fs only, never a build-arg or BuildKit secret, so tenant Dockerfile `RUN` steps can't read it. `PULL_SECRET` names a docker-config Secret (apps namespace) attached to tenant Deployments/CronJobs as an `imagePullSecret` so kubelet pulls authenticate when per-App credentials are disabled; both minted out-of-band by `scripts/registry-secrets.sh`; either unset ⇒ unauthenticated (dev default, byte-identical) |
| operator | `BEX_REGISTRY_NS` | namespace of the Zot registry Secrets (`zot-htpasswd`, `zot-config`) managed by the operator for per-App pull credentials (w7/m36, docs/ADR022-tenant-isolation.md § Per-App pull credentials); set ⇒ operator mints per-App `reg-pull-<name>` Secrets + htpasswd entries + Zot ACL entries, superseding the shared `BEX_REGISTRY_PULL_SECRET`; unset ⇒ shared pull-secret path (byte-identical to w7/m8) |
| operator | `BEX_ZOT_HTPASSWD_SECRET`, `BEX_ZOT_CONFIG_SECRET` | names of the Zot htpasswd and config Secrets in `BEX_REGISTRY_NS` (defaults `zot-htpasswd` and `zot-config`); override only when the Secrets are renamed |
| operator | `BEX_ZOT_RETENTION_COUNT` | `mostRecentlyPushedCount` in the Zot per-App repo retention policy (default `5`); increase to retain more historical image tags; decrease to save registry storage |
| operator | `BEX_BUILD_NAMESPACE` | namespace the in-cluster BuildKit Jobs, kpack Images/builds, and the pre-deploy Jobs (`predeploy-<name>-gen-<generation>`, w1/m33, docs/ADR004-deployment.md § Pre-deploy command) run in; unset ⇒ the App's own namespace |
| operator | `BEX_APPS_NAMESPACE` | tenant App/Database/KeyValue namespace whose Secrets the manager caches (default `default`); must match bex-api's `BEX_API_NAMESPACE` / `BEX_CP_APPS_NAMESPACE` and the namespace containing `bex-operator-apps` RoleBinding. Other resource kinds remain watched cluster-wide |
| operator | `BEX_TENANT_SIGNING_KEY_SECRET`, `BEX_TENANT_SIGNING_IMAGE` | opt-in tenant-image signing + admission verification (w6/006, w7/m11): a Secret name (keys `cosign.key`+`cosign.password`+`cosign.pub`, in the build namespace) that makes the build Job cosign-sign each pushed tenant image (build+push becomes an initContainer, cosign runs as the main container); when `cosign.pub` is also present in the Secret the `/validate-v1-pod` admission webhook rejects pods with unsigned or tampered tenant images; unset ⇒ tenant images unsigned and unverified (default, byte-identical). `BEX_TENANT_SIGNING_IMAGE` overrides the cosign image |
| operator | `BEX_OPENSANDBOX_URL` | OpenSandbox endpoint (opensandbox runtime) |
| operator | `BEX_BASE_DOMAIN`, `BEX_CLUSTER_ISSUER` | `*.onbex.co` app URLs, cert-manager issuer |
| operator | `BEX_DB_DOMAIN` | public managed-Postgres hostnames `<name>.<domain>` via Traefik TCP/SNI (docs/ADR009-postgresql-management.md); unset ⇒ internal-only |
| operator | `BEX_PROM_URL` | Prometheus base URL for the Postgres disk-autoscaling loop's already-scraped `kubelet_volume_stats_*` series; unset ⇒ the field round-trips but automatic disk growth is disabled |
| operator | `BEX_DB_BACKUP_DESTINATION`, `BEX_DB_BACKUP_ENDPOINT`, `BEX_DB_BACKUP_S3_SECRET` | CNPG `barmanObjectStore` target for managed-Postgres backups + PITR (docs/ADR009-postgresql-management.md): S3 URL prefix (e.g. `s3://bex-tfstate/postgres`), S3-compatible endpoint, and the Secret name (in the DB's namespace) with `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` — the etcd/OpenBao backup credential pattern; any unset ⇒ backups disabled for every plan (recovery unavailable) |
| operator | `BEX_KV_DOMAIN` | public managed key-value (Valkey) hostnames `<name>.<domain>` via Traefik TCP/SNI (docs/ADR021-keyvalue-management.md); unset ⇒ internal-only |
| operator | `BEX_ACTIVATOR_SERVICE` | k8s Service name of the wake activator (e.g. `bex-activator`); unset ⇒ auto-sleep disabled |
| operator | `BEX_ACTIVATOR_PORT` | activator service port (default `8888`) |
| activator | `BEX_ACTIVATOR_ADDR` | listen address (default `:8888`) |
| pg-sni-proxy | `BEX_PROXY_ADDR` | TCP listen address (default `:5432`); the proxy handles PostgreSQL SSLRequest preamble and routes by SNI to CNPG backends using `BEX_DB_DOMAIN` (docs/ADR009-postgresql-management.md §SNI proxy) |
| operator | `BEX_STATIC_S3_ENDPOINT`, `BEX_STATIC_S3_BUCKET`, `BEX_STATIC_S3_REGION`, `BEX_STATIC_S3_SECRET` | static-site (`static_site`) publish target (docs/ADR029-static-sites.md): S3-compatible endpoint + a bucket **dedicated to static content** (never `bex-tfstate`) + optional region + the Secret name (in the publish Job's namespace) with `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` — the etcd/OpenBao backup credential pattern; any unset ⇒ `static_site` Apps rejected |
| operator | `BEX_STATIC_SERVER_SERVICE`, `BEX_STATIC_SERVER_PORT` | k8s Service name + port (default `8080`) of the shared static-server a static-site host's Ingress backs onto (docs/ADR029-static-sites.md); service unset ⇒ `static_site` serving unavailable |
| static-server | `BEX_STATIC_ADDR`, `BEX_STATIC_NAMESPACE`, `BEX_STATIC_S3_ENDPOINT`/`_BUCKET`/`_REGION`, `BEX_STATIC_CACHE_BYTES`, `BEX_STATIC_RESYNC` | listen addr (default `:8080`), watched App ns (empty ⇒ all), object-store origin (endpoint/bucket unset ⇒ degraded 503 mode), in-memory cache budget (default 256 MiB), host→site refresh interval (default `10s`); AWS creds from the env (the `static-s3` Secret) |
| bex-api | `BEX_API_ADDR` (:8090), `BEX_API_NAMESPACE`, `BEX_API_CORS_ORIGIN`, `BEX_API_PUBLIC_URL` | listen addr, watched ns, CORS origin allowlist (comma-separated), and externally reachable API origin used in copy-ready deploy-hook URLs (e.g. `https://api.bex.co`; unset ⇒ relative hook paths) |
| bex-api | `BEX_REGION` | explicit platform placement name projected onto Service/Postgres/Key Value REST metadata (e.g. `fsn1`); unset ⇒ region omitted, never inferred from S3/provider settings |
| bex-api | `BEX_SSH_HOST` | public running-instance SSH gateway hostname (for example `ssh.bex.co`) advertised as Render-compatible `serviceDetails.sshAddress`; unset ⇒ SSH addresses omitted |
| ssh-gateway | `BEX_SSH_ADDR`, `BEX_SSH_METRICS_ADDR`, `BEX_SSH_HOST_KEY_PATH` | isolated gateway SSH address (default `:2222`), internal health/Prometheus address (default `:9090`), and required mounted stable host private-key path; the key is installed out of band by `scripts/ssh-host-key-secret.sh` |
| ssh-gateway | `BEX_CP_DB_URI`, `BEX_OPENFGA_URL`, `BEX_OPENFGA_TOKEN`, `BEX_API_NAMESPACE` | required key/audit store, required authorization endpoint, optional FGA token, and the one tenant App namespace the gateway may discover/exec in |
| ssh-gateway | `BEX_SSH_HANDSHAKE_TIMEOUT`, `BEX_SSH_SESSION_TIMEOUT`, `BEX_SSH_MAX_SESSIONS`, `BEX_SSH_MAX_SESSIONS_PER_IDENTITY` | connection bounds; defaults `10s`, `4h`, `100`, and `5` |
| bex-api | `BEX_BUILD_NAMESPACE` | the deploys feature's Cancel verb (w2/m10): namespace a repo-backed App's in-flight build Job runs in, so Cancel can compute its identity (`bld-<name>-gen-<generation>`) to delete it. Also where the logs feature reads pre-deploy Job pod logs (`type=predeploy`, w1/m33) from — must match the operator's own `BEX_BUILD_NAMESPACE` above; unset ⇒ the App's own namespace |
| bex-api | `BEX_HYDRA_ADMIN_URL` (required), `BEX_KRATOS_URL` | OAuth2 API keys via Hydra introspection; Kratos sessions (docs/ADR012-auth.md) |
| bex-api | `BEX_KRATOS_ADMIN_URL` | Kratos' admin API for the owners/members read API's email/MFA lookup (docs/render-artifacts/owners-api.md); unset ⇒ those fields omitted |
| bex-api | `BEX_OPENFGA_URL`, `BEX_OPENFGA_TOKEN` | authorization via OpenFGA; unset ⇒ allow-all (docs/ADR012-auth.md) |
| bex-api | `BEX_BASE_DOMAIN` | platform wildcard domain (e.g. `onbex.co`) — names custom-domain DNS targets `<app>.<domain>` in the DNS-instructions surface (docs/ADR005-custom-domain.md); unset ⇒ derived from the App's status URL |
| bex-api | `BEX_PROM_URL` | Prometheus base URL for request metrics (Traefik) and resource-metrics history (cAdvisor); unset ⇒ request metrics 503, resource metrics fall back to the metrics-server snapshot |
| bex-api | `BEX_USAGE_RETENTION_MONTHS` | usage hot window (docs/ADR023-usage-metering.md): calendar months (current included) kept at hourly detail before daily compaction folds older months into `usage_monthly` and purges the hourly rows; default `3`, minimum 1 |
| bex-api | `BEX_AUDIT_RETENTION_DAYS` | audit retention (w4/m10 + w2/m39, docs/ADR006-bex-api.md § Audit log): days `audit_events` and SSH-session metadata survive before the daily sweep purges them; default `90`, minimum 1 |
| bex-api | `BEX_LOKI_URL` | Loki base URL for the durable log store (docs/ADR010-observability.md); set ⇒ `QueryLogs`/`Logs` read Loki (history survives pod restarts) **and** the request-log split (`type=request`) + structured filters (`level`/`statusCode`/`method`/`path`/`host`) + label discovery (`list_log_label_values`) work; unset ⇒ live pod-log fallback (app logs, `text`/`time`/`instance` only — the store-only filters return 503, never silently ignored). The SSE live tail always reads pod logs |
| bex-api | `BEX_OPENBAO_URL` | OpenBao base URL for the tenant env-vars store (docs/ADR013-secrets.md); unset ⇒ env-vars verbs 503 |
| bex-api | `BEX_OPENBAO_JWT_PATH` | override the ServiceAccount token path for OpenBao k8s-auth login (off-cluster/dev; default is the pod's projected token) |
| bex-api | `BEX_CP_DB_URI` | Postgres URI for the control-plane store (docs/ADR003-control-plane.md); set ⇒ runs migrations + the apps-rows→App-CR projector + the internal tenant API; unset ⇒ bex-api alone |
| bex-api | `BEX_CP_APPS_NAMESPACE`, `BEX_CP_ADDR` (:8091), `BEX_CP_RESYNC`, `BEX_CP_TOKEN` | control-plane knobs: projection target ns, internal API addr, resync interval, internal API bearer |
| bex-api | `BEX_WEBHOOK_SECRET` | shared HMAC-SHA256 key for the git push webhook (`POST /v1/webhooks/git`); unset ⇒ webhook 503 (docs/ADR006-bex-api.md) |
| bex-api | `BEX_WEBHOOK_BACKOFF` | outbound-webhook retry schedule override (w3/m11, docs/ADR006-bex-api.md § Outbound event webhooks): comma-separated Go durations (`"2s,3s,4s"`) replacing the documented 8-retry/~33h default — a dev/verification knob (`scripts/webhooks-verify.sh` walks the auto-disable path in seconds with it); unset ⇒ the production schedule |
| bex-api | `BEX_GITHUB_APP_ID`, `BEX_GITHUB_APP_PRIVATE_KEY`, `BEX_GITHUB_APP_SLUG` | self-hosted GitHub App (docs/ADR026-github-integration.md): app id (JWT `iss`), RSA private key PEM (out-of-band secret; signs the app JWT and HMAC-signs the short-lived browser callback state), and slug (builds the `github.com/apps/<slug>/installations/new` install URL). Any unset ⇒ every git-connect verb 503 |
| bex-api | `BEX_GITHUB_WEBHOOK_SECRET` | the GitHub App's webhook HMAC-SHA256 key (docs/ADR026-github-integration.md): a **second** accepted key on `POST /v1/webhooks/git` so app-signed pushes redeploy hands-free; verified alongside `BEX_WEBHOOK_SECRET` (valid under either ⇒ accept; 503 only when neither set) |
| bex-api | `BEX_SMTP_ADDR`, `BEX_SMTP_FROM` | SMTP relay (`host:port`) + envelope `From` for workspace-invite email (w4/m12, docs/ADR024-members.md); same relay as the Kratos courier (SendGrid prod, Mailpit local). Either unset ⇒ invites recorded but not emailed |
| bex-api | `BEX_SMTP_USERNAME`, `BEX_SMTP_PASSWORD` | optional SMTP PLAIN-auth credentials (secret, out-of-band); unset ⇒ unauthenticated relay (Mailpit) |
| bex-api | `BEX_DASHBOARD_URL` | trusted dashboard origin for GitHub install-callback redirects, workspace-invite links, and Service/Postgres/Key Value `dashboardUrl` metadata (e.g. `https://dashboard.bex.co`); unset ⇒ callbacks return JSON, invite email has no deep link, and resource dashboard URLs are omitted |
| bex-api | `BEX_MCP_STDIO` | `1` ⇒ serve only the MCP tools over stdio (same as `api mcp-stdio`) |
| bex-api | `BEX_OAUTH_ISSUER`, `BEX_OAUTH_RESOURCE` | OAuth 2.1 discovery for MCP/agent clients (docs/ADR012-auth.md §7): Hydra public issuer + this API's canonical resource URI — drives RFC 9728 metadata, 401 `resource_metadata` hints, and the token-audience check; both unset ⇒ prior behavior |
| bex-api | `BEX_RATE_LIMIT` | per-caller token-bucket fill rate in requests/min (default 500 — Render's documented budget); `0` disables rate limiting (docs/ADR006-bex-api.md §Rate limits) |
| bex-api | `BEX_RATE_BURST` | token-bucket burst capacity (default = `BEX_RATE_LIMIT`); `0` defaults to `BEX_RATE_LIMIT` |
| bex-api | `BEX_MAX_BODY_BYTES` | max non-GET request body size in bytes (default 2097152 = 2 MiB); `0` disables |
| bex-api | `BEX_MAX_QUERY_HOURS` | max `startTime`..`endTime` window for log/metrics queries in hours (default 720 = 30 days); enforced on the log reads across REST, GraphQL, and MCP (w9/004); `0` disables |
| bex-api | `BEX_MAX_SSE_CONNS` | max concurrent `GET /v1/logs/subscribe` SSE connections (default 100); `0` disables |
| bex-api | `BEX_MAX_SERVICES` | per-workspace service creation cap (docs/ADR006-bex-api.md §Per-workspace resource caps); `0` = unlimited (default, byte-identical). Render Hobby anchor: 25 |
| bex-api | `BEX_MAX_POSTGRES` | per-workspace Postgres creation cap; `0` = unlimited. Render Hobby anchor: 1 |
| bex-api | `BEX_MAX_KEYVALUES` | per-workspace key-value creation cap; `0` = unlimited. Render Hobby anchor: 1 |
| operator | `BEX_APP_RECONCILE_WORKERS` | concurrent App reconcile loops (default `1`); source builds currently wait synchronously inside a reconcile, so production uses `2` for two independently dispatched builds (ADR034) |
| operator | `BEX_MAX_CONCURRENT_BUILDS` | max concurrent active build Jobs per workspace; `0` = unlimited. Newest-wins per App is always active (independent of this cap) |
| dashboard (SSR) | `HYDRA_ADMIN_URL`, `HYDRA_PUBLIC_URL`, `OAUTH_TRUSTED_CLIENTS` | OAuth2 consent + official Render CLI device verification at `/auth/consent` and `/auth/device` (docs/ADR012-auth.md §7/§8a): Hydra's admin API, its browser-reachable public issuer, and the allowlist of clients that skip the consent screen. Server-only (not `VITE_`); missing URLs make their corresponding routes answer 503. |

## Docs index

- [docs/ADR008-vision.md](docs/ADR008-vision.md) — mission, AI-native pillars, roadmap.
- [docs/ADR002-architecture.md](docs/ADR002-architecture.md) — the map: self-managed cluster + disposable bootstrap, two layers, node pools + substrate decisions, panorama diagram.
- [docs/ADR003-control-plane.md](docs/ADR003-control-plane.md) — Postgres source of truth (built, opt-in via `BEX_CP_DB_URI`) vs. operator mechanism.
- [docs/ADR020-identifiers.md](docs/ADR020-identifiers.md) — ADR: typed opaque resource ids `<prefix>-<xid>` (`tea-`/`srv-`/`cdm-`), hyphen not underscore; minted + guarded in `lego/backend/internal/id`.
- [docs/ADR006-bex-api.md](docs/ADR006-bex-api.md) — REST/GraphQL/MCP design: one core, thin adapters, Render compatibility.
- [docs/ADR025-connect-an-agent.md](docs/ADR025-connect-an-agent.md) — recipe: Claude Code/Cursor → bex `/mcp` over OAuth 2.1 (discovery, DCR, PKCE, API-key alternative, troubleshooting).
- [docs/ADR026-github-integration.md](docs/ADR026-github-integration.md) — ADR: self-hosted GitHub App (manifest flow) for private-repo deploys + zero-config push-to-deploy; installation tokens → clone Secret, app webhook as a second key.
- [docs/ADR018-render-parity.md](docs/ADR018-render-parity.md) — the parity ledger: one row per Render capability × REST/GraphQL/MCP/UI, each cell ✅/◐/✖/— with evidence; gaps mapped to owning milestones.
- [docs/cli-compatibility-checklist.md](docs/cli-compatibility-checklist.md) — bex's fifth surface: Render's own official CLI (`render-oss/cli`) run unmodified against bex-api, one row per command, ✅/◐/✖/— with captured evidence; the systemic wire-format bugs it found are the parity ledger's CLI-surface counterpart.
- [docs/ADR017-deploy-from-chat.md](docs/ADR017-deploy-from-chat.md) — ADR: deploy-from-chat rides `Core.Create` (no bespoke endpoint) + the HMAC push-to-deploy webhook (pillar 4).
- [docs/ADR010-observability.md](docs/ADR010-observability.md) — Logs (query + live-tail) and metrics over REST/GraphQL/MCP.
- [docs/ADR023-usage-metering.md](docs/ADR023-usage-metering.md) — Hourly usage rollup (instance-seconds/egress/build-seconds) + REST/GraphQL/MCP surface (`GET /v1/usage`, `usage` query, `get_usage` tool); bex extension (Render has no usage API).
- [docs/ADR004-deployment.md](docs/ADR004-deployment.md) — deploy flow, health gating, revisions.
- [docs/ADR005-custom-domain.md](docs/ADR005-custom-domain.md) — `App.spec.hosts[]`, Traefik + cert-manager.
- [docs/ADR007-restart-suspend-and-resume.md](docs/ADR007-restart-suspend-and-resume.md) — lifecycle verbs.
- [docs/ADR014-sandboxes.md](docs/ADR014-sandboxes.md) — ADR: E2B-compatible, idle-hibernated sandboxes over opensandbox (pillar 5).
- [docs/ADR011-etcd-backup-restore.md](docs/ADR011-etcd-backup-restore.md) — nightly etcd snapshot → object storage; restore runbook.
- [docs/ADR015-openbao-backup-restore.md](docs/ADR015-openbao-backup-restore.md) — nightly OpenBao Raft snapshot → object storage; restore runbook.
- [docs/ADR012-auth.md](docs/ADR012-auth.md) — ADR: Ory Kratos (identity) + Hydra (OAuth2) on CNPG; secrets out-of-band.
- [docs/ADR027-sso.md](docs/ADR027-sso.md) — ADR: SSO — social login (consumer OIDC, e.g. Sign in with GitHub) shipped via Kratos `oidc`; enterprise SSO/SAML/SCIM a deferred non-goal.
- [docs/ADR024-members.md](docs/ADR024-members.md) — workspace members & roles: invite by email, Render's five roles, `tenant_members` + OpenFGA tuples, invite-accept-on-login.
- [docs/ADR019-infra-credentials.md](docs/ADR019-infra-credentials.md) — ADR: bootstrap credential inventory + trust chain (`bex` SSH key → mgmt cluster CAPI PKI → app admin cert) and `.env` custody.
- [docs/ADR013-secrets.md](docs/ADR013-secrets.md) — ADR: OpenBao for tenant credentials; integrated Raft storage, Shamir unseal via `.env`, Kubernetes auth scoped to `tenants/*`.
- [docs/ADR016-sealed-secrets.md](docs/ADR016-sealed-secrets.md) — infra creds encrypted at rest in git (SealedSecrets), controller + `kubeseal` workflow.
- [docs/ADR009-postgresql-management.md](docs/ADR009-postgresql-management.md) — managed tenant Postgres: `Database` CR → CNPG Cluster, plans, internal/external URLs.
- [docs/ADR021-keyvalue-management.md](docs/ADR021-keyvalue-management.md) — managed tenant key-value (Valkey): `KeyValue` CR → Valkey StatefulSet, plans, internal/external URLs.
- [docs/ADR029-static-sites.md](docs/ADR029-static-sites.md) — the `static_site` type: build → object-store origin, the shared static-server, redirects/rewrites + custom headers, REST/GraphQL/MCP surface.
- [docs/ADR001-go-and-gitops.md](docs/ADR001-go-and-gitops.md) — why bex (Go product) ≠ GitOps (platform infra).
- [docs/ADR022-tenant-isolation.md](docs/ADR022-tenant-isolation.md) — ADR: east-west network enforcement — threat model, label-scoped NetworkPolicy mechanism, dialect choice, reachability matrix.
- [docs/ADR028-security-review.md](docs/ADR028-security-review.md) — evidence-backed audit (RBAC, supply chain, injection surface, network isolation, secrets hygiene, OAuth) with severities, remediation status, and a follow-up register.
- [docs/ADR031-platform-data-backup.md](docs/ADR031-platform-data-backup.md) — consolidated platform data-backup policy: etcd, OpenBao, and bex-db backup mechanisms, one-time setup, restore runbooks, drill records, and re-drill cadence.
- [docs/ADR032-environments.md](docs/ADR032-environments.md) — Environments: named subsets of a Project's services (staging/production), layered on `internal/projects` (w1/m31); assignment auto-joins the project, REST/GraphQL/MCP surface, MVP scope (no protected-environment ACLs, no dashboard UX).
- [docs/ADR034-scalable-build-pipeline.md](docs/ADR034-scalable-build-pipeline.md) — scalable in-cluster builds: ephemeral BuildKit workers, reconcile/admission concurrency, Pod-versus-machine capacity, and the non-blocking evolution path.
- [docs/ADR035-ssh.md](docs/ADR035-ssh.md) — running-instance SSH: identity keys, Render-compatible instance targeting, isolated gateway, Kubernetes exec bridge, and production activation gates.
- [docs/ADR036-ca-rotation-runbook.md](docs/ADR036-ca-rotation-runbook.md) — Kubernetes CA rotation and admin-cert renewal: annual cert renewal (non-disruptive) and full CA rotation (emergency, disruptive); AdminCertExpiringSoon alert response.
- [docs/ADR037-openbao-rekey-runbook.md](docs/ADR037-openbao-rekey-runbook.md) — OpenBao root-token rotation (`bao operator generate-root`) and Shamir re-key (`bao operator rekey`): when to use each, exact commands, `.env` + GitHub Actions update.

## Rules

- **Never `git commit` or `git push` unless the user runs `/ship` (Claude) or invokes `$ship` (Codex).** Leave work uncommitted otherwise.
- Never commit or print `.env` or `*.kubeconfig` contents.
- **Keep `.env.example` and `.env.template` in sync with `.env`'s variable names.** They're the checked-in, value-less mirrors of the local runtime env (`.env.example`) and CI secrets env (`.env.template`) — whenever a var is added, renamed, or removed from one, mirror the change (name + comment, never the value) in the other(s) so `cp .env.example .env` / `cp .env.template .env` never falls out of date.
- New Go files carry the Apache-2.0 header from `lego/operator/hack/boilerplate.go.txt`.
- **Mint every resource id through `lego/backend/internal/id` (`id.New(kind)`), never by hand.** Ids are typed opaque `<prefix>-<xid>` strings with a **hyphen** separator (never an underscore — they must stay valid DNS/k8s names); a new id-bearing resource adds its `id.Kind` to that package's registry (which the guard test then enforces). Rationale + known deviations: [docs/ADR020-identifiers.md](docs/ADR020-identifiers.md).
- Markdown is CI-checked: `npx prettier@3.4.2 --write "**/*.md"` before finishing doc changes.
- **`.pm` done items move to `done/` folders — move the entire folder, leave nothing behind.** When a task or milestone is completed, never leave it in place: a done task moves to `wN/mN/done/tNNN.md`; a milestone with no open tasks moves **whole** to `wN/done/mN/` (`mv` the directory, then `rmdir` the empty original — **never leave a tombstone, stub, or redirect README at the old path**; a done milestone's original path must simply not exist); a done inbox note moves to `wN/done/NNN.md`. Sync status in all three places (task frontmatter, milestone README `**Status:**` + `— **DONE**` row, workstream README checkbox). If a `/goal` (or other tooling) references a milestone by its pre-move path, that path is _expected_ to disappear on completion — do not recreate it. Full conventions: `.pm/CLAUDE.md`.
- Playwright MCP writes to `.playwright-mcp/` (`--output-dir` in `.mcp.json`, gitignored). When taking screenshots, pass a **bare** filename (e.g. `render-logs.png`) so it lands there — never a path that resolves to the repo root. If an image ever appears at the project root, move it into `.playwright-mcp/`.
