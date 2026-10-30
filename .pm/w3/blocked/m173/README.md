# w3 · m173 — Production credentials reach only main-branch production jobs

**Worker:** worker3 **Goal:** no workflow on a non-main branch, and no job outside its `production-*` GitHub environment, can read a production credential; the environment gates ADR080 declared and ADR083 relies on actually hold; a group- or world-readable local `.env` is refused **Status:** blocked — t001–t004, t006, t008, t009 done; release-tag policy fixed 2026-10-29; t005 needs the user to choose how the exact live secret values are sourced (A/B/C below); t007, t010 follow

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Map every production secret to the jobs that read it — **DONE** | 30m | — |
| t002 | Decide the gate shape; amend ADR083 obligation 5 and ADR019 §Decision 5 — **DONE** | 30m | t001 |
| t003 | Move every production-secret job into its environment — **DONE** | 45m | t002 |
| t004 | Fail closed on a production secret outside a production environment; scheduled drift check — **DONE** | 1h | t003 |
| t005 | Apply the GitHub settings: environments, branch policies, reviewers, secret scope (needs your authorization) | 45m | t003 |
| t006 | Refuse a group- or world-readable `.env` in every loader — **DONE** | 40m | t002 |
| t007 | Live proof: a branch probe sees no production secret; main stays green | 45m | t004, t005, t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- `gh api repos/bex-co/bex/environments` shows every `production-*` environment in `scripts/production-secrets.txt` with a main-only deployment-branch policy (no reviewer requirement — t002, `#ENV-REVIEWERS`); `production-snapshot` and `production-observe` exist; `scripts/github-environments-drift.sh` exits 0 against the live repo.
- No production-class secret (the t001/t002 list) remains at repository scope; a names-only listing of repository and environment secrets is recorded.
- A `workflow_dispatch` probe on a throwaway branch evaluates `secrets.BAO_ROOT_TOKEN != ''` as `false` (boolean only, never the value); run id recorded, branch deleted.
- `scripts/github-actions-validate.sh` fails on a fixture job that reads a production-class secret without a `production-*` environment, and the scheduled drift check opens one tracking issue on a fixture drift and closes it on recovery.
- On `main`, the next `deploy.yml`, deploy canary, production edge liveness and one restore drill run green after the move; run ids recorded.
- Every script that sources `.env` refuses a group- or world-readable file with a named error and accepts a `0600` file.

## Evidence (2026-10-19, read-only)

- `gh api …/environments`: `production-cluster`, `production-deploy`, `production-release` have **no** protection rules; `production-infra` has a branch policy and no reviewers; `production-snapshot` does not exist; only `production-restore` has required reviewers.
- Repository-scope secrets (names only): `AGE_BACKUP_PRIVATE_KEY`, `ARGOCD_REPO_SSH_KEY`, `BAO_ROOT_TOKEN`, `BAO_UNSEAL_KEY_1..3`, `BEX_SSH_PRIVATE_KEY`, `HCLOUD_TOKEN`, `TF_STATE_*`, `KRATOS_SECRETS_*`, `HYDRA_SECRETS_*`, `OPENFGA_PRESHARED_KEY` and others — 37 in total. Environments hold only `BEX_ONBEX_TLS_CERT`/`_KEY` (`production-deploy`) and `EXPO_TOKEN` (`production-release`). So `production-restore`'s reviewers do not guard the backup decrypt key either.
- Secret-reading workflows with no `environment:` at all: `cnb-run-image.yml`, `deploy-canary.yml`, `isolation-matrix.yml`, `log-shipper-image.yml`, `ssh-edge-liveness.yml`. Environment lines today: `app-cluster.yml:58`, `cli-release.yml:46`, `deploy.yml:438`, `infra.yml:84`, `mobile-release.yml:59`, `openbao-restore-drill.yml:45`, `keyvalue-restore-drill.yml:51`, `snapshot.yml:37`.
- Local custody: six sibling checkouts (`bex-security`, `bex5`, `bex6`, `bex7`, `bex8`, `bex10`) hold `.env` at `-rw-r--r--` (permissions checked, contents not read); only `scripts/bao-init.sh:34-44` re-applies `0600`, so any other copy is created world-readable — ADR079 finding 1's regression.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-10-19, item 1 (approved by the user the same day). Findings: [ADR080](../../../docs/ADR080-security-review-round19.md):58 (environment creation + reviewers left as an operator action), [ADR083](../../../docs/ADR083-security-review-round20.md):101 (obligation 5), [ADR079](../../../docs/ADR079-security-review-round18.md):30 (finding 1, `.env` custody + rotation).
- **Goal linkage:** V0 roadmap 7 (security review); the ADR028 → ADR085 review lineage; [ADR019](../../../docs/ADR019-infra-credentials.md) §Decision 5 credential custody.
- **Expected outcome:** the OpenBao root token, unseal keys, Hetzner token, SSH key, Terraform state and backup decrypt key are reachable only from main-branch jobs in their production environment; the gates the accepted runner-compromise risk (ADR083) assumes are real and guarded against drift.
- **Why now:** ADR083 accepted a critical-severity residual on the condition that these gates hold (obligation 5), and they do not; today any workflow run on any branch can read every production credential. The fix is small, and every new workflow added in the meantime widens the surface.
- **Render parity omitted:** no REST/GraphQL/MCP/UI change — CI workflow files, scripts and GitHub repository settings only.
- **Constraints:** stay on self-hosted runners (`DO_NOT_DO.md` `#CI-RUNNERS`); no runner-host change (`#RUNNER-HOSTS`). t005's GitHub settings changes and any credential rotation are outward-facing and need explicit user authorization.

## Unblock 记录（2026-10-29，/unblock w3，origin/main `e4d9c65d6`）

- **涉及：** t005、t007、t010。
- **用户明确决定 D2（2026-10-29，听完白话解释后确认）：**
  1. 授权 t005 的在线 GitHub 设置变更：新建 `production-snapshot`（`production-observe` 已存在，复核于 2026-10-29），让 `scripts/production-secrets.txt` 里的全部 `production-*` 环境都只允许 main 分支部署（不设 reviewer，见 `#ENV-REVIEWERS`）；运行 `bash scripts/gh-secrets.sh` 推送环境级副本；等每个受影响的 workflow 在 main 上各绿一次后，删除仓库级副本（以及 `BEX_AGENT_SNAPSHOT_S3_*`、`BEX_STRIPE_WEBHOOK_SECRET_TEST`）。
  2. 授权 t007 推一次性探针分支并 dispatch（只打印布尔值），完成后删除该分支。
  3. **不轮换凭据**：单用户 Mac，没有泄露证据（与 `#RUNNER-HOSTS` 一致）。ADR079 finding 1 的 rotation 按此结论记为不需要。
  4. 本地 `.env` 收紧权限：已由 `/unblock` 在 2026-10-29 执行 `chmod 600`，`bex-security`、`bex3`、`bex7`、`bex8` 现在都是 `-rw-------`；`bex5`、`bex6`、`bex10` 原本就是 0600。
- **复核现状：** 仓库级 secret 仍是 37 个；drift issue #101 仍然开着（预期如此，t005 完成后应自动关闭）；`production-restore` 的 reviewer 已于 2026-10-29 移除（`0fc15e4b8`）。
- **剩余阻塞：** 无。worker 严格按顺序执行 t005 → t007 → t010，在 t005/t007 里记录 run id 和只列名称的 secret 清单。

## loopx w3 (2026-10-29): two findings before the live apply

1. **Fixed — release tags.** `cli-release.yml` / `mobile-release.yml` run on `bex-cli/v*` / `bex-mobile/v*` tag pushes in `production-release`; a strict main-only policy would have refused every CLI and mobile release. `scripts/github-environments-drift.sh` now requires `production-release` = `main` + exactly those two tag rules (every other environment: exactly `main`), the self-test proves it and checks the allowed tags equal the workflows' `tags:` triggers, and ADR083 obligation 5 records the exception (their unprivileged `authorize` job already refuses a tag not on `main`). t005 step 2 therefore sets `production-release`'s policy to `main` + those two tag rules.
2. **New gate — secret values.** GitHub secrets are write-only, and no local `.env` holds them all (names-only check, 2026-10-29): `ARGOCD_REPO_SSH_KEY` and `BEX_TAP_PUSH_KEY` exist in no `.env`; the rest are split across `bex3`/`bex5`/`bex6` and may be **stale** — e.g. the CP nodes were replaced 2026-10-29, so a local `BEX_SSH_KNOWN_HOSTS` can differ from the live one. An environment secret overrides the repository one of the same name, so pushing a stale value would break production jobs. Options needing the user's choice: **(A)** a temporary main-only `workflow_dispatch` workflow, one job per environment (passes check 9), encrypts the values it already sees to a one-off local public key, uploads them as a 1-day artifact, and is deleted right after; the values are decrypted locally and pushed with `gh secret set --env` — exact live values, never plaintext anywhere new; **(B)** the user supplies a complete, current `.env` (or `*_FILE` paths) and confirms it is current; **(C)** move only the secrets whose local value can be verified against the live cluster (hash comparison) and leave the rest at repository scope (DoD not met).
