# w1 · m43 — Restore the `gitops (render)` CI guarantee

**Worker:** worker1 **Goal:** the `gitops (render)` check — the guarantee that the Argo-reconciled gitops tree always renders — is green on `main` again, by fixing the two confirmed pre-existing failures it has been silently red with. **Status:** done (2026-07-15)

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | -------------------------------------------------------------------------- | --- | ---------- |
| t001 | Fix traefik 41.0.0 values schema drift (`logs` key) in the gitops values — **DONE** | 30m | —          |
| t002 | Sync the stale `BackupCronJobStale` promtool test fixture — **DONE**        | 20m | —          |
| t003 | Verify `gitops-validate.sh` clean locally and the check green on `main` — **DONE** | 20m | t001, t002 |
| t004 | Simplify — **DONE**                                                         | 15m | t003       |
| t005 | Test coverage — **DONE**                                                    | 15m | t003       |
| t006 | Closeout — **DONE**                                                         | 15m | t005       |

## Definition of done

`bash scripts/gitops-validate.sh` exits 0 locally, and the `gitops (render)` check runs green on `main` (evidence: the check's run URL recorded at closeout). Both root causes — the traefik 41.0.0 chart schema rejecting the `logs` values key, and the `BackupCronJobStale` promtool fixture missing the `docs/ADR031-platform-data-backup.md` annotation reference — are fixed at the source, not skipped or suppressed.

## Closeout evidence

- This milestone was promoted from a stale inbox note after implementation had already landed in `67da0499` (`fix(ci): restore dashboard and gitops validation`). That commit migrated Traefik's removed `logs.access` values to chart 41.0.0's `accessLog` schema and synchronized the alert fixture's ADR031 runbook reference.
- Traefik 41.0.0's packaged values/schema confirms `accessLog` is the supported key. Both base and production-layer Helm renders pass.
- `bash scripts/gitops-validate.sh` passed on 2026-07-15 with the workflow-pinned Prometheus 2.55.1 `promtool`: 15 rules checked and all rule tests passed. Reading the script confirms it templates the pinned Traefik chart and extracts/checks/tests the Prometheus rule pack, so both regressions are guarded.
- The implementation commit's [`gitops (render)` run](https://github.com/bex-co/bex/actions/runs/29402875626) succeeded on `main`; later relevant main runs also remain green, including [run 29466033499](https://github.com/bex-co/bex/actions/runs/29466033499).
- Simplification found no remaining duplicate compatibility shim: the values use the chart-native schema and the fixture matches the rule source verbatim.

## Source + Goal linkage

- **Source:** `w1/020` (filed from `w4/m11/t009`'s investigation, which confirmed both failures pre-date that work via a clean worktree); promoted by `/pm-brainstorm` round 17.
- **Goal linkage:** platform reliability — a red guarantee-check masks the next *real* gitops regression and trains everyone to ignore CI.
- **Expected outcome:** the "gitops tree always renders" guarantee is real again; future gitops regressions re-trip a check people trust.
- **Why now:** both failures are already root-caused (pure execution), and every push wave this loop generates lands on a `main` whose gitops check is red — the cheapest possible time to fix it. Sizing is honestly borderline (~1–1.5h); promoted per user approval of the round-17 proposal.
- **Render parity:** omitted — pure platform infra (`deploy/gitops/` values + Prometheus rule fixtures); no REST/GraphQL/MCP/UI surface changes.
