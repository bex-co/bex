# lego/CLAUDE.md

All Go lives here (Latin _legō_, "I assemble" — bex assembles bought bricks; the modules are the bricks), self-contained: one image with multiple entrypoints from a Go workspace (`go.work`) of four modules — `types/` (the `App`/`Database` CRD contract; leaf, imports nothing), [`operator/`](operator/CLAUDE.md) (the mechanism-only, DB-free manager), [`backend/`](backend/CLAUDE.md) (bex-api + the isolated SSH gateway), and `cli/` (the `bex` CLI launcher). Dependency arrows point one way: `operator → types ← backend`; the operator never imports the backend, and `cli/` imports none of the three. Overview: [README.md](README.md).

## Commands (run from `operator/`)

All Go is a workspace under `lego/` (`lego/go.work` over `types/` `operator/` `backend/` `cli/`). All four modules are on the `go 1.26` line: `go.work` declares `go 1.26.0`, each module's own go.mod is on 1.26, and the shipped image compiles them per-module with `golang:1.26` (the Docker build has no workspace file). The platform moved off 1.25 with the `golang.org/x/crypto` v0.56.0 bump (v0.56.0 requires Go 1.26; it fixes the reachable ssh-gateway DoS the govulncheck gate flagged). The `make` targets live in **`lego/operator/`** (codegen, manager build, image build with context `lego/`, deploy). Build/test bex-api from `lego/backend/` (`cd lego/backend && go build ./... && go test ./...`); the CRD types are in `lego/types/`. Run `make` below **from `lego/operator/`**:

- `make test` — unit + envtest; auto-runs manifests/generate/fmt/vet first. First run downloads envtest binaries to `bin/`. (codegen reads CRD/deepcopy markers from `../types`, RBAC from `./...`.) **CI-enforced** on every push/PR touching `lego/operator/**` or `lego/types/**` (`.github/workflows/operator-test.yml`).
- `cd lego/backend && go test ./...` — backend unit + integration tests; integration tests gated on `BEX_TEST_DB_URI`/`BEX_TEST_OPENFGA_URL` are **not** skipped in CI — real ephemeral Postgres + OpenFGA containers run them (`.github/workflows/backend-test.yml`). **CI-enforced** on every push/PR touching `lego/backend/**` or `lego/types/**`.
- `make lint` — golangci-lint over **all four** modules (operator + backend + types + cli), then pinned whole-program dead-code analysis across their executable, test, and e2e-tagged roots. `make lint-fix` fixes the golangci findings; `make lint-backend` runs backend golangci-lint alone (its `.golangci.yml` depguard-enforces the id convention, [docs/ADR020-identifiers.md](../docs/ADR020-identifiers.md)). **CI-enforced** on every push/PR touching any `lego/` module (`.github/workflows/go-lint.yml`, which runs `make lint`).
- `cd lego/cli && go test ./...` — CLI launcher tests; **CI-enforced** on pushes/PRs touching `lego/cli/**` (`.github/workflows/cli-test.yml`). Releases ride the `bex-cli/v*` tag train (`cli-release.yml`, [docs/bex-cli.md](../docs/bex-cli.md), ADR058) — never the platform image.
- `make build` — build the manager binary.
- Dev inner loop against a cluster: `make install && BEX_RUNTIME=kubernetes make run` (runs the operator from the host).
- `make docker-build IMG=…` / `make deploy IMG=…` — image build / kustomize deploy to the current kubeconfig.
- ⚠️ `make test-e2e` creates and deletes a kind cluster (`control-plane-test-e2e`) — slow, CI territory; don't run casually.
- All three test suites (`make test`, `go test ./...`, `yarn test`) **must pass before `deploy.yml` builds or pushes any image**: deploy.yml's `build` job `needs:` all three test jobs.

## Rules

- New Go files carry the Apache-2.0 header from `lego/operator/hack/boilerplate.go.txt`.
- **Mint every resource id through `lego/backend/internal/id` (`id.New(kind)`), never by hand.** Ids are typed opaque `<prefix>-<xid>` strings with a **hyphen** separator (never an underscore — they must stay valid DNS/k8s names); a new id-bearing resource adds its `id.Kind` to that package's registry (which the guard test then enforces). Rationale + known deviations: [docs/ADR020-identifiers.md](../docs/ADR020-identifiers.md).
