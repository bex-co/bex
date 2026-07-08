# lego/ — the bex Go product

All of bex's Go lives here, self-contained. The name is Latin **_legō_** ("I gather, I assemble") — bex is the glue that assembles bought bricks (Kubernetes, Cluster API, Zot, Cloud Native Buildpacks, OpenSandbox, Ory, OpenFGA) into a Render-style PaaS; the modules below are the bricks. This directory is the whole product; everything outside it (`infra/`, `deploy/`, `docs/`, `dashboard/`) is provisioning, GitOps, prose, or the separate frontend.

## Three modules, one contract

A Go workspace (`go.work`) of three modules. The dependency arrows point **one way** — both sides depend on the shared CRD contract, and the operator never imports the backend:

```
        types/            the contract; imports nothing
       ▲      ▲
      /        \
 operator/    backend/    both import types/; neither imports the other
```

| module | path | role | imports |
| --- | --- | --- | --- |
| **types/** | `github.com/bex-co/bex/lego/types` | the **contract** — `App`/`Database` CRD Go types (`app.bex.co/v1alpha1`) | nothing (leaf) |
| **operator/** | `…/lego/operator` | **mechanism** — the manager reconciles `App` CRs → Deployment/Service/Ingress (+TLS). No DB, no business logic. | `types/` |
| **backend/** | `…/lego/backend` | **business logic** — **bex-api**: the Render REST/GraphQL/MCP surface (:8090) + OpenFGA authz + API-key auth + metrics + the opt-in control-plane store. | `types/` |

`types/` is a struct definition, not an API surface — it's the shape both layers read and write, so it's the only thing they share. That boundary is compiler-enforced: `operator/go.mod` carries none of the backend's deps (graphql/mcp), so the operator physically cannot compile against backend code.

> The Postgres **source of truth** (the "control plane" — tenants/apps/domains projected into App CRs) is **built into `backend/`** as `internal/store/`, opt-in: the api binary runs it when `BEX_CP_DB_URI` is set ([`../docs/control-plane.md`](../docs/control-plane.md)). Unset (today's prod default), `backend/` serves bex-api alone.

## One image, two binaries

The `Dockerfile` (build context is `lego/`) builds two binaries into one image:

- `/manager` — the operator (default entrypoint).
- `/api` — bex-api; the Deployment overrides `command: ["/api"]`. `api mcp-stdio` serves the MCP adapter over stdio for a local agent.

Deploy manifests live in [`operator/config/`](operator/config); Argo reconciles `lego/operator/config/default` ([`../deploy/gitops/base/bex.yaml`](../deploy/gitops/base/bex.yaml)).

## Build & test

The `make` targets live in **`operator/`** (codegen, manager build, image build, deploy). Run from there:

```bash
cd lego/operator
make test          # unit + envtest; auto-runs manifests/generate/fmt/vet first
make build         # build the manager binary
make docker-build IMG=…   # one image, both binaries (context is lego/)
make manifests generate   # codegen — reads CRD/deepcopy markers from ../types, RBAC from ./...
```

Build/test bex-api from its own module:

```bash
cd lego/backend && go build ./... && go test ./...
```

> **Codegen footgun:** the CRD types live in `types/`, not `operator/`. `make manifests generate` runs controller-gen against `../types/...`; the deepcopy lands in `types/v1alpha1/zz_generated.deepcopy.go` and the CRD YAML in `operator/config/crd/bases/`. Both are generated — never hand-edit. Details in [`operator/CLAUDE.md`](operator/CLAUDE.md).

## Where to read next

- Per-module rules: [`operator/CLAUDE.md`](operator/CLAUDE.md) · [`backend/CLAUDE.md`](backend/CLAUDE.md) · [`backend/internal/api/CLAUDE.md`](backend/internal/api/CLAUDE.md).
- The Render-compatible API design — one Core, three adapters: [`../docs/bex-api.md`](../docs/bex-api.md).
- The intent-vs-mechanism boundary (planned control plane): [`../docs/control-plane.md`](../docs/control-plane.md).
- The whole-system map: [`../docs/architecture.md`](../docs/architecture.md).
