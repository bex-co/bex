# Contributing to bex

Thanks for helping build the open-source Render alternative. Issues and PRs welcome.

## Prerequisites

- Go 1.25.x (see `operator/go.mod` for the exact toolchain)
- Docker (OrbStack works), `kubectl`, `kind`, `clusterctl`, `yq`

## Dev loop (all from `operator/`)

```bash
make test        # unit + envtest; first run downloads envtest binaries to bin/
make lint        # golangci-lint (make lint-fix to auto-fix)
make run         # run the operator from your host against the current kubeconfig:
                 #   make install && BEX_RUNTIME=kubernetes make run
```

- After touching `api/v1alpha1/*_types.go`, run `make manifests generate` and commit the regenerated files (`zz_generated.deepcopy.go`, `config/crd/bases/`).
- `make test-e2e` creates and deletes a kind cluster (`control-plane-test-e2e`) — it's slow and meant for CI; don't run it casually.

## Local cluster

The full local mock (kind infra cluster + Cluster API + Docker-container "machines") is one command: `bash scripts/mock-cluster.sh`. The README quickstart walks through deploying the operator and a sample App onto it.

## Conventions

- **Commits:** Conventional Commits (`feat(operator): …`, `fix(ci): …`).
- **License:** Apache-2.0, inbound = outbound, no CLA. New Go files carry the header from `operator/hack/boilerplate.go.txt`.
- **Markdown:** CI checks formatting — fix with `npx prettier@3.4.2 --write "**/*.md"`.
- **Docs:** product/design docs live in `docs/`; `.pm/` is internal maintainer notes, not documentation.

This repo is agent-friendly — see [CLAUDE.md](CLAUDE.md). PRs authored with coding agents are welcome as long as tests pass and you've reviewed the diff yourself.
