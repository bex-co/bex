# w5 · m29 — Deploy detail page: build log + status timeline

**Worker:** worker5 **Goal:** Render's per-deploy page exists in bex: clicking a deploy opens `/services/$serviceId/deploys/$deployId` with status/trigger/commit header, a status timeline, and (store-backed) the build log. **Status:** todo

## Tasks (in order)

| id   | title                                                    | est | depends_on |
| ---- | -------------------------------------------------------- | --- | ---------- |
| t001 | Deploy detail route + header (status · trigger · commit) | 45m | —          |
| t002 | GraphQL wiring: per-deploy query + hooks                 | 40m | t001       |
| t003 | Status timeline (deploy row + service events)            | 45m | t002       |
| t004 | Build-log pane over `type=build` (store-absent state)    | 45m | t003       |
| t005 | List → detail links; Cancel/Rollback on the detail page  | 30m | t003       |
| t006 | Render parity                                            | 30m | t004, t005 |
| t007 | Simplify                                                 | 30m | t006       |
| t008 | Test coverage                                            | 45m | t006       |
| t009 | Closeout                                                 | 15m | t008       |

## Definition of done

A deploy row click opens a detail page showing the deploy's status, trigger, and commit; a timeline built from the deploy row + service events; for a git-sourced deploy with the log store, the build log renders (with an explanatory state when the store or w7/m28 is absent); Cancel/Rollback work from the page. Canceled/failed deploys show their terminal state honestly.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker to work on until feature parity` 2026-07-14 (item 2); w5/m21 shipped the deploys *list* only — Render's per-deploy page has no bex equivalent.
- **Goal linkage:** Render dashboard parity; the UI where w2/m30–m32's deploy-object deepening (body fields, pagination, 11 statuses) becomes visible.
- **Expected outcome:** users watch a deploy progress (and debug a failed build) without leaving the dashboard.
- **Why now:** sequencing with w2/m30–m32 + w7/m28 means one UI pass instead of two. **Cross-milestone dependency:** t004's log pane consumes `w7/m28`'s `type=build`; the page and timeline land first with the pane's absent-state if m28 hasn't shipped. Render parity task included — UI surface.
