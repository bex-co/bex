# operator/CLAUDE.md

## Codegen (the footgun)

- After editing `api/v1alpha1/*_types.go`, run `make manifests generate` and keep the output.
- `api/v1alpha1/zz_generated.deepcopy.go` and `config/crd/bases/` are **generated — never hand-edit**.
- `make test` runs codegen automatically; envtest binaries download to `bin/` on first run (version derives from `k8s.io/api` in go.mod).

## Layering

- `internal/controller/` — mechanism, per runtime: `kubernetes` (Deployment/Service/Ingress) and `opensandbox` (sandbox lifecycle). No business logic.
- `internal/build/` — build plane: CNB / Dockerfile → Zot registry.
- `internal/runtime/` — OpenSandbox client.
- `internal/api/` — bex-api (REST + GraphQL).

## bex-api invariant

New verbs go in `internal/api/core.go` **only**. `rest.go` and `graphql.go` are thin presentation adapters over identical Core methods and must not contain logic — this is the design guarantee that the two APIs can't drift ([docs/bex-api.md](../docs/bex-api.md)). REST shapes are verified against Render's OpenAPI spec (e.g. `suspended` is the string enum `"suspended"`/`"not_suspended"`, not a boolean) — don't "fix" them to look more conventional.
