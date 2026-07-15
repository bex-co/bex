# w5 · m29 — Deploy detail page: build log + status timeline

**Worker:** worker5 **Goal:** Render's per-deploy page exists in bex: clicking a deploy opens `/services/$serviceId/deploys/$deployId` with status/trigger/commit header, a status timeline, and (store-backed) the build log. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Deploy detail route + header (status · trigger · commit) — **DONE** | 45m | — |
| t002 | GraphQL wiring: per-deploy query + hooks — **DONE** | 40m | t001 |
| t003 | Status timeline (deploy row + service events) — **DONE** | 45m | t002 |
| t004 | Build-log pane over `type=build` (store-absent state) — **DONE** | 45m | t003 |
| t005 | List → detail links; Cancel/Rollback on the detail page — **DONE** | 30m | t003 |
| t006 | Render parity — **DONE** | 30m | t004, t005 |
| t007 | Simplify — **DONE** | 30m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006 |
| t009 | Closeout — **DONE** | 15m | t008 |

> **Verification (2026-07-14):** dashboard `yarn typecheck`, `yarn lint`, all 1,045 `yarn test` tests, and `yarn build` pass; `git diff --check` passes. A real image-backed service and store-backed `dep-…` row were created against the CAPD mock cluster, reached `Running`/`live`, and returned HTTP 200. A headless Chrome click-through from its Events row opened the exact deploy-detail URL and verified the Live header, factual status timeline, and rollback action (`.playwright-mcp/m29-deploy-detail-real.png`). Build/pre-deploy/app log merging, durable-store fallback, polling, terminal statuses, not-found behavior, exact `details.deployId` links, and shared actions are covered by focused component/hook tests. Codex has no `/simplify` skill installed, so t007 used the equivalent behavior-preserving diff review: the newer upstream commit-metadata and log-viewer work was preserved, event links use deploy ids instead of event ids, actions are shared, and the timeline renders row facts without inventing phases. The remaining deeper stored lifecycle was filed as `w2/m38`.

## Definition of done

A deploy row click opens a detail page showing the deploy's status, trigger, and resolved commit when available; a timeline built from the deploy row + service events; and deploy-window build/pre-deploy/app logs (with an explanatory state when build history needs the durable store). Cancel/Rollback work from the page. Canceled/failed deploys show their terminal state honestly.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker to work on until feature parity` 2026-07-14 (item 2); w5/m21 shipped the deploys _list_ only — Render's per-deploy page has no bex equivalent.
- **Goal linkage:** Render dashboard parity; the UI where w2/m30–m31's deploy-body/pagination work, w9/001's commit metadata, and the filed `w2/m38` lifecycle deepening become visible.
- **Expected outcome:** users watch a deploy progress (and debug a failed build) without leaving the dashboard.
- **Why now:** sequencing with w2/m30–m31 + w7/m28 means one UI pass instead of two. **Cross-milestone dependency:** t004's log pane consumes `w7/m28`'s `type=build`; the page and timeline land first with the pane's absent-state if m28 hasn't shipped. Render parity task included — UI surface.
