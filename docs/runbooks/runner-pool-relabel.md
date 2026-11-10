# Runbook: the self-hosted runner fleet's trust pools (`bex-ci` / `bex-production` / `bex-build` / `other-ci`) — superseded

> **Superseded 2026-11-09.** By owner decision every bex job now runs on a GitHub-hosted runner (`ubuntu-24.04-arm`, or `ubuntu-latest` for linux/amd64 image work); the `bex-ci`, `bex-production` and `bex-build` pools are retired for bex. See [ADR083 §Amendment 2026-11-09](../ADR083-security-review-round20.md#amendment-2026-11-09--github-hosted-runners) and [ADR019 §Decision 5](../ADR019-infra-credentials.md). `scripts/github-actions-validate.sh` now rejects the pool labels below. Other repositories' `other-ci` pool is not covered by that decision. This runbook is kept as the record of the retired fleet; do not use it to route bex jobs.

Bounds ADR083 finding 3 (high): PR-capable test jobs, production-credentialed jobs and main-only image builds are routed into disjoint runner pools. The repo side (workflow `runs-on` labels + `scripts/github-actions-validate.sh` enforcement, `w2/m88`, `w6/m155`) lands fail-safe — until a pool has a runner carrying its label, jobs **queue** instead of running on the wrong pool. This runbook is the operator-side half: registering each runner into exactly one pool. **Host-level separation was rejected by user decision 2026-09-07** (`.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`): the whole fleet runs on the operator's single Mac as a hard constraint, and the shared-host residual is accepted — do not provision separate production hosts or schedule the associated credential rotation.

## Mechanism: labels pick the pool, groups pick the repositories

A bex workflow selects its pool with a fourth runner label — `runs-on: [self-hosted, Linux, ARM64, <pool>]` with `<pool>` one of `bex-ci`, `bex-production`, `bex-build`.

Each pool also registers into the org runner group of the same name, and the group decides which repositories may use it. `bex-ci` (group 3), `bex-production` (4) and `bex-build` (5) grant `bex` only. Other repositories' CI — `beancount-cms-v2`, `eden-cms-v2` and `block-eden-mono` itself — runs on `other-ci` (group 6, since 2026-11-07, w6/m165). Those repositories pin `runs-on: [self-hosted, Linux, ARM64, bex-ci]` or bare `self-hosted`, so the `other-ci` runners keep the `bex-ci` label as well as their own; the group, not the label, keeps them off bex's pools. Before m165 their jobs held two of the seven `bex-ci` runners for half an hour each on a slow link and starved bex deploys.

The runner source is `bex-co/block-eden-mono`, `projects/github-runner/`. Ephemeral runners re-register after every job, so each Compose service sets `RUNNER_GROUP` and the matching `RUNNER_LABELS` entry on every registration; a one-time API relabel is lost after the next job. `RUNNER_GROUP_REPOS` makes `start.sh` re-grant listed repositories to the group on each registration; only `runner-other-ci` sets it, and it must never list `bex`. The group itself must already exist. `runner-config.test.sh` asserts the groups, labels, grants and cache separation.

All pools share Docker Desktop's Linux host — privileged DinD containers share that host kernel. That topology is final (`.pm/DO_NOT_DO.md` `#RUNNER-HOSTS` — see intro).

## Pools, limits and caches

| pool | runners | per-runner limit | caches (Docker volumes, per pool) | repositories |
| --- | --- | --- | --- | --- |
| `bex-ci` | 7 | 4 CPU / 8 GB / 4096 PIDs | `ci-*` npm, yarn, pnpm, Go module cache; `registry-mirror-ci` | `bex` |
| `bex-production` | 3 | 12 CPU / 24 GB / 4096 PIDs | `production-*` package and Go module caches; `registry-mirror-production` | `bex` |
| `bex-build` | 2 | 12 CPU / 24 GB / 4096 PIDs | `build-*` package and Go module caches; `registry-mirror-build` | `bex` |
| `other-ci` | 2 | 4 CPU / 8 GB / 4096 PIDs | `other-*` package and Go module caches; `registry-mirror-other` | `beancount-cms-v2`, `eden-cms-v2`, `block-eden-mono` |

The limits date from 2026-11-05/06 (block-eden-mono `3e43b7c68d`, `7d3a2c28ef`, `47827d2027`). Each `registry-mirror-*` is a `registry:3` pull-through Docker Hub cache that the pool's DinD daemon uses as `--registry-mirror` (`DOCKER_REGISTRY_MIRROR`), so service and base images are not re-downloaded over the host's shared link. Caches are never shared across pools: a poisoned cache in one trust pool must not reach another.

## Pool assignment (the trust classification)

Classified by what each job can touch — secrets, `environment:` gates, and write-capable tokens — not by name. `scripts/github-actions-validate.sh` re-derives the credential side mechanically on every run. The lists below are every runner-backed job in `.github/workflows/` and `lego/operator/.github/workflows/` as of 2026-11-07; reusable-workflow callers (`deploy.yml`'s `test-*`, `native-common-image.yml`'s `publish`) run on the called workflow's pool.

**`bex-production` (31 jobs)** — repository/environment secrets, write tokens, release authorization, cluster access: `alerting-heartbeat.yml` (`heartbeat`), `app-cluster.yml` (`bootstrap-app-cluster`), `build-toolchain-freshness.yml` (`freshness`), `ci-red-streak.yml` (`streak`), `ci-schedule-drift.yml` (`drift`), `cli-release-staleness.yml` (`staleness`), `cli-release.yml` (`authorize`, `release`), `cluster-diagnose.yml` (`diagnose`), `container-policy-drill.yml` (`drill`), `deploy-canary.yml` (`deploy-canary`), `deploy-latency.yml` (`latency`), `deploy.yml` (`build`, `deploy`), `disk-snapshot-drill.yml` (`drill`), `github-environments-drift.yml` (`drift`), `hcloud-csi-rollout.yml` (`cluster`), `helm-mirror.yml` (`mirror`), `infra.yml` (`terraform`), `isolation-matrix.yml` (`isolation-matrix`), `keyvalue-restore-drill.yml` (`restore`), `mobile-release.yml` (`authorize`, `release`), `openbao-restore-drill.yml` (`restore`), `platform-rebuild.yml` (`rebuild`), `production-ssh-trust.yml` (`fingerprints`), `runner-link.yml` (`link`), `snapshot.yml` (`bake`), `ssh-edge-liveness.yml` (`public-edge-liveness`, `request-logs-liveness`, `tenant-view-liveness`). `deploy.yml`'s `build` stays here (registry-push `GITHUB_TOKEN` + cosign OIDC only, secretless by design, w2/m89) so a deploy never queues behind `bex-build`'s hours-long image builds.

**`bex-ci` (30 jobs)** — PR-capable and credential-free: `backend-test.yml` (`backend`), `cli-test.yml` (`cli`), `clusterapi-validate.yml` (`validate`), `dashboard-test.yml` (`dashboard`), `deploy.yml` (`secret-scan`, `check-supersession`), `docs.yml` (`prettier`), `egress-meter-live.yml` (`cilium-live-matrix`, kind-isolated), `gitops.yml` (`cli-analytics`, `render`), `go-lint.yml` (`lint`), `govulncheck.yml` (`govulncheck`), `hcloud-csi-rollout.yml` (`validate`, `observer-channel-smoke`), `infra.yml` (`validate`), `mobile-test.yml` (`mobile`), `native-backport-acceptance.yml` (`acceptance`), `native-base-image.yml` (`matrix`), `native-common-image.yml` (`plan`, `qualify`, `qualify-profile`), `opensandbox-controller-test.yml` (`controller`), `operator-test.yml` (`operator`), `render-schema-drift.yml` (`render-schema`, `render-mcp`), `repo-artifact-guard.yml` (`guard`), `scripts.yml` (`test`), and `lego/operator`'s `lint.yml`, `test-e2e.yml`, `test.yml`.

**`bex-build` (8 jobs)** — main-only platform image publication, `GITHUB_TOKEN` write scopes only (packages, id-token, attestations), no other secret or environment (ADR019 § Decision 5; validator check 6): `cnb-run-image.yml` (`build`), `hcloud-csi-image.yml` (`build`), `kubelet-artifact.yml` (`build`), `log-shipper-image.yml` (`build`), `native-base-image.yml` (`build`), `native-common-image.yml` (`common-arm64-local`), `pg-operand-image.yml` (`matrix`, `build`).

**`other-ci`** — no bex jobs; the other repositories' CI, isolated from bex by its runner group.

## Re-label sequence

Ordering rule: **production first** — a not-yet-labeled pool only queues its jobs, so bringing `runner-bex-production` up first unblocks main-branch deploys before PR traffic resumes; there is no window where a job runs on the wrong pool.

1. **Pool sizing on the single operator host** (`.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`): keep each runner container in exactly one pool, sized per the table above.
2. **Configure durable registration** in the runner source. Each service uses `RUNNER_GROUP=<pool>` and includes its pool label in `RUNNER_LABELS`. Never put two bex pool labels on one bex runner; `other-ci` alone also carries `bex-ci`, for the reason above. Preserve the existing `self-hosted`, `Linux`, `ARM64`, and organization compatibility labels. Create a new pool's org runner group (visibility `selected`) before its first registration.
3. **Apply the services idle-only.** Never recreate a busy runner: replace each container only once `gh api orgs/bex-co/actions/runners` shows its runner (named after the container hostname) idle, then `docker compose up -d --no-deps --no-recreate --scale <service>=<N> <service>`. Never print `docker compose config` with real credentials; the configuration test uses a dummy token.
4. **Moving a repository between groups:** bring the destination pool online and grant the repository there first, then revoke it from the old group (`DELETE orgs/bex-co/actions/runner-groups/<id>/repositories/<repo-id>`) — only after every old-pool runner has re-registered without it in `RUNNER_GROUP_REPOS`, or `start.sh` grants it back.
5. **Verify:** push (or re-run a queued run) and confirm placement per job:

   ```sh
   gh api repos/bex-co/bex/actions/runs/<run-id>/jobs \
     --jq '.jobs[] | [.name, .conclusion, .runner_name, (.labels|join(","))] | @tsv'
   ```

   Done when one run per pool is green **on its intended pool** — originally `w2/m88` t004's evidence for `bex-ci` and `bex-production`; with the `#RUNNER-HOSTS` decision it settles ADR083 finding 3 at the label level.

## Failure modes

- **Jobs queue forever** → a pool has no host carrying that exact label (or a label typo). Compare `gh api orgs/bex-co/actions/runners` labels against the canonical spellings, and check the pool's group grants the repository. Queuing is the designed fail-safe, not an error.
- **Jobs fail in seconds with zero steps** → inspect GitHub annotations and group/repository access settings. Do not infer a plan limitation from this symptom alone; the validator still requires the canonical pool-label form.

## Remote Go cache failures

On 2026-09-05, mobile job `101379637759` attempted to restore a 4.45 GB setup-go cache; after roughly ten minutes Azure rejected the signed request. The setup action remained active after reporting the failure. Operator and CLI tests passed but their cache-save post steps also occupied runners for many minutes. This is the same failure class already documented in `go-lint.yml`.

Backend, mobile, operator, CLI-test and edge-liveness workflows disable the setup-go remote cache. Go still uses its runner-local module and build caches; no test command, failure handling, or production gate is removed. Changes take effect on newly dispatched workflow runs after shipping, not on active jobs.
