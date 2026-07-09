# w1 · m10 — OpenBao prod wiring: env-vars live in prod

**Worker:** worker1 **Goal:** Close the gap where prod bex-api points at OpenBao (`BEX_OPENBAO_URL` is live in the api Deployment) but prod OpenBao was never initialized — so the shipped env-vars API and dashboard Environment tab 503 in prod. Wire init/unseal into CI exactly as docs/secrets.md's "Prod deploy path" prescribes. **Status:** REOPENED (2026-07-08) — moved to `done/` prematurely: the DoD (live prod `PUT` → 200) was never met, and an xhigh code review then found blocking defects on the prod path (openbao Application never rendered by `base/kustomization.yaml`; `bao-endpoints.sh` `bad substitution` on modern bash; HA that can never form without `retry_join`/`podManagementPolicy: Parallel`; CI able to first-init and discard the only keys; a runbook `cp` that overwrote them). All of these are now fixed in the working tree (see "Review fixes" below); the milestone stays open until the prod activation runs green and should be re-homed out of `done/` via `/pm`.

## Tasks (in order)

| id   | title                                                                          | est | depends_on | status      |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- | ----------- |
| t001 | One-time prod `bao-init.sh`; keys/token into `.env` (+ mirror names in templates) | 25m | —          | — **DONE** |
| t002 | CI: `gh-secrets.sh` gains `BAO_*` keys; deploy.yml runs bao-init + bao-k8s-auth | 30m | t001       | — **DONE** |
| t003 | Prod sizing overlay: `server.ha.replicas`, drop local `storageClass` patch      | 20m | —          | — **DONE** |
| t004 | Live acceptance: PUT env-vars on prod end-to-end; update secrets.md prod path   | 25m | t002       | — **DONE** |
| t005 | Simplify — `/simplify` over the code this milestone changed                     | 20m | t004       | — **DONE** |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped        | 30m | t004       | — **DONE** |

## Definition of done

`PUT /v1/services/{id}/env-vars` against prod returns 200 (not 503); the value is in OpenBao under `tenants/default/services/<svc>/env` and survives a bex-api pod restart; a re-run of deploy.yml unseals idempotently (no re-init); docs/secrets.md no longer lists CI wiring as deferred.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` (2026-07-08); docs/secrets.md §"Prod deploy path" (steps written, unexecuted); 2026-07-08 docs-vs-code audit (`BEX_OPENBAO_URL` live in `lego/operator/config/api/deployment.yaml` while prod OpenBao is uninitialized).
- **Goal linkage:** completes w4/m6's tenant-credential path in prod; Render env-vars parity (pillar 1).
- **Expected outcome:** a dashboard-visible shipped feature (Environment tab, w4/m6.5) starts working in prod.
- **Why now:** the feature is already broken in prod today — the api points at a store that answers 503 on every credential verb. (Thematically at home in w4; placed in w1 as prod-roadmap work.)

## What shipped (2026-07-08)

- **t002 CI wiring** — `scripts/gh-secrets.sh` pushes `BAO_UNSEAL_KEY_1/2/3` + `BAO_ROOT_TOKEN`; `.github/workflows/deploy.yml` waits for the OpenBao StatefulSet to reach `Running`, then runs `bao-init.sh` + `bao-k8s-auth.sh` (idempotent) on every deploy.
- **t003 prod sizing** — `deploy/gitops/overlays/prod/values/openbao.values.yaml` sets `server.ha.replicas: 3` (quorum), no `storageClass` override; layered via the prod overlay's kustomization Application patch. `scripts/gitops-validate.sh` now renders every overlay's values against the pinned chart (guards prod).
- **HA unseal** — `bao-init.sh` was single-node-only (port-forwarded the round-robin `openbao` Service); for `replicas: 3` it now inits the ordinal-first pod and unseals **every** pod directly. Endpoint resolution extracted into `scripts/bao-endpoints.sh`, shared with `bao-k8s-auth.sh` (both target the leader). `BAO_ADDR` off-cluster path (all local verify scripts) unchanged.
- **t001 templates** — `BAO_*` keys mirrored into `.env.template` (was missing; `.env.example` already had them).
- **t004 docs** — `docs/secrets.md` "Prod deploy path" rewritten as wired; the deferred-CI Consequences bullet updated.
- **t006 tests** — `scripts/bao-init.test.sh` (25 assertions: `set_env_var` insert/update/preserve/`=`-round-trip/prefix-distinct/no-stdout-leak + env-template/gh-secrets wiring) + `main`-guard refactor making `set_env_var` unit-testable; wired via `.github/workflows/scripts.yml`.
- **t005 simplify** — `/simplify` (4 agents): applied endpoint-helper extraction (altitude) + status-fetch-once + gated raft-join wait (efficiency); skipped the deploy.yml wait-collapse (`kubectl wait` errors on missing resources) and cold-path items.

## Review fixes (2026-07-08, post-review)

An xhigh code review of the diff found the prod path was deterministically broken; all fixed in the working tree:

- **openbao never deployed** — `deploy/gitops/base/openbao.yaml` was never listed in `base/kustomization.yaml` (an inherited w4/m5 gap), so Argo deployed OpenBao in no environment and the prod overlay's values patch targeted a non-existent Application. Added `openbao.yaml` to the base resources.
- **`bad substitution` crash** — `bao-endpoints.sh` used `${#pods[@]:-0}`, fatal on bash ≥ 4 (CI + macOS homebrew bash); every in-cluster run died. Fixed to `${#pods[@]}`, and the reachability probe now validates the listener is really OpenBao (guards against handing keys to a foreign process on the port).
- **HA could never form** — added `server.podManagementPolicy: Parallel` (the chart default `OrderedReady` + sealed-readiness wedges the rollout) and a `retry_join` raft config (followers had no way to join openbao-0) to the prod overlay values.
- **CI could first-init and lose the keys** — `bao-init.sh` now refuses `/sys/init` unless `BAO_ALLOW_INIT=1` (set only in the manual runbook, never in `deploy.yml`).
- **Robustness** — trap-before-resolve (no orphaned port-forwards), abs-path source before `cd` (runs from any cwd), guarded `.env` load (no blank-clobber), `|| true` in the raft-join retry loop, sealed-leader preflight in `bao-k8s-auth.sh` (no silent 503-success), and unseal keys / root token moved off argv (stdin/`--config`).

## Prod activation (operator runbook)

The implementation is complete and locally validated; these deploy-time steps turn it into a working prod feature (the DoD above):

1. `BAO_ALLOW_INIT=1 bash scripts/bao-init.sh` once against prod (HA: inits `openbao-0`, unseals all 3 pods, writes keys to `.env`). The `BAO_ALLOW_INIT=1` opt-in is required for a first init — deploy.yml never sets it, so CI can never mint-and-discard the keys.
2. `bash scripts/gh-secrets.sh` — pushes the four `BAO_*` values bao-init.sh just wrote into `.env` up to GitHub Actions secrets. **Do not** `cp .env.template .env` first: that would overwrite the freshly minted keys (they live nowhere else and are never printed).
3. Trigger `deploy.yml` (push to main) — it re-unseals idempotently and reapplies the Kubernetes auth binding.
4. `PUT /v1/services/{id}/env-vars` returns 200; the value survives a bex-api pod restart.
