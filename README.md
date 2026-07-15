# bex

**The open-source Render alternative — AI-native.**

Push a Git repo (or a prebuilt image), get a running HTTPS service at `<name>.onbex.co` — on machines you own. bex runs identically as a local Docker mock and on Hetzner; only the infrastructure provider overlay changes. Built so AI agents can deploy and operate apps as first-class users, not an afterthought.

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE) [![deploy](https://github.com/bex-co/bex/actions/workflows/deploy.yml/badge.svg)](https://github.com/bex-co/bex/actions/workflows/deploy.yml) [![docs](https://github.com/bex-co/bex/actions/workflows/docs.yml/badge.svg)](https://github.com/bex-co/bex/actions/workflows/docs.yml)

> [!WARNING]
> **bex is under active development and is not yet ready for production use. APIs, configuration, and behavior may change without notice.**

## Why bex

- **Own your PaaS.** Render's developer experience — deploy-from-git, custom domains + TLS, suspend/resume — on your own hardware, Apache-2.0.
- **Drop-in familiar.** `bex.yml` is `render.yaml`-shaped, and `bex-api` speaks Render's REST and GraphQL, verified against Render's OpenAPI spec ([docs/ADR006-bex-api.md](docs/ADR006-bex-api.md)). How far the compatibility actually goes — every Render capability × REST/GraphQL/MCP/UI, with evidence — is the parity ledger ([docs/ADR018-render-parity.md](docs/ADR018-render-parity.md)).
- **Built for agents.** Every action is an API call or a Kubernetes CR; state is machine-readable (`phase` / `revision` / `url`). No dashboard-only actions. See the mission and roadmap in [docs/ADR008-vision.md](docs/ADR008-vision.md).

## Quickstart: local mock (machines = Docker containers)

Prereqs: Docker (OrbStack works), Go 1.25+, `kubectl`, `kind`, `clusterctl`.

```bash
# 1. stand up the mock substrate: kind infra cluster + Cluster API + CAPD
#    + an app cluster whose nodes are Docker containers
bash scripts/mock-cluster.sh            # writes infra/local/bex.kubeconfig
export KUBECONFIG=$PWD/infra/local/bex.kubeconfig

# 2. build the operator image, load it into every node, deploy it as a pod
( cd lego/operator && make docker-build IMG=bex-operator:dev )
docker save bex-operator:dev -o /tmp/bex-op.tar
for n in $(kubectl get nodes -o name | sed 's|node/||'); do
  docker cp /tmp/bex-op.tar "$n":/op.tar && docker exec "$n" ctr -n k8s.io images import /op.tar
done
( cd lego/operator && make deploy IMG=bex-operator:dev )   # ns bex-system, BEX_RUNTIME=kubernetes
# local CAPD only: pin the operator to the control-plane node (see docs/ADR004-deployment.md)
kubectl -n bex-system patch deploy bex-controller-manager --type merge -p \
 '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
  "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]}}}}'
kubectl -n bex-system rollout status deploy/bex-controller-manager

# 3. deploy an App — the operator reconciles it onto the worker machines
kubectl apply -f examples/whoami-app.yaml
kubectl get pods -l app.bex.co/app=whoami -o wide

# 4. ★ add a machine, then scale the App onto it
bash scripts/mock-cluster.sh scale 2
kubectl patch apps.app.bex.co whoami --type merge -p '{"spec":{"replicas":6}}'

# fast dev loop (optional): run the operator from source instead of as a pod —
# ( cd lego/operator && make install && BEX_RUNTIME=kubernetes make run )
```

**Deploy to Hetzner:** same bex, different provider — swap `infra/clusterapi/overlays/local-capd` → `…/hetzner-caph`. See [infra/README.md](infra/README.md).

## The `App` resource

```yaml
apiVersion: app.bex.co/v1alpha1
kind: App
metadata: { name: whoami }
spec:
  image: traefik/whoami # prebuilt image; OR build from git with `repo:` + `branch:`
  port: 80
  replicas: 2 # pods bin-pack across machines
```

`kubectl get apps.app.bex.co` shows phase / revision / url. Prefer Render-style config? `scripts/app-apply.sh <bex.yml>` applies a `render.yaml`-shaped `bex.yml` as an App CR (`DRY_RUN=1` to preview).

## bex vs Render

| Capability | bex |
| --- | --- |
| Deploy from git (CNB / Dockerfile) | ✅ |
| Custom domains + TLS | ✅ |
| Suspend / resume / restart | ✅ |
| REST API (Render-compatible) | ✅ lifecycle verbs, logs, metrics, env vars, API keys — create-service / deploys planned |
| GraphQL (Render dashboard-compatible) | ✅ |
| MCP server | ✅ `/mcp` + stdio, OAuth 2.1 ([docs/ADR006-bex-api.md](docs/ADR006-bex-api.md), connect recipe: [docs/ADR025-connect-an-agent.md](docs/ADR025-connect-an-agent.md)) |
| Managed Postgres | ✅ Render `/v1/postgres`-compatible ([docs/ADR009-postgresql-management.md](docs/ADR009-postgresql-management.md)) |
| Auth (API keys, sessions, roles) | ✅ Ory Hydra/Kratos + OpenFGA ([docs/ADR012-auth.md](docs/ADR012-auth.md)) |
| Elastic machines | ✅ manual scale — autoscaler planned |
| Postgres control plane (tenants) | ✅ built, opt-in — not yet the prod default ([docs/ADR003-control-plane.md](docs/ADR003-control-plane.md)) |

## AI-native

Today: a Render-compatible REST + GraphQL + **MCP** API ([docs/ADR006-bex-api.md](docs/ADR006-bex-api.md)) an agent can drive end-to-end with its own revocable API key, and structured state on the App CR (`status.phase`, `status.revision`, `status.url`) that agents read without scraping. Next: deploy-from-chat (repo → URL in one call) and E2B-compatible sandboxes. The thesis and roadmap live in [docs/ADR008-vision.md](docs/ADR008-vision.md).

## Architecture

Two clusters: the **app cluster** runs the bex operator and your Apps; the **infra cluster** runs Cluster API, which provisions the app cluster's machines (Docker containers locally via CAPD, Hetzner servers via CAPH — same manifests, different overlay). The **operator** is the mechanism (reconciles `App` CRs into Deployment/Service/Ingress); the **Postgres control plane** is the intent layer (tenants/apps/domains) that writes those CRs — built into bex-api as an opt-in, [docs/ADR003-control-plane.md](docs/ADR003-control-plane.md). Two runtimes via `BEX_RUNTIME`: `kubernetes` (elastic, multi-machine) and `opensandbox` (single host, real pause/resume). Full map with diagrams: [docs/ADR002-architecture.md](docs/ADR002-architecture.md).

## Layout

All Go lives in `lego/` — a workspace of three modules; one image, two binaries. Details: [lego/README.md](lego/README.md).

```
lego/            the product: ALL Go (Latin legō, "I assemble"). go.work · Dockerfile — one image, two binaries
  types/            App/Database CRD contract (app.bex.co/v1alpha1); leaf, imports nothing
  operator/         mechanism: cmd/manager · internal/{controller,build,runtime} · config/ · codegen (make)
  backend/          bex-api: cmd/api · internal/{apps,logs,metrics,apikeys,postgres,secrets,store,…} —
                    Render REST/GraphQL/MCP + authz + control-plane store
                    dependency: operator → types ← backend  (operator never imports backend)
dashboard/       the human-facing dashboard (TanStack Start + Apollo + shadcn), client of bex-api's GraphQL
infra/           bex-infra: terraform/ · clusterapi/{base,overlays/{local-capd,hetzner-caph}} · local/
deploy/          gitops/{bootstrap,base,overlays/{local,staging,prod},charts,authz} · opensandbox/ configs
examples/        whoami-app.yaml (prebuilt) · hello-go/ (build-from-git sample)
docs/            vision · architecture · control-plane · bex-api · connect-an-agent · render-parity ·
                 observability · deployment · custom-domain · restart-suspend-and-resume · auth · secrets ·
                 postgresql-management · sandboxes · etcd-backup-restore · go-and-gitops
scripts/         mock-cluster.sh · app-apply.sh · domain-add.sh · deploy-sample.sh ·
                 auth-*.sh + authz-model.sh (Ory/OpenFGA bootstrap) · bao-*.sh + secrets-verify.sh (OpenBao) ·
                 gh-secrets.sh · gitops-validate.sh · up.sh + start-opensandbox*.sh (legacy single-host path)
```

## Status & roadmap

Working and verified: App CRD + reconcile, kubernetes runtime (App → Deployment → pods on machines), local CAPD mock with add/remove machine, opensandbox runtime with real pause/resume, custom domains + TLS, the full REST/GraphQL/MCP surface (lifecycle, logs, metrics, env vars, API keys, managed Postgres), auth (Hydra/Kratos/OpenFGA), the opt-in Postgres control plane, the dashboard, and a live Hetzner deployment. Tracked next — control plane on-by-default + tenant onboarding, wake activator + HMAC webhook, autoscaler wiring, in-cluster builds: [docs/ADR008-vision.md](docs/ADR008-vision.md#roadmap).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). This repo is agent-friendly ([CLAUDE.md](CLAUDE.md)). Licensed [Apache-2.0](LICENSE).
