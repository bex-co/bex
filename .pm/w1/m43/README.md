# w1 · m43 — Restore the `gitops (render)` CI guarantee

**Worker:** worker1 **Goal:** the `gitops (render)` check — the guarantee that the Argo-reconciled gitops tree always renders — is green on `main` again, by fixing the two confirmed pre-existing failures it has been silently red with. **Status:** todo

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | -------------------------------------------------------------------------- | --- | ---------- |
| t001 | Fix traefik 41.0.0 values schema drift (`logs` key) in the gitops values  | 30m | —          |
| t002 | Sync the stale `BackupCronJobStale` promtool test fixture                  | 20m | —          |
| t003 | Verify `gitops-validate.sh` clean locally and the check green on `main`    | 20m | t001, t002 |
| t004 | Simplify                                                                    | 15m | t003       |
| t005 | Test coverage                                                               | 15m | t003       |
| t006 | Closeout                                                                    | 15m | t005       |

## Definition of done

`bash scripts/gitops-validate.sh` exits 0 locally, and the `gitops (render)` check runs green on `main` (evidence: the check's run URL recorded at closeout). Both root causes — the traefik 41.0.0 chart schema rejecting the `logs` values key, and the `BackupCronJobStale` promtool fixture missing the `docs/ADR031-platform-data-backup.md` annotation reference — are fixed at the source, not skipped or suppressed.

## Source + Goal linkage

- **Source:** `w1/020` (filed from `w4/m11/t009`'s investigation, which confirmed both failures pre-date that work via a clean worktree); promoted by `/pm-brainstorm` round 17.
- **Goal linkage:** platform reliability — a red guarantee-check masks the next *real* gitops regression and trains everyone to ignore CI.
- **Expected outcome:** the "gitops tree always renders" guarantee is real again; future gitops regressions re-trip a check people trust.
- **Why now:** both failures are already root-caused (pure execution), and every push wave this loop generates lands on a `main` whose gitops check is red — the cheapest possible time to fix it. Sizing is honestly borderline (~1–1.5h); promoted per user approval of the round-17 proposal.
- **Render parity:** omitted — pure platform infra (`deploy/gitops/` values + Prometheus rule fixtures); no REST/GraphQL/MCP/UI surface changes.
