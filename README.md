# bex

**The open-source Render alternative — AI-native.**

Push a Git repo (or a prebuilt image), get a running HTTPS service at `<name>.onbex.co` — on machines you own. bex runs identically as a local Docker mock and on Hetzner; only the infrastructure provider overlay changes. Built so AI agents can deploy and operate apps as first-class users, not an afterthought.

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE) [![deploy](https://github.com/bex-co/bex/actions/workflows/deploy.yml/badge.svg)](https://github.com/bex-co/bex/actions/workflows/deploy.yml) [![docs](https://github.com/bex-co/bex/actions/workflows/docs.yml/badge.svg)](https://github.com/bex-co/bex/actions/workflows/docs.yml)

## Why bex

- **Own your PaaS.** Render's developer experience — deploy-from-git, custom domains + TLS, suspend/resume — on your own hardware, Apache-2.0.
- **Drop-in familiar.** `bex.yml` is `render.yaml`-shaped, and `bex-api` speaks Render's REST and GraphQL, verified against Render's OpenAPI spec ([docs/bex-api.md](docs/bex-api.md)).
- **Built for agents.** Every action is an API call or a Kubernetes CR; state is machine-readable (`phase` / `revision` / `url`). No dashboard-only actions. See the mission and roadmap in [docs/vision.md](docs/vision.md).

## Quickstart: local mock (machines = Docker containers)

Prereqs: Docker (OrbStack works), Go 1.25+, `kubectl`, `kind`, `clusterctl`.

```bash
# 1. stand up the mock substrate: kind infra cluster + Cluster API + CAPD
#    + an app cluster whose nodes are Docker containers
bash scripts/mock-cluster.sh            # writes infra/local/bex.kubeconfig
export KUBECONFIG=$PWD/infra/local/bex.kubeconfig

# 2. build the operator image, load it into every node, deploy it as a pod
( cd operator && make docker-build IMG=bex-operator:dev )
docker save bex-operator:dev -o /tmp/bex-op.tar
for n in $(kubectl get nodes -o name | sed 's|node/||'); do
  docker cp /tmp/bex-op.tar "$n":/op.tar && docker exec "$n" ctr -n k8s.io images import /op.tar
done
( cd operator && make deploy IMG=bex-operator:dev )   # ns bex-system, BEX_RUNTIME=kubernetes
# local CAPD only: pin the operator to the control-plane node (see docs/deployment.md)
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
# ( cd operator && make install && BEX_RUNTIME=kubernetes make run )
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
| REST API (Render-compatible) | ✅ lifecycle verbs — create-service / deploys / logs planned |
| GraphQL (Render dashboard-compatible) | ✅ |
| Elastic machines | ✅ manual scale — autoscaler planned |
| Postgres control plane (tenants/auth) | 🔜 planned |
| MCP server | 🔜 planned |
| Managed databases | — non-goal |

## AI-native

Today: a bearer-authed, Render-compatible REST + GraphQL API ([docs/bex-api.md](docs/bex-api.md)) an agent can drive end-to-end, and structured state on the App CR (`status.phase`, `status.revision`, `status.url`) that agents read without scraping. Next: an MCP server over the same verbs, deploy-from-chat (repo → URL in one call), and E2B-compatible sandboxes. The thesis and roadmap live in [docs/vision.md](docs/vision.md).

## Architecture

Two clusters: the **app cluster** runs the bex operator and your Apps; the **infra cluster** runs Cluster API, which provisions the app cluster's machines (Docker containers locally via CAPD, Hetzner servers via CAPH — same manifests, different overlay). The **operator** is the mechanism (reconciles `App` CRs into Deployment/Service/Ingress); the planned **Postgres control plane** is the intent layer (tenants/apps/domains) that will write those CRs — [docs/control-plane.md](docs/control-plane.md). Two runtimes via `BEX_RUNTIME`: `kubernetes` (elastic, multi-machine) and `opensandbox` (single host, real pause/resume). Full map with diagrams: [docs/architecture.md](docs/architecture.md).

## Layout

```
operator/   Go operator + bex-api (kubebuilder)
  api/v1alpha1/     App CRD              internal/build/      build plane (CNB/Dockerfile → Zot)
  cmd/              manager + api        internal/runtime/    OpenSandbox client
  config/           CRD/RBAC kustomize   internal/controller/ reconcile: kubernetes + opensandbox
                                         internal/api/        bex-api (REST + GraphQL)
infra/           bex-infra: terraform/ · clusterapi/{base,overlays/{local-capd,hetzner-caph}} · local/
deploy/          gitops/{bootstrap,base,overlays/{local,staging,prod},charts} · opensandbox/ configs
examples/        whoami-app.yaml (prebuilt) · hello-go/ (build-from-git sample)
docs/            vision · architecture · control-plane · bex-api · deployment · custom-domain ·
                 restart-suspend-and-resume · go-and-gitops
scripts/         mock-cluster.sh · app-apply.sh · domain-add.sh · deploy-sample.sh ·
                 up.sh + start-opensandbox*.sh (legacy single-host path)
```

## Status & roadmap

Working and verified: App CRD + reconcile, kubernetes runtime (App → Deployment → pods on machines), local CAPD mock with add/remove machine, opensandbox runtime with real pause/resume, custom domains + TLS, lifecycle verbs over REST/GraphQL, and a live Hetzner deployment. Tracked next — Postgres control plane, wake activator + HMAC webhook, autoscaler wiring, in-cluster builds, MCP server: [docs/vision.md](docs/vision.md#roadmap).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). This repo is agent-friendly ([CLAUDE.md](CLAUDE.md)). Licensed [Apache-2.0](LICENSE).
