# w1 · m5 — Build & deploy from git, in-cluster

**Worker:** worker1 **Goal:** Make `App.spec.repo` (build-from-git) work **in-cluster** on containerd nodes, image pullable by nodes, and redeploy on push. Today the operator shells `docker build` (impossible on containerd — the demo built locally and `ctr`-imported by hand). **Status:** in progress — t001–t003 DONE (2026-07-08, verified live); t004 (Simplify) + t005 (Test coverage) remain.

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- |
| t001 | Build via BuildKit/kpack Job → push to Zot — **DONE**                        | 30m | —          |
| t002 | Verify build-from-git end-to-end (node pulls Zot) — **DONE**                 | 25m | t001       |
| t003 | Push webhook shipped in w2 (deploy-from-chat) — verify vs m5 acceptance, close — **DONE** | 15m | —          |
| t004 | Simplify — `/simplify` over the code this milestone changed                  | 20m | t002, t003 |
| t005 | Test coverage — meaningful tests for the build path (+ webhook gap-fill)     | 30m | t002, t003 |

## Definition of done

Setting `App.spec.repo` builds the image in-cluster (Dockerfile or CNB), pushes to Zot, and runs it; a git push to `App.branch` (with `autoDeploy`) bumps the revision automatically.

## Current state (2026-07-07)

- **Host-based MVP exists** (`lego/operator/internal/build/build.go`): `Build()` clones the repo, runs `docker build` (Dockerfile) or `pack build` (CNB) via `exec.Command`, and pushes to `BEX_REGISTRY`. This proved the full git-clone → build → push → run flow but shells out to host tools — impossible on containerd app nodes. Comment in the file explicitly calls out "an in-cluster BuildKit/kpack Job is the productionization."
- **Zot already deployed**: `deploy/gitops/base/zot.yaml` → `zot-0` StatefulSet in-cluster; nodes already pull from it (`BEX_REGISTRY` points there).
- **t001**: replace `exec.Command("docker", ...)` / `exec.Command("pack", ...)` in `build.go` with dispatching a Kubernetes Job that runs `moby/buildkit` or `kpack`. The `Build()` function signature can stay; only the backend changes.
- **t003 overtaken (2026-07-08)**: the HMAC push-to-deploy webhook shipped in bex-api via w2's deploy-from-chat work (`lego/backend/internal/apps/webhook.go`, `POST /v1/webhooks/git`, `BEX_WEBHOOK_SECRET`; docs/deploy-from-chat.md) — in the backend, as an API surface, exactly where it belongs (operator is mechanism-only). t003 reduces to verifying it satisfies m5's acceptance and closing.

## Source + Goal linkage

- **Source:** converted from `.tmp/008-in-cluster-builds-webhook.md`. Retrofitted to current `/pm` conventions 2026-07-08.
- **Goal linkage:** V0 roadmap #3 ("Git push to deploy") — the defining Render flow; pillar-1 Render parity (`repo`/`branch`/`autoDeploy` are Render service fields bex already accepts).
- **Expected outcome:** a `bex.yml` with `repo:` deploys with no local docker and no manual `ctr` import; a push to the tracked branch redeploys hands-free.
- **Why now:** build-from-git is advertised in the contract (`App.spec.repo`, `BEX_CNB_BUILDER`) but **broken on prod containerd nodes today** — the shipped demo path only worked by hand-importing images. The webhook half already exists; the build half is the last missing piece of the flow.

## Completion notes (2026-07-08)

- **t001 — in-cluster BuildKit Job** (`lego/operator/internal/build/build.go`): `Build()` no longer shells `docker`/`pack`; it dispatches a **rootless-BuildKit** Kubernetes Job that git-clones the repo, builds the Dockerfile, and pushes `<registry>/<name>:gen-<generation>` to Zot, blocking until the Job completes (idempotent per revision). Wired `BEX_REGISTRY`→Zot + `BEX_BUILD_NAMESPACE`; `batch/jobs` RBAC added, codegen re-run. **CNB (`builder: buildpack`) is out** — needs kpack, so it reports a clear error instead of silently failing on containerd.
- **t002 — verified live**: ran the operator against a real apiserver → a repo-backed App dispatched a valid `bld-<name>-gen-1` Job that a containerd node **scheduled and ran rootless**; BuildKit git-cloned and **built a real image** (exported manifest + config), failing only at the push because no Zot was in the bare test cluster (a platform concern). Verification also **caught + fixed a real bug**: BuildKit was fetching the `https://…` URL as an HTTP file (parsing GitHub's HTML as the Dockerfile) — fixed by forcing a git context (`.git` suffix).
- **t003 — done**: the bex-api webhook satisfies m5's acceptance; this milestone added the missing **`autoDeploy` gate** (webhook skips `autoDeploy:false` Apps; repo-backed creates default `autoDeploy:true`, threaded across REST/GraphQL/MCP/bex.yml).
- **Remaining:** t004 (run `/simplify` over the changed code) + t005 (a dedicated test-coverage pass — build unit tests already added in `build_test.go`; t005 gap-fills the webhook side). **Unblocks w2/m2 t004** (deploy-from-chat live acceptance).

## Notes

- Build config already exists: `BEX_REGISTRY` (Zot) and `BEX_CNB_BUILDER` env vars drive the current (docker-shell) path in `lego/operator/internal/build/build.go` — t001 replaces the execution, not the contract.
