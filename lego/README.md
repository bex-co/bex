# lego/ — the bex Go product

All of bex's Go lives here, self-contained. The name is Latin **_legō_** ("I gather, I assemble") — bex is the glue that assembles bought bricks (Kubernetes, Cluster API, Zot, Cloud Native Buildpacks, OpenSandbox, Ory, OpenFGA) into a Render-style PaaS; the modules below are the bricks. This directory is the whole product; everything outside it (`infra/`, `deploy/`, `docs/`, `dashboard/`) is provisioning, GitOps, prose, or the separate frontend.

## Three platform modules, one contract — plus the CLI

A Go workspace (`go.work`) of four modules. Three of them are the platform: the dependency arrows point **one way** — both sides depend on the shared CRD contract, and the operator never imports the backend:

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
| **cli/** | `…/lego/cli` | the **`bex` CLI launcher** — imports the pinned upstream `render-oss/cli` (no fork) and maps Bex defaults. Not a platform entrypoint: released as user-installed binaries via the `bex-cli/v*` tag train, excluded from the image build context. | nothing of the three above |

`types/` is a struct definition, not an API surface — it's the shape both layers read and write, so it's the only thing they share. That boundary is compiler-enforced: `operator/go.mod` carries none of the backend's deps (graphql/mcp), so the operator physically cannot compile against backend code.

> The Postgres **source of truth** (the "control plane" — tenants/apps/domains projected into App CRs) is **built into `backend/`** as `internal/store/`, opt-in: the api binary runs it when `BEX_CP_DB_URI` is set ([`../docs/ADR003-control-plane.md`](../docs/ADR003-control-plane.md)). Unset (today's prod default), `backend/` serves bex-api alone.

## One image, multiple entrypoints

The `Dockerfile` (build context is `lego/`) builds the platform entrypoints into one image, including:

- `/manager` — the operator (default entrypoint).
- `/api` — bex-api; the Deployment overrides `command: ["/api"]`. `api mcp-stdio` serves the MCP adapter over stdio for a local agent.
- `/ssh-gateway` — the isolated public-key SSH server; its separate Deployment and ServiceAccount alone receive namespaced `pods/exec` permission ([ADR035](../docs/ADR035-ssh.md)).

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

Build/test the CLI from its own module (its releases are the `bex-cli/v*` tag train, not the image):

```bash
cd lego/cli && go build ./... && go test ./...
```

> **Go-version split:** `cli/` and its pinned upstream `render-oss/cli` require Go 1.26, so `go.work` declares `go 1.26.0` and local workspace builds use the 1.26 toolchain. The three platform modules stay on `go 1.25.7`, and the shipped image still compiles them per-module with `golang:1.25` — the Docker build copies no `go.work`, so it is unaffected by the workspace bump.

> **Codegen footgun:** the CRD types live in `types/`, not `operator/`. `make manifests generate` runs controller-gen against `../types/...`; the deepcopy lands in `types/v1alpha1/zz_generated.deepcopy.go` and the CRD YAML in `operator/config/crd/bases/`. Both are generated — never hand-edit. Details in [`operator/CLAUDE.md`](operator/CLAUDE.md).

## Where to read next

- Per-module rules: [`operator/CLAUDE.md`](operator/CLAUDE.md) · [`backend/CLAUDE.md`](backend/CLAUDE.md) · [`backend/internal/api/CLAUDE.md`](backend/internal/api/CLAUDE.md).
- The Render-compatible API design — one Core, three adapters: [`../docs/ADR006-bex-api.md`](../docs/ADR006-bex-api.md).
- The intent-vs-mechanism boundary (planned control plane): [`../docs/ADR003-control-plane.md`](../docs/ADR003-control-plane.md).
- The whole-system map: [`../docs/ADR002-architecture.md`](../docs/ADR002-architecture.md).
