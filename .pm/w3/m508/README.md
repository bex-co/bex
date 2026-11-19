# w3 · m508 — A project's Service Metrics view charts every service together, as Render documents

**Worker:** worker3 **Goal:** the project's Service Metrics view (the `METRICS` saved search w3/m505 creates) charts CPU, memory, instances, HTTP requests, response time and bandwidth for every service of the project or environment, one series per service, grouped as Render's bundle allows. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Re-check Render's Service Metrics view and record what it shows | 30m | — |
| [t002](t002.md) | Metrics for a project scope with a group-by | 1h30m | t001, w3/m505/t003 |
| [t003](t003.md) | Dashboard: the Service Metrics view page | 2h | t002, w3/m505/t006 |
| [t004](t004.md) | Partial reads, resources without data, the fan-out bound and Hobby | 45m | t003 |
| [t005](t005.md) | Service Metrics in New View and Edit view | 30m | t004, w3/m507/t003 |
| [t006](t006.md) | Verify on dev-3 and on production | 30m | t005 |
| [t007](t007.md) | ADR010 and ADR018 | 20m | t006 |
| [t008](t008.md) | Render parity | 20m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 30m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

## Definition of done

- The project sidebar lists the Service Metrics view; its page charts CPU, memory, instances, HTTP requests, response time and bandwidth with one series per service of the project (or the filtered environment).
- Plan gates and retention apply per service; unreadable services are absent.
- Members can create and edit metrics views (w3/m507).
- Verified on dev-3 and production at 1440 and 390.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first: Render's docs describe the Service Metrics view, and its bundle and default saved searches carry it.
- **Recommendation, not a user decision:** in the reference workspace Render hid the view on 2026-11-19 (no sidebar entry, no New View option, the URL redirects). The recommendation is to build it from Render's docs, which describe it as available, under parity first; t001 re-checks first. Holding it until Render shows it everywhere is the alternative, and that is the user's call.
- **Expected outcome:** a project's resource use can be compared across its services on one page.
- **Why now:** w3/m505 creates the `METRICS` default view, and the backend already fans out over several resources, so the remaining work is mostly the page.
- **Render parity included:** GraphQL and dashboard changes.
