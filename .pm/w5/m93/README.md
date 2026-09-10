# w5 · m93 — CLI adoption and reliability dashboard at obs.bex.co

**Worker:** worker5 **Goal:** Make observed CLI adoption, repeat usage, command reliability and automation usage visible in the existing ops Grafana dashboard. **Status:** in progress (t001–t006 done; ship/live verification and closeout pending)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Provision a restricted CLI analytics datasource — **DONE** | 45m | — |
| t002 | Build adoption and repeat-usage panels — **DONE** | 45m | t001 |
| t003 | Build command, reliability, duration and audience panels — **DONE** | 50m | t002 |
| t004 | Document metric definitions and coverage boundaries — **DONE** | 25m | t003 |
| t005 | Simplify — **DONE** | 30m | t004 |
| t006 | Test coverage — **DONE** | 40m | t005 |
| t007 | Ship and verify the live obs dashboard | 30m | t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- The GitOps-provisioned CLI usage board at obs.bex.co shows adoption, repeat usage, command popularity, outcomes, command duration, detected agents/CI, client environment and telemetry health.
- A dedicated read-only datasource reads only CLI analytics data with bounded database resources; SQL is verified against real Postgres fixtures.
- Labels and no-data states reflect consent/authentication/delivery gaps, retained-history boundaries, cohort maturity and upstream version identity.
- Changes are shipped to origin/main; the live board and datasource queries are verified in the browser, with desktop/narrow screenshots and evidence recorded here.

## Source + Goal linkage

- **Source:** User-approved dashboard discussion and explicit implementation/ship/show request on 2026-09-10; supersedes the deleted inbox note `0001-deploy-blocked-telemetry-rollout.md`.
- **Goal linkage:** ADR008 AI-native developer experience and ADR088 ops visibility: prioritize widely used commands and identify friction for people and agents.
- **Expected outcome:** Operators can measure observed adoption and repeat usage, find commands affecting the most installations, and distinguish user-command failures from telemetry delivery failures.
- **Why now:** m92 already stores the necessary event fields, but its six-panel Prometheus board lacks distinct counts, duration and agent/CI analytics. The recorded deployment gate failures are historical: deploy run 34444812690 (46e16ec3) succeeded.
- **Sizing:** Multiple hours of datasource provisioning, analytical SQL, UI composition, behavioral verification and live rollout.
- **Render parity omitted:** This milestone changes only the internal ops Grafana surface and its read-only database access. It adds no tenant REST/GraphQL/MCP/dashboard or CLI contract.
- **Coverage boundary:** Bex release-version instrumentation and successful exec-based provider-launch telemetry require separate sender work; this board labels the existing upstream version and documents these gaps instead of claiming those measurements.

## Recovered handoff

m92 landed in 9504b0de1 (endpoint/store/consent), b9797df53 (deletion inventory), and bf1e22369 (counter/initial board). POST /v1/cli-telemetry-events attributes subject/workspace server-side, stores 18 upstream fields, and increments a bounded-dimension Prometheus counter after insert. Raw events follow audit retention and account deletion removes the reporter's rows. The installed 0.2.1 release preceded the telemetry consent flip; source or a later telemetry-enabled binary is needed for an emission check. Prior local proof covered success, execution failure, opt-out and real Postgres. The old note's expired CLI login is not proof of current credentials; verify without exposing them. Root version requests, logged-out/early setup failures and successful exec-based provider launchers have collection gaps.

## Verification

- Ten real-Postgres tests pass, including every committed SQL query, fixture arithmetic, cohort maturity and reader permission denials. The suite uses a unique database and cluster-wide role per run.
- Three simplify reviews completed: retained-history cohort calculation now joins distinct weekly activity once; SQL test roles are isolated across concurrent runs. Reuse review found no further changes.
- Base and prod Grafana Helm rendering and the full GitOps validation pass, including the datasource contract guard and sealed credential manifest. Local promtool is unavailable (existing alert rules unchanged); the GitOps CI workflow installs it.
- Production restricted view/role provisioned; can read the view and cannot read the raw telemetry table. A real authenticated source-built `bex workspaces -o json` invocation reached the collector at 2026-09-10 23:52:56 UTC. This first direct Go build reported upstream version `dev`; subsequent verification uses the pinned 2.27.0 build flag.
- Live rendering and final ship pending.
