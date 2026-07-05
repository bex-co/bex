# operator/CLAUDE.md

## Codegen (the footgun)

- After editing `api/v1alpha1/*_types.go`, run `make manifests generate` and keep the output.
- `api/v1alpha1/zz_generated.deepcopy.go` and `config/crd/bases/` are **generated — never hand-edit**.
- `make test` runs codegen automatically; envtest binaries download to `bin/` on first run (version derives from `k8s.io/api` in go.mod).

## Layering

- `internal/controller/` — mechanism, per runtime: `kubernetes` (Deployment/Service/Ingress) and `opensandbox` (sandbox lifecycle). No business logic.
- `internal/build/` — build plane: CNB / Dockerfile → Zot registry.
- `internal/runtime/` — OpenSandbox client.
- `internal/api/` — bex-api (REST + GraphQL + MCP).

## bex-api invariant

One `Core` with three thin adapters (REST + GraphQL + MCP), all Render-consistent; a change to one adapter must fan out to the other two. Full rules cascade from [internal/api/CLAUDE.md](internal/api/CLAUDE.md) when you edit that package.
