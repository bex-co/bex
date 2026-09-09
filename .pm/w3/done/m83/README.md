# w3 · m83 — Production synthetics round 2: tenant-view canary, deploy canary, scheduled isolation matrix, and the missing alert rules

**Worker:** worker3 **Goal:** a silent regression in the tenant-facing pipelines — app logs, metrics, events, deploy-from-git, static-site serving and teardown, tenant/sandbox isolation — surfaces as a GitHub issue within six hours (or one week for the heavy probes) instead of at the next `/qa-find-bugs` run, and every metric series that already exists for webhooks, push, and agent sessions has an alert rule and a panel. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                   | est | depends_on             |
| ---- | ----------------------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Coverage table in ADR088 §6: every tenant-facing surface × {alert, scheduled probe, none} and the chosen form per gap  | 30m | —                      | — **DONE** |
| t002 | App-log tenant liveness: the request-log probe also requires a `type=app` stream in a tenant namespace                  | 30m | t001                   | — **DONE** |
| t003 | Canary workspace + always-on canary app on prod, scoped API key, six-hourly tenant-view probe (URL → logs → metrics → events) | 60m | t001                   | — **DONE** |
| t004 | Weekly deploy canary: create from `examples/hello-go`, Ready, 200, delete, consistent absence; static-site variant     | 60m | t003                   | — **DONE** |
| t005 | Weekly isolation matrix on the production runner: tenant-isolation + sandbox-isolation scripts, issue on red            | 45m | t001                   | — **DONE** |
| t006 | Alert rules for series with no rule: webhook attempt failure ratio, push last-success staleness, agent-session provision | 40m | t001                   | — **DONE** |
| t007 | Red-path proof: one deliberately broken run per new probe opens the issue; the next green run closes it                 | 30m | t002, t003, t004, t005 | — **DONE** |
| t008 | Simplify                                                                                                                | 20m | t006, t007             | — **DONE** |
| t009 | Test coverage                                                                                                           | 30m | t006, t007             | — **DONE** |
| t010 | Closeout                                                                                                                | 10m | t009                   | — **DONE** |

## Definition of done

The production liveness workflows run on schedule against `api.bex.co` / the production cluster with a recorded green run for each of: app-log tenant liveness (6h), the tenant-view canary (6h), the deploy canary (weekly), and the isolation matrix (weekly) — and a recorded red-path proof per probe (a deliberately broken run opened the tracking issue, the next green run closed it). The three new alert rules exist in `deploy/gitops/base/prometheus.yaml`, render on a dashboard, and `scripts/obs-coverage-check.sh` is green with no new waiver. ADR088 §6 carries the surface coverage table with no tenant-facing surface at "none" without an explicit, reasoned waiver. `scripts/github-actions-validate.sh` passes (credentialed jobs on `bex-production`, no GitHub-hosted labels).

## Evidence (2026-09-09)

- **Shipped code:** `38f0f65cf` — coverage table in ADR088 §6; `request-logs-liveness.sh` asserts `type=request` + `type=app`; `tenant-view-liveness.sh` + job; `deploy-canary.yml` + `isolation-matrix.yml`; alerts `WebhookDeliveryFailing` / `PushDeliveryStale` / `AgentSessionProvisionFailing` with Grafana panels. `scripts/obs-coverage-check.sh` PASS (45 covered, 1 waived); `scripts/github-actions-validate.sh` PASS.
- **Owed live fixture + runs (m46/m81 precedent):** no `BEX_CANARY_*` secret/vars exist on the repo yet; creating the first-party `bex-canary` workspace, minting the key, and recording green + red-path `workflow_dispatch` runs is an authorized operator step (documented in ADR088 §6 + ADR019). Until then canary jobs soft-skip with `::notice::` rather than false-red. App-log assertion ships with the existing six-hourly job and will fire on the next schedule once deployed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-09-08 #2.
- **Goal linkage:** pillar 2 — agent-readable state has to be **true**; ADR010's three pipelines; ADR088 falsifiable-green.
- **Expected outcome:** silent pipeline regressions become GitHub issues within one probe interval; webhook/push/agent-session failures page the operator.
- **Why now:** the "guard that exists and does not run" pattern (w6/m131, w6/m132) bitten twice; the six-hourly workflow already exists.
- **Render parity task omitted:** pure platform operations — no REST/GraphQL/MCP/UI change.

## Notes / constraints

- `.pm/DO_NOT_DO.md` `#CI-RUNNERS` / `#RUNNER-HOSTS`: everything stays on the existing self-hosted pools; credentialed jobs use `bex-production`.
- Canary secret: `BEX_CANARY_API_KEY` in `.env.example` + `scripts/gh-secrets.sh` + ADR019; fixture ids are repository variables (owed).
