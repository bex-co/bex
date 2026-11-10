# w6 · m309 — Pipeline-minute spend limit

**Worker:** worker6 **Goal:** a workspace can cap its monthly pipeline-minute spend, and builds stop at the cap until it is raised, as on Render. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record Render's spend-limit surface](t001.md) | 30m | — |
| t002 | [Store and edit a workspace's pipeline spend limit](t002.md) | 1.5h | t001 |
| t003 | [Builds stop when the spend limit is reached](t003.md) | 1h | t002 |
| t004 | [Render parity](t004.md) | 30m | t003 |
| t005 | [Simplify](t005.md) | 15m | t004 |
| t006 | [Test coverage](t006.md) | 30m | t005 |
| t007 | [Closeout](t007.md) | 10m | t006 |

## Definition of done

- t001's Render record is in this README.
- On dev-6, a workspace with a card and a spend limit stops building when the month's pipeline overage reaches the limit and builds again after the limit is raised.
- The limit is settable only by billing owners, on the surfaces t001 says Render offers.

## Source + Goal linkage

- **Source:** this session's 2026-11-10 discussion of the tenant build experience against Render, then `/pm-brainstorm for w6` (same session) and `/pmx them all to w6`; ADR018 § Gap backlog row "Pipeline-minute spend limit"; ADR030 §13 ("until the pipeline spend limit ships").
- **Goal linkage:** ADR008 Render parity first; Render lets a workspace set a monthly spend limit on pipeline minutes and stops builds at it ([build pipeline](https://render.com/docs/build-pipeline)).
- **Expected outcome:** a workspace with a card can bound its build spend, the way it can on Render.
- **Why now:** once the per-workspace cap goes (`m306`) and overage is automatic (`m308`), nothing bounds a card-holding workspace's build spend. Schedule after `m308`, whose exhaustion mechanism this reuses.
- **Render parity task:** included (billing settings and deploy failure are tenant-visible).
