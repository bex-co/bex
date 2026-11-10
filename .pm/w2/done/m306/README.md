# w2 · m306 — CI on main reports only real reds after the move to GitHub-hosted runners

**Worker:** worker2 **Goal:** a red workflow on main means a real defect again: `scripts (test)` is green, every step it skipped since 2026-11-08 runs, and the red-streak and schedule-drift trackers stop listing signals that are stale or mis-calibrated. **Status:** done` push runs)

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | `scripts (test)` is green: a reasoned SC2218 directive in `mock-cluster.test.sh`; `dev-env.test.sh`'s `assert()` evaluates without `pipefail` — **DONE** | 20m | — |
| t002 | The steps CI has not reached since 2026-11-08 (mock-cluster, restore and disk-snapshot tests) pass on `ubuntu-24.04-arm` — **DONE** | 30m | t001 |
| t003 | Local and CI run the same ShellCheck (0.11.0 in `scripts.yml` and the `github-actions-validate.sh` sha table) — **DONE** | 20m | t001 |
| t004 | `ci-red-streak.sh` groups runs by workflow id, and a later success of any event ends a jobless parse-failure streak — **DONE** | 45m | — |
| t005 | `ci-schedule-drift.sh` windows recalibrated from measured GitHub-hosted cron gaps; the deleted `runner-link.yml` dropped — **DONE** | 20m | — |
| t006 | Simplify — **DONE** | 15m | t002, t003, t004, t005 |
| t007 | Test coverage — **DONE** | 30m | t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- `scripts (test)` passes on two consecutive push runs on main, with every step executed (none skipped by an earlier failure).
- A locally clean `shellcheck` run means a clean CI run: both use the same version.
- The next scheduled `ci-red-streak` run no longer lists `scripts (test)` or `.github/workflows/native-base-image.yml`. Issue #109 lists only reds that are tracked elsewhere (`postgres operand image` → `w8/blocked/m92` t008; the native-common hosted failure, recorded below).
- The next `ci-schedule-drift` run no longer lists `alerting-heartbeat.yml`, `deploy-latency.yml`, `ssh-edge-liveness.yml` or `runner-link.yml` for gaps inside the measured hosted-cron spread. Issue #112 closes or lists only gaps beyond the new windows.
- Red and green fixtures prove both detectors still catch a real streak and a real gap.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2`, 2026-11-10, item 1. Evidence:
  - GitHub #109 ("A workflow is persistently red on main"): `scripts (test)` 22 consecutive failures, first at `e22f9cad3` (w2/m289), run 42530480243. That run flagged `scripts/mock-cluster.test.sh` line 388 `operator_import_image` as SC2218. The latest, run 42787673233, flags 13 call sites.
  - Cause A: CI pins ShellCheck 0.10.0 (`.github/workflows/scripts.yml:198`, sha in `scripts/github-actions-validate.sh:702-704`). The test sources `scripts/mock-cluster.sh` with `# shellcheck source=/dev/null` and then redefines functions as stubs further down, so 0.10.0 reports calls before definition. Local 0.11.0 does not, which is how it shipped. Verified with `koalaman/shellcheck:v0.10.0`: a `# shellcheck disable=SC2218` on line 2 gives rc=0 on 0.10.0 and 0.11.0. The other 39 listed files are clean under 0.10.0.
  - Cause B: `scripts/dev-env.test.sh:15-20` `assert()` evals `sed -n '/^start_api()/,/^}/p' … | grep -q` under `set -euo pipefail`. `grep -q` exits early, sed gets EPIPE (`sed: couldn't flush stdout: Broken pipe`, runs 42744186783 and 42529060683), and `pipefail` turns a true condition into `FAIL: status checks Prometheus ready` (the failing check varies). It accounts for 7 of the streak's 27 failures and hides cause A when it fires. Present since `aecc49c16` (2026-11-07).
  - `native-base-image.yml`: the counted failures (run 41020368108 and siblings, 2026-10-30) are jobless parse failures named after the file path. The workflow is now `workflow_dispatch`/`workflow_call` only and its dispatches pass (41360577241). `scripts/ci-red-streak.sh:88` groups by `.name` and counts only push/schedule events, so this group can never clear.
  - GitHub #112: GitHub-hosted cron creates only 2–6 of `alerting-heartbeat`'s 24 hourly runs a day (median gap 5.2h, p90 8h, max 10.8h; 32 of 85 gaps over its 6h window). `deploy-latency` max gap 10.5h against 9h. `ssh-edge-liveness` max 15.5h against 12h (7 of 186). Runs start as soon as they are created and nearly all succeed: the lateness is GitHub not creating them, not queued or failing probes. `runner-link.yml` was deleted in `4630a48a7`. Windows: `scripts/ci-schedule-drift.sh:61-65`.
- **Goal linkage:** the CI gates in AGENTS.md § Commands, and the rule "never stop or wait because main's CI is red … fix CI first" (user decision 2026-10-21).
- **Expected outcome:** a red badge on main, an open #109 or an open #112 again names something a worker must act on. The mock-cluster, restore and disk-snapshot tests run on every push again.
- **Why now:** main has been red for 22 runs, so a genuine regression in `scripts/` would go unread. Three test steps have not executed on CI since 2026-11-08. The cause is this workstream's own commits (w2/m289, m294, m295). AGENTS.md says t001 ships alone as `fix(ci): …` with `--fixes-red-main`, before anything else.
- **Render parity omitted:** CI and repository scripts only, with no REST/GraphQL/MCP/UI change.
- **Out of scope:**
  - `postgres operand image (build + verify)` fails on `ERROR: permission denied to set parameter "timescaledb.restoring"` (`deploy/pg-operand/check.sh:239`, run 42772567926). It is a deliberate strict check of a real gap owned by `w8/blocked/m92` t008. Do not weaken it here.
  - `common Bookworm native image` amd64 qualify failures on its first hosted run (42772567795). The causes are `Syft source differs from actual generated Docker config` (`native-common-qualify.py:1188`; `ubuntu-latest` is now 26.04 with Docker 29.4.2 and the containerd image store) and a Ghostscript `Permission denied` on `qa-gs.pdf`. These native inputs sit under w8/m91's D3 custody. Record them in the closeout note for w8 and do not edit them here.
  - `Watchdog` goes to the `null` receiver and no external dead-man switch exists (ADR010 § Alerting heartbeat). A dropped hosted heartbeat can leave a total alerting failure unseen for about 11h. Fixing that needs a user decision and a third-party credential. Raise it in the closeout; do not build it here.
