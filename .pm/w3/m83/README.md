# w3 · m83 — Production synthetics round 2: tenant-view canary, deploy canary, scheduled isolation matrix, and the missing alert rules

**Worker:** worker3 **Goal:** a silent regression in the tenant-facing pipelines — app logs, metrics, events, deploy-from-git, static-site serving and teardown, tenant/sandbox isolation — surfaces as a GitHub issue within six hours (or one week for the heavy probes) instead of at the next `/qa-find-bugs` run, and every metric series that already exists for webhooks, push, and agent sessions has an alert rule and a panel. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                   | est | depends_on             |
| ---- | ----------------------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Coverage table in ADR088 §6: every tenant-facing surface × {alert, scheduled probe, none} and the chosen form per gap  | 30m | —                      |
| t002 | App-log tenant liveness: the request-log probe also requires a `type=app` stream in a tenant namespace                  | 30m | t001                   |
| t003 | Canary workspace + always-on canary app on prod, scoped API key, six-hourly tenant-view probe (URL → logs → metrics → events) | 60m | t001                   |
| t004 | Weekly deploy canary: create from `examples/hello-go`, Ready, 200, delete, consistent absence; static-site variant     | 60m | t003                   |
| t005 | Weekly isolation matrix on the production runner: tenant-isolation + sandbox-isolation scripts, issue on red            | 45m | t001                   |
| t006 | Alert rules for series with no rule: webhook attempt failure ratio, push last-success staleness, agent-session provision | 40m | t001                   |
| t007 | Red-path proof: one deliberately broken run per new probe opens the issue; the next green run closes it                 | 30m | t002, t003, t004, t005 |
| t008 | Simplify                                                                                                                | 20m | t006, t007             |
| t009 | Test coverage                                                                                                           | 30m | t006, t007             |
| t010 | Closeout                                                                                                                | 10m | t009                   |

## Definition of done

The production liveness workflows run on schedule against `api.bex.co` / the production cluster with a recorded green run for each of: app-log tenant liveness (6h), the tenant-view canary (6h), the deploy canary (weekly), and the isolation matrix (weekly) — and a recorded red-path proof per probe (a deliberately broken run opened the tracking issue, the next green run closed it). The three new alert rules exist in `deploy/gitops/base/prometheus.yaml`, render on a dashboard, and `scripts/obs-coverage-check.sh` is green with no new waiver. ADR088 §6 carries the surface coverage table with no tenant-facing surface at "none" without an explicit, reasoned waiver. `scripts/github-actions-validate.sh` passes (credentialed jobs on `bex-production`, no GitHub-hosted labels).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-09-08 #2, verified the same day: only four probes run on a schedule today (`ssh-kexinit-probe`, `onbex-default-tls-verify`, `shell-ws-probe`, `request-logs-liveness` in `.github/workflows/ssh-edge-liveness.yml`); `request-logs-liveness.sh` asserts only `type=request`; no workflow references `verify-tenant-isolation.sh`, `verify-sandbox-isolation-live.sh`, `static-delete-timing-verify.sh`, or a hello-go deploy; the only webhook alert is `WebhookDeliveryAdmissionPressure`, and `bex_agent_session_provision_seconds` / the push `last_success` gauge have no rule at all. Lineage: `w6/m131` ("the guard that exists and does not run" — `logs-verify.sh` idle while request logs were dark for every tenant from the 2026-07-29 ADR043 migration until 2026-08-28) and `w6/m132` ("the SECOND such script found idle" — `ssh-verify.sh` while the SSH edge never sent KEXINIT); ADR088 §6's falsifiable-green rule; the owed live probes recorded on `w3/m46` t008 and `w3/m81` t004.
- **Goal linkage:** pillar 2 — agent-readable state has to be **true**; the Render DX promise (push becomes a running URL); ADR010's three pipelines (logs, metrics, events).
- **Expected outcome:** a regression of the m36 (app logs empty for all tenants), m110 (metrics silently missing), m131, or m132 class is a GitHub issue within one probe interval; isolation invariants are re-proven weekly on the real substrate; webhook/push/agent-session failures page the operator.
- **Why now:** the pattern has bitten twice in two weeks, the six-hourly workflow and its issue plumbing already exist, the `bex-production` runner pool already holds the deploy kubeconfig, and each new probe costs one job.
- **Render parity task omitted:** pure platform operations — scripts, workflows, alert rules, dashboards, docs; no REST/GraphQL/MCP/UI change.

## Notes / constraints

- `.pm/DO_NOT_DO.md` `#CI-RUNNERS` / `#RUNNER-HOSTS`: everything stays on the existing self-hosted pools; credentialed jobs use `bex-production` (validator check 6).
- New secret: one scoped API key for the canary workspace (`BEX_CANARY_API_KEY`), added to `.env.example` + `scripts/gh-secrets.sh` + the ADR019 credential inventory. The canary workspace is first-party and `billing_excluded` (ADR040 §7).
- The live sandbox-isolation matrix consumes a model key per run; keep it weekly, or gate its model-dependent checks behind a flag and run the rest.
