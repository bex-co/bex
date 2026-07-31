# w1 · m59 — deploy-run supersession hygiene — neutral conclusion + pre-build staleness skip

**Worker:** worker1 **Goal:** make a superseded `deploy (bex via Argo)` run conclude neutrally (not red) and skip the expensive image build, instead of failing at the write-back guard after building and signing throwaway images. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                        | est  | depends_on   |
| ---- | ------------------------------------------------------------------------------------------------------------ | ---- | ------------ |
| t001 | Write-back guard: conclude neutral/skipped (not failure) with a "superseded by newer run" signal             | 45m  | —            |
| t002 | Pre-build staleness check: skip the image build when a newer deploy-triggering commit exists on main         | 90m  | t001         |
| t003 | Regression guard for the supersession detection + verify a superseded run concludes neutral and builds nothing | 45m  | t001, t002   |
| t004 | Simplify (`/simplify` over the diff)                                                                         | 30m  | t003         |
| t005 | Test coverage                                                                                                | 60m  | t004         |
| t006 | Closeout                                                                                                     | 15m  | t005         |

## Definition of done

Pushing a second `lego/`/`dashboard/`/`deploy/` change while an older deploy run is queued/running causes the older run to conclude `neutral` (shown as such in the Actions UI, not red) and to perform no image build/push when superseded pre-build; the newest run builds and rolls normally. A guard test pins the staleness/supersession detection.

## Source + Goal linkage

- **Source:** directly observed 2026-07-30 while shipping w3/m35 (`c87dafe3`): the in-flight deploy run for `2fc94bd7` **failed noisily** at `pin image digests in GitOps (write-back)` with *"production inputs advanced … refusing stale digest write-back"* (because the newer push touched `lego/`), then cascaded `failure` through `wait for the bex operator`/`roll bex` — after already building + signing three images that were discarded. The guard is correct (don't pin stale images); the *signal* and the wasted build are the problem.
- **Goal linkage:** platform reliability / CI signal quality — the w1/m28 ("gate deploys on real CI test runs") lineage.
- **Expected outcome:** a superseded deploy run concludes `neutral` with a clear "superseded by newer run" message and skips the image build; only the newest run's images are built and pinned.
- **Why now:** observed this session; with the current ship cadence (multiple `lego/`-touching ships per week) superseded runs are a routine source of red `deploy (bex via Argo)` entries that look like prod failures and waste build minutes.
- **Render parity:** omitted — this is CI/deploy-workflow internals with no tenant-facing REST/GraphQL/MCP/UI surface.
