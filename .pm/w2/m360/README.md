# w2 · m360 — Cron jobs get Render's Builds and Runs pages

**Worker:** worker2 **Goal:** a cron job's dashboard is laid out like Render's. Its sidebar has a **Builds** page for build history and a **Runs** page for run history, each run's logs, Trigger Run and cancel. The Events page shows events only, and Render-style links to a cron job's runs or builds open the matching page. **Status:** in progress (t001–t004 and t006–t008 done; t005, the dev-2 pending/ready comparison, waits for the local cluster; then t009)

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | Capture Render's cron dashboard: sidebar order, Builds and Runs columns, per-run logs, Trigger Run and cancel placement — **DONE** | 30m | — |
| t002 | A Runs page at `/services/$serviceId/runs`: the runs list leaves Events, with each run's logs and Trigger Run in the page header — **DONE** | 75m | t001 |
| t003 | A Builds page at `/services/$serviceId/builds` for cron jobs, replacing their Deploys entry; `/deploys` redirects there for a cron job — **DONE** | 45m | t002 |
| t004 | Render aliases and links: `/cron/srv-…/runs` and `…/builds` resolve; emails and notifications about a run link to Runs — **DONE** | 30m | t003 |
| t005 | Skeletons that match the loaded Runs and Builds pages at desktop and narrow-mobile widths; en/zh strings | 30m | t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t007 |
| t009 | Closeout | 10m | t008 |

## Definition of done

- A cron job's sidebar shows Builds and Runs in Render's order, and no Deploys entry. Other service types are unchanged.
- The Runs page lists runs (status, start, duration, trigger) with cursor paging. Opening a run shows that run's logs. Trigger Run and cancel behave as they do today (confirmations, permission gating, the cancel-then-replace rule).
- The Builds page lists the cron job's builds and opens a build's detail. A cron job's old `/deploys` URL redirects to it.
- The Events page of a cron job no longer carries the runs section.
- Render-style deep links to a cron job's runs and builds land on the matching bex page.
- On a dev-N stack at desktop and narrow-mobile widths, the pending and ready states of both pages match side by side. `yarn lint` and `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-12, item 4, filed by `/pmx them all for w2`.
- **Render:**
  - Changelog 2026-09-03: a new Deploys page, and cron jobs get separate Builds and Runs pages.
  - [render.com/docs/cronjobs](https://render.com/docs/cronjobs) (fetched 2026-11-12): "After creating your cron job, you can track build progress and history on the **Builds** page." "Your cron job's **Runs** page shows run history and logs for each run." To run outside the schedule, "go to its **Runs** page … and click **Trigger Run**". A manual trigger during an active run "first _cancels_ the active run".
- **bex today:**
  - `dashboard/src/features/services/components/service-nav.tsx:103` gives every non-static type, cron included, Deploys and Settings at the top level.
  - Cron runs are `CronRunsSection` (`dashboard/src/features/services/components/cron-runs-section.tsx`) at the bottom of the Events page (`dashboard/src/routes/services.$serviceId.events.tsx:410-412`).
  - There is no `runs` or `builds` route under `dashboard/src/routes/`. `dashboard/src/common/lib/render-alias.ts` maps `cron` only to the service list and create paths.
  - The REST, GraphQL and MCP run APIs already exist (ADR018 cron row), so this milestone is dashboard-only.
- **Goal linkage:** ADR008 Render parity first, on the dashboard surface.
- **Expected outcome:** a Render user finds a cron job's runs and builds where Render puts them, and links copied from Render's layout work.
- **Why now:** this is the one structural difference left on the cron dashboard, and nothing on the board owns it. Everything it needs already exists, so the risk is low.
- **Render parity included:** dashboard UI. REST, GraphQL and MCP do not change. t006 confirms they still feed both pages.
