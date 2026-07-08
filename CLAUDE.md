# CLAUDE.md

bex is the open-source Render alternative — AI-native ([docs/vision.md](docs/vision.md)). A Go Kubernetes operator reconciles `App` CRs (`app.bex.co/v1alpha1`, namespace `bex-system`) into running services. All Go lives in **`lego/`** (Latin _legō_, "I assemble" — bex assembles bought bricks; the modules are the bricks): one image, two binaries from a Go workspace (`lego/go.work`) of three modules. **lego/types/** — the `App`/`Database` CRD contract (leaf; imports nothing); **lego/operator/** — the **manager** (`cmd/manager`), a mechanism-only, DB-free reconciler; **lego/backend/** — **bex-api** (`cmd/api`, the Render-compatible REST/GraphQL/MCP surface on :8090 + OpenFGA authz + API keys + metrics). Dependency arrows point one way: `operator → types ← backend`; the operator never imports the backend.

## Repo map

- `lego/` — **the product: all Go**, self-contained (workspace `go.work`, `Dockerfile` + `.dockerignore`; build context is `lego/`). Overview: [`lego/README.md`](lego/README.md).
  - `lego/types/` — the `App`/`Database` CRD types (`app.bex.co/v1alpha1`); leaf, imports nothing.
  - `lego/operator/` — the operator (kubebuilder): manager reconciles CRs → Deployment/Service/Ingress (mechanism, no DB). Owns codegen, `config/`, `hack/`. See `lego/operator/CLAUDE.md`.
  - `lego/backend/` — **bex-api**, the business-logic service (Render REST/GraphQL/MCP + authz + API keys + metrics). Imports `types/`, never `operator/`. See `lego/backend/CLAUDE.md`.
- `dashboard/` — the human-facing dashboard (TanStack Start + Apollo + shadcn), client of `bex-api`'s GraphQL. See `dashboard/CLAUDE.md` before editing.
- `infra/` — day-0 provisioning: Terraform + Cluster API; overlays `local-capd` (Docker) ⇄ `hetzner-caph`. The operator never references `infra/`.
- `deploy/gitops/` — day-1+: what Argo CD reconciles into the cluster (zot registry, opensandbox controller, CAPI, autoscaler, bex itself). GitOps is for platform infra, not user deploys.
- `examples/` — `whoami-app.yaml` (prebuilt image), `hello-go/` (build-from-git sample).
- `scripts/` — local-cluster and app helpers (below). `up.sh` / `start-opensandbox*.sh` are the legacy single-host path.
- `docs/` — the real documentation; one file per topic, indexed below.
- `.pm/` — internal PM notes and milestone logs. Not documentation; may be stale or aspirational.

## Commands (run from `lego/operator/`)

All Go is a workspace under `lego/` (`lego/go.work` over `types/` `operator/` `backend/`). The `make` targets live in **`lego/operator/`** (codegen, manager build, image build with context `lego/`, deploy). Build/test bex-api from `lego/backend/` (`cd lego/backend && go build ./... && go test ./...`); the CRD types are in `lego/types/`. Run `make` below **from `lego/operator/`**:

- `make test` — unit + envtest; auto-runs manifests/generate/fmt/vet first. First run downloads envtest binaries to `bin/`. (codegen reads CRD/deepcopy markers from `../types`, RBAC from `./...`.)
- `make lint` / `make lint-fix` — golangci-lint.
- `make build` — build the manager binary.
- Dev inner loop against a cluster: `make install && BEX_RUNTIME=kubernetes make run` (runs the operator from the host).
- `make docker-build IMG=…` / `make deploy IMG=…` — image build / kustomize deploy to the current kubeconfig.
- ⚠️ `make test-e2e` creates and deletes a kind cluster (`control-plane-test-e2e`) — slow, CI territory; don't run casually.

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
| operator | `BEX_REGISTRY`, `BEX_CNB_BUILDER` | image registry (zot) and CNB builder for build-from-git |
| operator | `BEX_OPENSANDBOX_URL` | OpenSandbox endpoint (opensandbox runtime) |
| operator | `BEX_BASE_DOMAIN`, `BEX_CLUSTER_ISSUER` | `*.onbex.co` app URLs, cert-manager issuer |
| operator | `BEX_DB_DOMAIN` | public managed-Postgres hostnames `<name>.<domain>` via Traefik TCP/SNI (docs/postgresql-management.md); unset ⇒ internal-only |
| operator | `BEX_ACTIVATOR_SERVICE` | k8s Service name of the wake activator (e.g. `bex-activator`); unset ⇒ auto-sleep disabled |
| operator | `BEX_ACTIVATOR_PORT` | activator service port (default `8888`) |
| activator | `BEX_ACTIVATOR_ADDR` | listen address (default `:8888`) |
| bex-api | `BEX_API_ADDR` (:8090), `BEX_API_NAMESPACE`, `BEX_API_CORS_ORIGIN` | listen addr, watched ns, CORS origin allowlist (comma-separated) |
| bex-api | `BEX_HYDRA_ADMIN_URL` (required), `BEX_KRATOS_URL` | OAuth2 API keys via Hydra introspection; Kratos sessions (docs/auth.md) |
| bex-api | `BEX_OPENFGA_URL`, `BEX_OPENFGA_TOKEN` | authorization via OpenFGA; unset ⇒ allow-all (docs/auth.md) |
| bex-api | `BEX_PROM_URL` | Prometheus base URL for request metrics (Traefik) and resource-metrics history (cAdvisor); unset ⇒ request metrics 503, resource metrics fall back to the metrics-server snapshot |
| bex-api | `BEX_OPENBAO_URL` | OpenBao base URL for the tenant env-vars store (docs/secrets.md); unset ⇒ env-vars verbs 503 |
| bex-api | `BEX_OPENBAO_JWT_PATH` | override the ServiceAccount token path for OpenBao k8s-auth login (off-cluster/dev; default is the pod's projected token) |
| bex-api | `BEX_CP_DB_URI` | Postgres URI for the control-plane store (docs/control-plane.md); set ⇒ runs migrations + the apps-rows→App-CR projector + the internal tenant API; unset ⇒ bex-api alone |
| bex-api | `BEX_CP_APPS_NAMESPACE`, `BEX_CP_ADDR` (:8091), `BEX_CP_RESYNC`, `BEX_CP_TOKEN` | control-plane knobs: projection target ns, internal API addr, resync interval, internal API bearer |
| bex-api | `BEX_MCP_STDIO` | `1` ⇒ serve only the MCP tools over stdio (same as `api mcp-stdio`) |

## Docs index

- [docs/vision.md](docs/vision.md) — mission, AI-native pillars, roadmap.
- [docs/architecture.md](docs/architecture.md) — the map: two clusters, two layers, panorama diagram.
- [docs/control-plane.md](docs/control-plane.md) — Postgres source of truth (built, opt-in via `BEX_CP_DB_URI`) vs. operator mechanism.
- [docs/bex-api.md](docs/bex-api.md) — REST/GraphQL/MCP design: one core, thin adapters, Render compatibility.
- [docs/observability.md](docs/observability.md) — Logs (query + live-tail) and metrics over REST/GraphQL/MCP.
- [docs/deployment.md](docs/deployment.md) — deploy flow, health gating, revisions.
- [docs/custom-domain.md](docs/custom-domain.md) — `App.spec.hosts[]`, Traefik + cert-manager.
- [docs/restart-suspend-and-resume.md](docs/restart-suspend-and-resume.md) — lifecycle verbs.
- [docs/sandboxes.md](docs/sandboxes.md) — ADR: E2B-compatible, idle-hibernated sandboxes over opensandbox (pillar 5).
- [docs/etcd-backup-restore.md](docs/etcd-backup-restore.md) — nightly etcd snapshot → object storage; restore runbook.
- [docs/auth.md](docs/auth.md) — ADR: Ory Kratos (identity) + Hydra (OAuth2) on CNPG; secrets out-of-band.
- [docs/secrets.md](docs/secrets.md) — ADR: OpenBao for tenant credentials; integrated Raft storage, Shamir unseal via `.env`, Kubernetes auth scoped to `tenants/*`.
- [docs/postgresql-management.md](docs/postgresql-management.md) — managed tenant Postgres: `Database` CR → CNPG Cluster, plans, internal/external URLs.
- [docs/go-and-gitops.md](docs/go-and-gitops.md) — why bex (Go product) ≠ GitOps (platform infra).

## Rules

- **Never `git commit` or `git push` unless the user runs `/ship`.** Leave work uncommitted otherwise.
- Never commit or print `.env` or `*.kubeconfig` contents.
- **Keep `.env.example` and `.env.template` in sync with `.env`'s variable names.** They're the checked-in, value-less mirrors of the local runtime env (`.env.example`) and CI secrets env (`.env.template`) — whenever a var is added, renamed, or removed from one, mirror the change (name + comment, never the value) in the other(s) so `cp .env.example .env` / `cp .env.template .env` never falls out of date.
- New Go files carry the Apache-2.0 header from `lego/operator/hack/boilerplate.go.txt`.
- Markdown is CI-checked: `npx prettier@3.4.2 --write "**/*.md"` before finishing doc changes.
- **`.pm` done items move to `done/` folders.** When a task or milestone is completed, never leave it in place: a done task moves to `wN/mN/done/tNNN.md`; a milestone with no open tasks moves whole to `wN/done/mN/`; a done inbox note moves to `wN/done/NNN.md`. Sync status in all three places (task frontmatter, milestone README `**Status:**` + `— **DONE**` row, workstream README checkbox). Full conventions: `.pm/CLAUDE.md`.
- Playwright MCP writes to `.playwright-mcp/` (`--output-dir` in `.mcp.json`, gitignored). When taking screenshots, pass a **bare** filename (e.g. `render-logs.png`) so it lands there — never a path that resolves to the repo root. If an image ever appears at the project root, move it into `.playwright-mcp/`.
