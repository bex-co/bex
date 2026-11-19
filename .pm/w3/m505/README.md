# w3 · m505 — A project's Application Logs and Request Logs show every resource in one view, as on Render

**Worker:** worker3 **Goal:** a project gets Render's Observability group: Application Logs and Request Logs views, each a saved search, that read every service, cron job, Postgres and Key Value of the project (or one environment) in one time-ordered stream, each line naming its resource. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | ADR010 + ADR032: project views are saved searches; scope resolution, filter document and caps | 45m | — |
| [t002](t002.md) | Saved-search store with three default views per project | 1h30m | t001 |
| [t003](t003.md) | GraphQL `savedSearches(projectId, type)` and `savedSearch(id)` with the project's view permission | 1h | t002 |
| [t004](t004.md) | GraphQL `logs` takes a project scope and a filter document | 2h | t003 |
| [t005](t005.md) | Live tail for a project scope | 1h | t004 |
| [t006](t006.md) | Dashboard: the project Observability group and its log view pages | 2h30m | t004 |
| [t007](t007.md) | Request Logs Pro gate and the partial, unavailable and empty states | 45m | t006 |
| [t008](t008.md) | Verify on dev-3 and on production | 45m | t005, t007 |
| [t009](t009.md) | ADR018 Projects row, ADR010 and the dashboard guide | 30m | t008 |
| [t010](t010.md) | Render parity | 20m | t009 |
| [t011](t011.md) | Simplify | 15m | t010 |
| [t012](t012.md) | Test coverage | 30m | t010 |
| [t013](t013.md) | Closeout | 10m | t011, t012 |

## Definition of done

- Every project has Render's three default saved searches (Application Logs, Request Logs, Service Metrics), readable through GraphQL `savedSearches`/`savedSearch`.
- On dev-3 and on production, `/project/<prj>/observability/logs/application?savedSearchId=<svs>` shows the project's web services', cron jobs', Postgres' and Key Value's lines interleaved in time, each with its resource name and instance, with an environment filter narrowing it, live tail included.
- Request Logs shows the project's web services' request lines; a Hobby workspace gets the Pro refusal.
- A resource the caller cannot read contributes no line, label or name.
- Pending and ready states match at 1440 and 390.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first. [render.com/docs/projects](https://render.com/docs/projects) § Observability: "You can view logs, metrics, and task runs for all services in a project or environment"; the default views are Application Logs and Request Logs ("Available if you have a Pro workspace or higher"). bex has per-service logs only: the project sidebar has Overview and Settings, and GraphQL `logs` takes one resource.
- **Recommendation, not a user decision:** the capture shows Render's own model, so the tasks follow it: every view is a saved search (`svs-`), defaults included. Leaving out the Region picker (bex hosts one region) is recorded in ADR018 as an omission, not a behavior difference.
- **Expected outcome:** a project's whole stack can be read in one stream, as on Render, instead of one service page at a time.
- **Why now:** Render ships this in every workspace and bex's ledger did not record the gap. The saved-search store is the base w3/m507 (view management) and w3/m508 (Service Metrics) build on. Sequenced beside w5/m503 (slow `/v1/logs` reads): t004 reuses the time-boxed paging so a project query cannot outlast it.
- **Render parity included:** new GraphQL reads and dashboard pages; REST and MCP are unchanged because Render has no public operation for them.
