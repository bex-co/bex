# w6 · m309 — Pipeline-minute spend limit

**Worker:** worker6 **Goal:** a workspace can cap its monthly pipeline-minute spend, and builds stop at the cap until it is raised, as on Render. **Status:** in progress (t001–t003, t005, t006 done; t004 checks the live behavior after the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record Render's spend-limit surface](done/t001.md) — **DONE** | 30m | — |
| t002 | [Store and edit a workspace's pipeline spend limit](done/t002.md) — **DONE** | 1.5h | t001 |
| t003 | [Builds stop when the spend limit is reached](done/t003.md) — **DONE** | 1h | t002 |
| t004 | [Render parity](t004.md) | 30m | t003 |
| t005 | [Simplify](done/t005.md) — **DONE** | 15m | t004 |
| t006 | [Test coverage](done/t006.md) — **DONE** | 30m | t005 |
| t007 | [Closeout](t007.md) | 10m | t006 |

## Definition of done

- t001's Render record is in this README.
- On dev-6, a workspace with a card and a spend limit stops building when the month's pipeline overage reaches the limit and builds again after the limit is raised.
- The limit is settable only by billing owners, on the surfaces t001 says Render offers.

## Render record (t001, 2026-11-10)

Read from [render.com/docs/build-pipeline](https://render.com/docs/build-pipeline) on 2026-11-10, and from the pinned public OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`, 252 operations):

- **Where:** "In the Render Dashboard, go to your Workspace Settings page. In the Build Pipeline section, click Set spend limit", then enter the limit in the dialog. The public API and the CLI have no spend-limit or pipeline operation, so the setting is dashboard-only.
- **Unit and default:** a monthly amount of money. No default is documented: without one, overage is bought without bound.
- **Scope:** "the monthly spend limit" on pipeline minutes, workspace-wide, for the workspace's current pipeline tier.
- **Reached:** Render stops buying supplementary minutes, and "stops running pipeline tasks (including service builds!) for the remainder of the current month". This is the same stop as running out without a payment method; the troubleshooting docs call it "Build Minutes Exhausted".
- **Lift:** "You can reenable pipeline tasks by raising your spend limit (and adding a payment method if you haven't)."

So bex offers the limit on GraphQL and the dashboard (Workspace Settings → Build Pipeline), not on REST or MCP, which mirror Render's public API and MCP tools.

## Source + Goal linkage

- **Source:** this session's 2026-11-10 discussion of the tenant build experience against Render, then `/pm-brainstorm for w6` (same session) and `/pmx them all to w6`; ADR018 § Gap backlog row "Pipeline-minute spend limit"; ADR030 §13 ("until the pipeline spend limit ships").
- **Goal linkage:** ADR008 Render parity first; Render lets a workspace set a monthly spend limit on pipeline minutes and stops builds at it ([build pipeline](https://render.com/docs/build-pipeline)).
- **Expected outcome:** a workspace with a card can bound its build spend, the way it can on Render.
- **Why now:** once the per-workspace cap goes (`m306`) and overage is automatic (`m308`), nothing bounds a card-holding workspace's build spend. Schedule after `m308`, whose exhaustion mechanism this reuses.
- **Render parity task:** included (billing settings and deploy failure are tenant-visible).
