# w1 · m59 — deploy-run supersession hygiene — neutral conclusion + pre-build staleness skip

**Worker:** worker1 **Goal:** make a superseded `deploy (bex via Argo)` run conclude neutrally (not red) and skip the expensive image build, instead of failing at the write-back guard after building and signing throwaway images. **Status:** done (2026-07-30) — `check-supersession` job skips a queued-behind-newer run (neutral, no build); the write-back guard concludes neutral (green) on a mid-build supersession; detection factored into `scripts/deploy-superseded.sh` + guard test + actionlint (see Closeout)

## Tasks (in order)

| id   | title                                                                                                        | est  | depends_on   | status     |
| ---- | ------------------------------------------------------------------------------------------------------------ | ---- | ------------ | ---------- |
| t001 | Write-back guard: conclude neutral/skipped (not failure) with a "superseded by newer run" signal             | 45m  | —            | — **DONE** |
| t002 | Pre-build staleness check: skip the image build when a newer deploy-triggering commit exists on main         | 90m  | t001         | — **DONE** |
| t003 | Regression guard for the supersession detection + verify a superseded run concludes neutral and builds nothing | 45m  | t001, t002   | — **DONE** |
| t004 | Simplify (`/simplify` over the diff)                                                                         | 30m  | t003         | — **DONE** |
| t005 | Test coverage                                                                                                | 60m  | t004         | — **DONE** |
| t006 | Closeout                                                                                                     | 15m  | t005         | — **DONE** |

## Definition of done

Pushing a second `lego/`/`dashboard/`/`deploy/` change while an older deploy run is queued/running causes the older run to conclude `neutral` (shown as such in the Actions UI, not red) and to perform no image build/push when superseded pre-build; the newest run builds and rolls normally. A guard test pins the staleness/supersession detection.

## Closeout (2026-07-30)

The supersession refusal stops looking like a prod deploy failure and stops wasting a build.

- ✅ **Pre-build skip (no wasted build)**: a new `check-supersession` job runs `scripts/deploy-superseded.sh` and gates `build-and-deploy` (`if: needs.check-supersession.outputs.superseded != 'true'`). A run queued behind a newer deploy-triggering commit is **skipped** (neutral) before any `docker/build-push-action`. `cancel-in-progress: false` untouched.
- ✅ **Neutral, not red (mid-build race)**: the write-back guard now runs the same script; on supersession it sets `SUPERSEDED=true`, emits a `::notice::`, and `exit 0` (green). The four rollout steps gate on `env.SUPERSEDED != 'true'`, so nothing stale is pinned or rolled. An errored check still refuses (red) rather than risk a stale pin.
- ✅ **Detection is one testable place**: `scripts/deploy-superseded.sh` (exit 0 superseded / 1 current / 2 error) shared by both consumers; `scripts/deploy-superseded.test.sh` pins five cases (incl. generated-digest-only = NOT superseded); `github-actions-validate.sh` pins the deploy.yml wiring; `actionlint` validates the workflow expressions.

Accepted minor inefficiency: on a mid-build supersession the idempotent Argo-sync/secret steps still run (harmless reconciliation from `main`); only the red-risk / pod-churn rollout steps are gated. No tenant-facing REST/GraphQL/MCP/UI surface (CI/deploy-workflow internals).

## Source + Goal linkage

- **Source:** directly observed 2026-07-30 while shipping w3/m35 (`c87dafe3`): the in-flight deploy run for `2fc94bd7` **failed noisily** at `pin image digests in GitOps (write-back)` with *"production inputs advanced … refusing stale digest write-back"* (because the newer push touched `lego/`), then cascaded `failure` through `wait for the bex operator`/`roll bex` — after already building + signing three images that were discarded. The guard is correct (don't pin stale images); the *signal* and the wasted build are the problem.
- **Goal linkage:** platform reliability / CI signal quality — the w1/m28 ("gate deploys on real CI test runs") lineage.
- **Expected outcome:** a superseded deploy run concludes `neutral` with a clear "superseded by newer run" message and skips the image build; only the newest run's images are built and pinned.
- **Why now:** observed this session; with the current ship cadence (multiple `lego/`-touching ships per week) superseded runs are a routine source of red `deploy (bex via Argo)` entries that look like prod failures and waste build minutes.
- **Render parity:** omitted — this is CI/deploy-workflow internals with no tenant-facing REST/GraphQL/MCP/UI surface.
