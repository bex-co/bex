# CLAUDE.md

bex is the open-source Render alternative — AI-native ([docs/vision.md](docs/vision.md)). A Go Kubernetes operator reconciles `App` CRs (`app.bex.co/v1alpha1`, namespace `bex-system`) into running services. One image, two binaries from `operator/`: the **manager** (`cmd/main.go`) and **bex-api** (`cmd/api/main.go`, Render-compatible REST + GraphQL on :8090).

## Repo map

- `operator/` — the Go product (kubebuilder). See `operator/CLAUDE.md` before editing.
- `infra/` — day-0 provisioning: Terraform + Cluster API; overlays `local-capd` (Docker) ⇄ `hetzner-caph`. The operator never references `infra/`.
- `deploy/gitops/` — day-1+: what Argo CD reconciles into the cluster (zot registry, opensandbox controller, CAPI, autoscaler, bex itself). GitOps is for platform infra, not user deploys.
- `examples/` — `whoami-app.yaml` (prebuilt image), `hello-go/` (build-from-git sample).
- `scripts/` — local-cluster and app helpers (below). `up.sh` / `start-opensandbox*.sh` are the legacy single-host path.
- `docs/` — the real documentation; one file per topic, indexed below.
- `.pm/` — internal PM notes and milestone logs. Not documentation; may be stale or aspirational.

## Commands (run from `operator/`)

- `make test` — unit + envtest; auto-runs manifests/generate/fmt/vet first. First run downloads envtest binaries to `bin/`.
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
| bex-api | `BEX_API_ADDR` (:8090), `BEX_API_NAMESPACE`, `BEX_API_TOKEN`, `BEX_API_CORS_ORIGIN` | listen addr, watched ns, bearer token (required), CORS |

## Docs index

- [docs/vision.md](docs/vision.md) — mission, AI-native pillars, roadmap.
- [docs/architecture.md](docs/architecture.md) — the map: two clusters, two layers, panorama diagram.
- [docs/control-plane.md](docs/control-plane.md) — planned Postgres source of truth vs. operator mechanism.
- [docs/bex-api.md](docs/bex-api.md) — REST/GraphQL design: one Core, thin adapters, Render compatibility.
- [docs/observability.md](docs/observability.md) — Logs API (query + live-tail) over REST/GraphQL/MCP; metrics next.
- [docs/deployment.md](docs/deployment.md) — deploy flow, health gating, revisions.
- [docs/custom-domain.md](docs/custom-domain.md) — `App.spec.hosts[]`, Traefik + cert-manager.
- [docs/restart-suspend-and-resume.md](docs/restart-suspend-and-resume.md) — lifecycle verbs.
- [docs/go-and-gitops.md](docs/go-and-gitops.md) — why bex (Go product) ≠ GitOps (platform infra).

## Rules

- **Never `git commit` or `git push` unless the user runs `/ship`.** Leave work uncommitted otherwise.
- Never commit or print `.env` or `*.kubeconfig` contents.
- New Go files carry the Apache-2.0 header from `operator/hack/boilerplate.go.txt`.
- Markdown is CI-checked: `npx prettier@3.4.2 --write "**/*.md"` before finishing doc changes.
- Playwright MCP writes to `.playwright-mcp/` (`--output-dir` in `.mcp.json`, gitignored). When taking screenshots, pass a **bare** filename (e.g. `render-logs.png`) so it lands there — never a path that resolves to the repo root. If an image ever appears at the project root, move it into `.playwright-mcp/`.
