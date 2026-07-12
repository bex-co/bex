# bex: Go direction + do we need GitOps?

Status: decision note. Reviews what the MVP work established, recommends moving the control plane to **Go**, and answers **"do we need GitOps here?"** → **yes, for the platform substrate; no, for the product's per-deploy user workloads.**

## 1. What we learned (review)

bex is the **deploy-from-git half** of 211.09: `webhook → build → deploy → serve` plus `idle → hibernate → wake`. The MVP proved the flow end-to-end on OrbStack and, crucially, clarified the **ours vs. assembled** split and the language of every assembled part.

| Layer | Component | Ours / Assembled | Language | Status on OrbStack |
| --- | --- | --- | --- | --- |
| Control plane | gateway, deploy orchestrator, idle/hibernate, activator | **ours** | Node (MVP) | ✅ works |
| Build | Cloud Native Buildpacks (`pack` + Paketo) | assembled | **Go** | ✅ builds amd64 (Hetzner-correct) |
| Registry | Zot | assembled | **Go** | ✅ |
| Orchestration | OpenSandbox controller (BatchSandbox/Pool/SandboxSnapshot CRDs) | assembled | **Go** | ✅ in vcluster |
| Orchestration | OpenSandbox server (Lifecycle API) | assembled | **Python** | ✅ docker + k8s runtimes |
| Cluster | Kubernetes, vcluster | assembled | **Go** | ✅ k8s + per-tenant vcluster |
| Node plane | Kata/Firecracker microVM | assembled | Go/Rust | ❌ not on OrbStack (pods only) |
| Edge | reverse proxy + wake activator | **ours** | Node (MVP) | ✅ (TLS/ingress deferred) |

**OrbStack-specific findings (carry into any Hetzner plan):**

- Reference the local registry as `127.0.0.1:5050`, not `localhost` (Docker pull resolves `localhost` → IPv6 `::1`, where the published port isn't bound).
- OrbStack does **not** route cluster pod IPs to the host → the edge bridges to pods via `kubectl port-forward` (a real cluster uses in-cluster ingress/gateway).
- OrbStack k8s runs on **cri-dockerd (Docker)**, not standalone containerd → OpenSandbox's snapshot `image-committer` (needs the containerd CRI socket) fails → **k8s-mode pause/resume is unavailable on OrbStack**. Real pause/resume works on the `opensandbox` (Docker) runtime. A containerd-CRI cluster (kind/k3s/Hetzner) would unblock it.

**The pile of imperative setup** the MVP required (and which is _not yet reproducible_): enable k8s, `vcluster create`, `helm install opensandbox-controller` (+ snapshot values), run Zot, generate OpenSandbox configs, create namespaces/secrets. This is the seed of the GitOps question.

## 2. Direction: move the control plane to Go

**Recommendation: yes — our code should be mostly Go.** The MVP's Node gateway was the right call to prove the flow fast (zero deps), but the _target_ is a Kubernetes control plane, and the entire assembled stack is Go: k8s, client-go, controller-runtime, vcluster, Zot, BuildKit, containerd, CNB `pack`/lifecycle, the OpenSandbox **controller**, Knative (activator), Argo CD/Flux. There is even an **OpenSandbox Go SDK** (`sdks/sandbox/go`).

What "mostly Go" means concretely:

- The bex pieces map cleanly onto **Go k8s controllers + a gateway service**:
  - `App` CRD + controller (deploy-from-git revisions) — kubebuilder/controller-runtime.
  - idle/hibernate controller (reconciles activity → pause), wake-on-request **activator** (Knative-style, Go).
  - gateway HTTP service (E2B/ACP wire + webhook receiver) using `client-go` and the OpenSandbox Go SDK.
- Deps stay as they are (we don't rewrite the Python OpenSandbox server or the charts — we _consume_ them). "Mostly Go" = **our** code is Go; we talk to deps over their APIs/CRDs.
- Keep the Node MVP as the executable spec/reference until the Go control plane reaches parity. _(Done — the Node MVP is gone; `lego/` is all Go and the control plane is built.)_

**Two pieces inside "the control plane."** As bex goes multi-tenant, split it: a thin **operator** (CR reconciler, no DB — exists today) and a **control-plane service** backed by **Postgres** as the product's _source of truth_ (tenants / apps / domains + business logic) that projects rows into `App` CRs. Business logic lives in the control plane; the operator stays mechanical. Postgres is the durable truth; **etcd becomes a rebuildable projection** of it. Full design: [`ADR003-control-plane.md`](ADR003-control-plane.md).

This also reinforces the GitOps answer below: a Go, k8s-native control plane is itself just more Deployments + CRDs in the same declarative substrate.

## 3. Do we need GitOps here? — Yes (scoped)

**Answer: prepare GitOps for the _platform substrate_; do _not_ GitOps the product's per-deploy user workloads.** They are two different planes and conflating them is the common mistake.

### Why GitOps for the platform (yes)

The substrate is now a multi-component, version-pinned, Helm-+-CRD stack that must be reproducible across environments (local OrbStack today, Hetzner multi-node next). That is exactly GitOps's sweet spot:

- Everything I installed by hand (vcluster, opensandbox-controller @ chart 0.2.0 / image v0.2.0 + snapshot flags, Zot, CRDs, namespaces, secrets, eventually ingress/cert-manager, Loki/Prometheus, and the **bex control-plane Deployments/CRDs themselves**) is declarative and belongs in Git, reconciled by **Argo CD or Flux**, with drift detection + self-heal.
- Heading to Hetzner / multiple nodes / the 211.x consolidation means many clusters and tenants — GitOps is how you keep them identical and auditable.
- It directly fixes today's gap: the imperative `helm`/`kubectl`/`curl` steps vanish on restart and aren't reproducible.

### Why NOT GitOps for user deploys (no)

bex's **product** _is_ a deploy-from-git system (webhook → build → deploy). Per-tenant app revisions are driven by bex's own control loop and must not live in the platform GitOps repo — that would be building a second, conflicting deploy system. The gateway/controllers own that plane at runtime.

### The gray area: tenant/vcluster provisioning

- **Bootstrap + platform vclusters**: GitOps (Argo `ApplicationSet`, one app per tenant).
- **Self-serve tenant creation**: dynamic, via the bex control plane (it's part of the product). Pick per how self-serve tenanting needs to be; likely _bootstrap via GitOps, scale tenants dynamically_.

### Scope summary

| Concern | GitOps? | Owner |
| --- | --- | --- |
| Cluster addons (ingress, cert-manager, registry/Zot, monitoring) | ✅ yes | platform repo (Argo/Flux) |
| OpenSandbox controller + CRDs (+ snapshot config) | ✅ yes | platform repo |
| bex control-plane **service** (operator/CRDs + gateway + Postgres StatefulSet) | ✅ yes | platform repo |
| Per-tenant vcluster provisioning | ◑ bootstrap via GitOps; scale dynamically | both |
| **User service revisions (webhook→build→deploy)** | ❌ no | **bex product runtime** |
| **Control-plane business data** (tenants/apps/domains _inside_ Postgres) | ❌ no | **bex product runtime** — Postgres is the source of truth; etcd is a rebuildable projection |
| Cluster creation (nodes) | ❌ no (Terraform/infra) | infra repo |

## 4. Proposed GitOps shape (prepared in `deploy/gitops/`)

> **Superseded by the implemented layout.** `deploy/gitops/` landed as `bootstrap/ · base/ · overlays/{local,staging,prod}/ · charts/ · authz/` — same app-of-apps idea, different directory names. The sketch below is kept as the original proposal.

Argo CD **app-of-apps**, environment overlays, secrets via SOPS/sealed-secrets:

```
deploy/gitops/
  README.md
  bootstrap/app-of-apps.yaml        # root Application -> platform/
  platform/                          # one Argo Application per component (pinned versions)
    zot.yaml
    opensandbox-controller.yaml      # chart 0.2.0, image v0.2.0, snapshot values
    vcluster.yaml                    # per-tenant via ApplicationSet (future)
    bex-gateway.yaml                 # our control plane (once containerized in Go)
  values/                            # the exact config the MVP proved, now declarative
    opensandbox-controller.values.yaml
    zot.values.yaml
  envs/{orbstack,hetzner}/           # overlays (insecure-registry/local vs TLS/prod)
```

This converts the imperative MVP setup into reproducible, pinned Git state and gives the Go control plane a home in the same substrate.

## 5. Next steps

1. Scaffold `deploy/gitops/` — **done** (see that dir).
2. Containerize the (Go) gateway so it can be an Argo `Application` like everything else.
3. Begin the Go control plane: `App` CRD + controller + activator (kubebuilder), reusing the Node MVP as the spec — **done except the activator** (`lego/types` + `lego/operator` + `lego/backend/internal/store`; the wake activator is still roadmap item 2 in [ADR008-vision.md](ADR008-vision.md)).
4. For k8s-mode pause/resume: move off OrbStack k8s to a containerd-CRI cluster (kind/k3s locally, Hetzner for real).
