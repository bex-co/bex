# w3 · m177 — Move the Prometheus rule pack out of the inline Argo Application values

**Worker:** worker3 **Goal:** the platform alert rules live in a reviewed file mounted into Prometheus from a ConfigMap, so the Prometheus Argo Application regains ample size headroom and the next alert or scrape job no longer fails `gitops (render)` **Status:** blocked — t001–t003, t005, t006 done (shipped); t004 production rule-load check waits on the rollout; t007 closeout follows

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Move the rule pack into `deploy/gitops/base/rules/alerting_rules.yml`, mount it from a ConfigMap, and let the reloader watch it — **DONE** | 1h | — |
| t002 | Repoint the rule tooling (`gitops-validate.sh`, `obs-coverage-check.sh`, `test_platform_metrics.py`, `alerts-verify.sh`) at the file — **DONE** | 45m | t001 |
| t003 | Prove the rendered rule set is unchanged and the size budget has headroom — **DONE** | 20m | t002 |
| t004 | Production check: Prometheus loads the same rule groups after the rollout | 15m | t003 |
| t005 | Simplify — **DONE** | 15m | t003 |
| t006 | Test coverage — **DONE** | 20m | t003 |
| t007 | Closeout | 10m | t004, t005, t006 |

## Definition of done

- `scripts/test_platform_metrics.py`'s size check passes in both overlays with at least 64 KiB headroom.
- The rule groups Prometheus loads are identical before and after the move (same group names, rule names and expressions; `promtool check rules` count unchanged), and every `deploy/gitops/base/rules/*_test.yml` passes against the moved file.
- `scripts/gitops-validate.sh`, `scripts/obs-coverage-check.sh` and `scripts/alerts-verify.sh` read the file, and fail if it is missing or not mounted.
- After the production rollout, Prometheus' `/api/v1/rules` lists the same groups and the Watchdog alert is firing.

## Source + Goal linkage

- **Source:** promoted from `w3/045` (filed by the loopx w3 run, 2026-10-21) when it turned out to exceed an hour: four tools read the inline pack. Found by `w3/m174` and `w3/m175`, which left the local Prometheus Application 70 bytes under its 1,310,720-byte modelled budget (`scripts/test_platform_metrics.py` `verify_application_size`).
- **Goal linkage:** V0 roadmap 2 (basic observability for operation): alert coverage must be able to grow.
- **Expected outcome:** adding an alert costs ConfigMap bytes, not ~8× its text in the Argo Application's retained history.
- **Why now:** the next alert anyone adds fails CI; trimming comments to fit was already how m174 got in. `revisionHistoryLimit` was already cut from 10 to 3 after an etcd request-limit incident, and the three local rollback entries are a deliberate choice, so lowering it further is not the fix.
- **Render parity omitted:** platform monitoring configuration only; no REST/GraphQL/MCP/UI surface.
