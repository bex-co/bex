# bex platform GitOps

The **platform substrate** as declarative, version-pinned state, reconciled by Argo CD.

> **GitOps the platform; never GitOps the per-deploy user workloads** (those are bex's product runtime — webhook → build → deploy). And cluster/node _creation_ lives in [`infra/`](../../infra/), not here (engine vs. desired-infra — see [docs/ADR002-architecture.md](../../docs/ADR002-architecture.md)).

## Layout (base + overlays)

```
deploy/gitops/
├── bootstrap/
│   ├── local.yaml          per-env app-of-apps entrypoint → overlays/local
│   ├── staging.yaml        (seam)
│   └── prod.yaml           (seam)
├── base/                   shared platform components (one Argo Application each)
│   ├── zot.yaml            OCI registry
│   ├── opensandbox-controller.yaml   CRDs + controller (chart 0.2.0, image v0.2.0)
│   ├── bex.yaml            the bex control plane
│   ├── cluster-api.yaml    CAPI/provider controllers (engine; desired pools in infra/)
│   │                       (cluster-autoscaler is deliberately NOT here — it installs on
│   │                       the MGMT cluster via scripts/install-autoscaler.sh; rationale
│   │                       in infra/clusterapi/autoscaler-values.yaml, w1/m3)
│   ├── values/             default values
│   └── kustomization.yaml
├── overlays/               per-env differences only (reference ../base)
│   ├── local/              insecure registry, single replicas, CAPD
│   ├── staging/  (seam)
│   └── prod/     (seam)
└── charts/                 vendored Helm charts (opensandbox-controller, …)
```

`base/` is the Kustomize **baseline** (the components common to every env); `overlays/<env>/` reference it and patch only what differs; `bootstrap/<env>.yaml` is the Argo entrypoint that points at one overlay.

## Status

- ✅ opensandbox-controller chart **vendored** (`charts/opensandbox-controller`, 0.2.0); renders with pinned values (image `v0.2.0` + snapshot flags).
- ✅ Argo CD installed in the cluster; Application manifests validate (`--dry-run=server`).
- ⬜ Push to a git remote, set `repoURL` in `bootstrap/local.yaml` (+ components), then `kubectl apply -f bootstrap/local.yaml`.
- ⬜ Containerize the Go control plane → fill in `base/bex.yaml`.

Secrets via **SOPS** / **sealed-secrets** (never plaintext).
