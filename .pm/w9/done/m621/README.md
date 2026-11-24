# w9 · m621 — A Billing member sees no service events, metrics or deploy history, as on Render

**Worker:** worker9 **Goal:** the Billing role reads on bex what Render's table gives it: billing and non-sensitive resource details, no events, metrics, deploys or logs. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A relation for events, metrics and deploy history without the Billing role: model, `RelCan`, audit CHECK migration, capability grant, ADR012's matrix — **DONE** | 40m | — |
| [t002](done/t002.md) | Service and workspace events, deploy list and get, and every metric read on the new relation — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Dashboard: no Events, Deploys, Metrics or project Observability for a Billing member — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Amend ADR087, ADR024 and ADR018's members row; role tests; a live walk on dev-9 — **DONE** | 1h | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 45m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

## Definition of done

On dev-9, a Billing member is refused service and workspace events, deploy history, and every service, datastore and project metric on REST, GraphQL and MCP, and the dashboard shows none of them, with the reason. The service list and names still read. A Viewer still reads events and metrics and is still refused logs. ADR012's matrix, ADR024, ADR087 and ADR018's members row no longer record the Billing-reads divergence.

## Open decision (recorded as a recommendation, not a user decision)

Deploy history is restricted along with events. That is this milestone's recommendation: Render's table denies the Billing role "View service events", and a deploy's start and end are service events (`deploy_started` and `deploy_ended` in Render's event types). The alternative is to keep the deploy list readable for a Billing member. That would be a new divergence, needing the user's sign-off in ADR018. Recorded under Render parity first (ADR008), not as a user decision. **Shipped as recommended (2026-11-24),** with a cron job's run history beside the deploys: a cron run's start and end are Render's `cron_job_run_started`/`cron_job_run_ended` service events.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w9` (2026-11-23), item 3, verified before filing. Render: role table (render.com/docs/team-members, read 2026-11-23): Billing ❌ "View service events", ❌ "View service metrics", ❌ "View service logs". The Billing role's description grants only "non-sensitive details of the workspace's resources (such as members and service names)". bex: `deploy/gitops/authz/model.fga:84`, `lego/backend/internal/events/service.go:1144,1283`, `lego/backend/internal/deploys/service.go:483`, `lego/backend/internal/metrics/`.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). A divergence stands only with the user's sign-off in ADR018, and w6/m136's "Billing keeps supervision reads" has none.
- **Expected outcome:** the Billing role on bex is Render's billing-only role: billing plus resource names, no activity or performance data.
- **Why now:** the parity rule (2026-10-17 and 2026-10-20) says to amend an earlier design that diverges. Open signup (2026-11-21) makes Scale workspaces with Billing members real. Sequenced after w9/m620, which also adds a relation and a CHECK migration.
- **Render parity closing task:** included, because REST, GraphQL, MCP and dashboard reads change.
