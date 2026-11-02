# w6 · m155 — Image builds run on their own runner pool, so production deploys never wait behind them

**Worker:** worker6 **Goal:** a production deploy starts within minutes of a green main even while hours-long platform image builds run. **Status:** todo (t001–t004 done; t005 live proof next)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Add the `bex-build` runner pool](done/t001.md) — **DONE** | 40m | — |
| t002 | [Teach the workflow validator the `bex-build` trust class](done/t002.md) — **DONE** | 45m | — |
| t003 | [Move the image-build jobs onto `bex-build`](done/t003.md) — **DONE** | 30m | w6/m155/t002 |
| t004 | [Record the third pool in ADR019/ADR083 and the runner README](done/t004.md) — **DONE** | 20m | w6/m155/t003 |
| t005 | [Prove a deploy no longer waits behind a native build](t005.md) | 30m | w6/m155/t001, w6/m155/t003 |
| t006 | [Simplify](t006.md) | 15m | w6/m155/t005 |
| t007 | [Test coverage](t007.md) | 20m | w6/m155/t006 |
| t008 | [Closeout](t008.md) | 10m | w6/m155/t007 |

## Definition of done

- A third org runner pool `bex-build` (2 ephemeral runners, own runner group granted only to `bex-co/bex`, own package caches) is registered and online, from `block-eden-mono/projects/github-runner`.
- `scripts/github-actions-validate.sh` accepts exactly `bex-ci` / `bex-build` / `bex-production`. A `bex-build` job must run on `main` only and reject `pull_request`. It may hold only `GITHUB_TOKEN` write scopes (`packages: write`, `id-token: write`, `contents`/`attestations` as needed) and never `secrets.*` or an `environment:`. Each rule has a red/green test.
- Platform image builds that need no production secret run on `bex-build`: native base/common, cnb-run, pg-operand, hcloud-csi image, kubelet artifact, log-shipper image. The deploy pipeline's own `build` and `deploy` jobs, and every credentialed job, stay on `bex-production`.
- Live: while a native image build runs, a deploy run's `build` job gets a `bex-production` runner within 5 minutes. Run ids recorded.

## Source + Goal linkage

- **Source:** user decision D4 = "拆出 bex-build 池" (2026-11-01, w6 session), after w6/099's fix waited 11h17m to deploy (pin 21:19 → 08:36). All three `bex-production` runners were held by `native base images` (`max-parallel: 2`, 40 runs in 20 h, 1–3 h per image) and a `hcloud CSI` dispatch. 36 jobs share the pool. Image builds sit there only because the validator puts every write-token job on `bex-production` (rule 6).
- **Goal linkage:** ADR008 reliable delivery. Deploy latency was w6/087's starvation class, kept closed with a 12 h reopen threshold.
- **Expected outcome:** deploys and drills keep their pool. Long builds queue only behind each other.
- **Why now:** 34 of the last 40 native runs were manual qualification dispatches for w1/m188 and w8/m91, and more are coming. D3 (amd64-only production qualification, same date) shrinks each build but not the contention.
- **Render parity:** omitted. CI infrastructure, no tenant-facing surface.
- **Constraints:** `.pm/DO_NOT_DO.md` `#CI-RUNNERS` (no GitHub-hosted runners) and `#RUNNER-HOSTS` (the pools share the operator's host by user decision) still hold. Host capacity for two more runners (4 CPU / 8 GB burst each) must be checked in t001.

## Implementation — 2026-11-02

- **t001:** org runner group `bex-build` (id 5, visibility selected, granted to `bex` only) created. `block-eden-mono` `aa911b0693` adds `runner-bex-build` (2 replicas, labels `…,bex-co,stargately,bex-build`, own npm/yarn/pnpm caches); `runner-config.test.sh` now asserts three pools with disjoint labels and caches (red/green). Started on the runner host, which is the operator's Docker Desktop: the live deployment checkout is `/Volumes/nvme4tbfish/block-eden-mono`, not `projects/`. Both runners are online in group 5 (`0470aa849d3b`, `33dca3c6a938`). Host: Docker VM 15 CPU / ~51 GB; 12 runners at 0.5 CPU / 512 MB reserved each, bursts over-commit CPU (accepted, as before).
- **t002:** `scripts/github-actions-validate.sh` checks 5–6 accept `bex-build`. A `bex-build` job may hold only GITHUB_TOKEN (case-insensitive), no other secret or the whole `secrets` object, no `environment:`, no `write-all`, must carry a job-level `if: github.ref == 'refs/heads/main'` (optionally `&& …`), and rejects pull_request. A caller must not pass `secrets:` into a reusable workflow that has a `bex-build` job. Separately, the whole-object `toJSON(secrets)` now counts as credentialed for every pool, closing an older gap. 16 red/green cases cover these rules.
- **t003:** `native-base-image`, `cnb-run-image`, `pg-operand-image` (matrix + build, build job gained an explicit main gate), `hcloud-csi-image`, `kubelet-artifact` and `log-shipper-image` now run on `bex-build`. `deploy.yml` build/deploy and all credentialed jobs stay on `bex-production`. The 139 `.github/scripts` workflow tests pass.
- **t004:** ADR019 §Decision 5, ADR083 and `docs/runbooks/runner-pool-relabel.md` describe three pools.
