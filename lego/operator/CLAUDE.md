# operator/CLAUDE.md

The **operator** module (`github.com/bex-co/bex/lego/operator`) is the **mechanism** layer: the manager reconciles `App`/`Database` CRs into runtime objects. No business logic, no DB. It imports the `types/` module (the CRD contract) and **never** the `backend/` module — the `App` CR is the contract between them.

## Codegen (the footgun)

- The CRD **types live in the sibling `types/` module** (`../types/v1alpha1`), not here. After editing `../types/v1alpha1/*_types.go`, run `make manifests generate` from `lego/operator/` and keep the output.
- controller-gen reads CRD/deepcopy markers from `../types/...` and RBAC markers from `./...`; deepcopy lands in `../types/v1alpha1/zz_generated.deepcopy.go`, the CRD YAML in `config/crd/bases/`. Both are **generated — never hand-edit**.
- `make test` runs codegen automatically; envtest binaries download to `bin/` on first run (version derives from `k8s.io/api` in go.mod). **CI-enforced** on every push/PR touching `lego/operator/**` or `lego/types/**` (`.github/workflows/operator-test.yml`); the test suite must pass before `deploy.yml` proceeds.

## Layering (mechanism only)

- `cmd/manager/` — the manager entrypoint (`go build ./cmd/manager`).
- `internal/controller/` — mechanism, per runtime: `kubernetes` (Deployment/Service/Ingress) and `opensandbox` (sandbox lifecycle). No business logic.
- `internal/build/` — build plane: CNB / Dockerfile → Zot registry.
- `internal/runtime/` — OpenSandbox client.

The Render-facing API, authz, API keys, and metrics — the **business logic** — live in the `backend/` module (bex-api). See [`../backend/CLAUDE.md`](../backend/CLAUDE.md).
