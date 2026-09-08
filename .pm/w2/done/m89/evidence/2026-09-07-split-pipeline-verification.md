# m89 split-pipeline live verification — 2026-09-07

Why: t004 requires one real production deploy end to end through the secretless
`build` job and the environment-gated `deploy` job, with the deployed workloads
running the digests build emitted.

## Run 1 — `34165067426` (head `7c17e931e`, the m89 ship itself): success

First live run of the split workflow. Proved the mechanism:

- Six gates green on `bex-ci` runners (backend `4488b43a5034`, operator
  `c57df3d66c97`, dashboard `7b984aaf270f`, controller `974e367eae33`,
  secret-scan, check-supersession).
- `build` job: success on `bex-production` runner `6edea777f8d2` (22:15–22:40Z)
  — checkout, buildx, GHCR login, five image builds, cosign/SBOM, trivy; its
  only secret reference is `secrets.GITHUB_TOKEN` (workflow YAML, enforced by
  validator check 8).
- `deploy` job: success on a **different** `bex-production` runner
  `b3667097eb79` (22:40–22:42Z) behind `environment: production-deploy` — the
  digest handoff crossed runners via job outputs, so no host-local state links
  the jobs.
- The deploy correctly took the **superseded** neutral path (w1/m59): commits
  landed on main during the build (head `25e8cbc9f` triggered a newer deploy
  run), so the pin write-back step exited neutral and every rollout step
  reported `skipped` — TLS validate, kubeconfig fetch, Argo install, repo
  creds, app-of-apps, control-plane credential, auth secrets, bootstrap
  client, authz model, and OpenBao unseal all still ran green inside the gated
  job. Exactly the designed behavior: never pin images built from a stale
  production input.

## Run 2 — `34165277329` (head `25e8cbc9f`): backend gate red, build/deploy skipped

`test-backend` failed on `TestAccountDeletionOpsWorkspaceBlockedPG` — a test
new in `25e8cbc9f` itself (w5/m86). Root cause diagnosed and fixed this
session: `accountDispositionWithoutMachines` excluded machine subjects with
`NOT (subject = ANY($2::text[]))`; a nil slice arrives as SQL NULL, `ANY(NULL)`
is NULL, so the filter dropped every member row and every workspace
misclassified as `blocked`. Deterministic (reproduced on a local throwaway
Postgres; a `--failed` rerun failed identically). Fixed with
`COALESCE($2::text[], '{}')` in `42d4532a3`; failing test + full
`internal/store` + `internal/accounts` green against real PG locally, then in
CI. The gates correctly kept the broken commit out of build and deploy —
fail-closed exactly as designed.

## Run 3 — `34168435249` (head `42d4532a3`, the fix): green, superseded again

All gates green (backend gate recovered), `build` success on `bex-production`
runner `6edea777f8d2`, `deploy` success on `bex-production` runner
`57789c8a59a8` — second live proof of the split mechanism, again across two
different production runners. The pin step again took the neutral superseded
path (`cca2f571d`/`6158d8630` landed during the build), so rollout legs
skipped by design.

## Run 4 — `34170037075` (head `e609e9322`): green, superseded a third time

Gates + `build` + `deploy` all green (third mechanism proof); `d58245084`
landed during its build, so the pin/rollout legs again skipped by design.
Concurrent workers were pushing deploy-triggering commits faster than one
~40-minute build — the supersede logic (w1/m59) coalesced them correctly every
time instead of pinning stale images.

## Run 5 — `34171816919` (head `d58245084`): END-TO-END COMPLETE

The quiet-window run. Every job and every deploy step succeeded — nothing
skipped:

- `build`: success on `bex-production` runner `6edea777f8d2`; secret scope =
  `GITHUB_TOKEN` only.
- `deploy`: success on `bex-production` runner `57789c8a59a8` (fourth
  distinct build/deploy runner pairing across the day's runs), behind
  `environment: production-deploy`. Steps 7–28 all `success`: digest pin
  write-back, kubeconfig fetch + scrub, onbex TLS install + public verify,
  chart mirror, Argo install/creds/app-of-apps, control-plane + OpenSandbox
  credentials, auth secrets, bootstrap client, authz model, OpenBao unseal,
  operator wait, bex + ssh-gateway + dashboard rollouts, PROXY-proxy wait,
  final credential scrub.
- Pin write-back commit on main: `5e0ff91ac` "chore(deploy): pin platform
  images to d5824508480c [skip ci]" — operator
  `sha256:212d1a54…`, dashboard `sha256:eab554ad…`, opensandbox-server
  `sha256:d89956bb…`, opensandbox-controller `sha256:07961f6f…`,
  agent-sandbox `sha256:9dbbeaa9…`.
- **Digest match proven in production**: steps 19 ("wait for OpenSandbox
  control plane") and 26 ("wait for PROXY-compatible datastore proxies")
  assert the in-cluster Deployment/DaemonSet images equal
  `needs.build.outputs.*` — both succeeded, so the cluster runs exactly the
  digests this run's secretless build emitted.

DoD clause "one real production deploy shipped end to end through the split
pipeline" holds on run 5; the supersede-protection behavior got three live
proofs for free along the way.

## Validator + CI evidence

- `scripts (test)` run for `7c17e931e` (validator check 8 + new fixtures) and
  `gitops (render)` run (updated `.jobs."deploy"` guards) both green in CI.
- Local: `github-actions-validate.sh`, its self-test (including the
  bracket-form-secret and annotated-job-key red fixtures added after the
  simplify review), and `gitops-validate.sh` all pass.
