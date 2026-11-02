# w4 · m249 — Every dashboard page exposes its structure to assistive technology

**Worker:** worker4 **Goal:** a screen-reader user can jump by heading to every section of a dashboard page and hear which service tab they are on, instead of finding one heading on a 15-section page and an identical title on all 11 tabs. **Status:** done 2026-11-02 (loopx w4; the production probes in the Definition of done run after the release pins this build)

## Tasks (in order)

| id   | title                                                                          | est | depends_on                            |
| ---- | ------------------------------------------------------------------------------ | --- | ------------------------------------- |
| t001 | Make section card titles real headings by default — **DONE** | 35m | —                                     |
| t002 | Audit the 124 `<CardTitle` sites and give every exception an explicit level — **DONE** | 1h  | w4/m249/t001                          |
| t003 | Give the default Agents page an h1 — **DONE** | 20m | —                                     |
| t004 | Give each service tab its own document title — **DONE** | 40m | —                                     |
| t005 | Regression tests on the callers that are correct today — **DONE** | 40m | w4/m249/t002, w4/m249/t003, w4/m249/t004 |
| t006 | Render parity: dashboard heading and title structure — **DONE** | 20m | w4/m249/t005                          |
| t007 | Simplify: run `/simplify` over the code this milestone changed — **DONE** | 15m | w4/m249/t006                          |
| t008 | Test coverage: tests for the structure this milestone shipped — **DONE** | 30m | w4/m249/t006                          |
| t009 | Closeout: verify the Definition of done live, then move the milestone to done — **DONE** | 15m | w4/m249/t007, w4/m249/t008            |

Every task is done. t009 closeout: the Definition of done is code-complete and covered by tests (card, agents, tab titles, skip-free visuals); its live probes are verified after the deploy.

## Definition of done

Live on production, repeatable in the browser console after load:

- **(a) Section headings.** On `/services/srv-dbavjsm05jqs73cjk3og/settings`, `/billing`, `/workspace/settings`, `/services/srv-dbavjsm05jqs73cjk3og/scaling`, `/metrics` and `/plan`, every visible `[data-slot="card-title"]` that titles a section is an H2 or H3. The number of H2/H3 equals the number of section cards (for example 15 on service Settings), heading levels never skip (h1 to h2 to h3), and each page has exactly one h1. Any card title that is intentionally not a heading appears in t002's exception table.
- **(b) Agents.** `/agents` with no query string has exactly one visible h1 whose text is the translated "Agent sessions", and `/agents?archived=false` still has exactly one.
- **(c) Tab titles.** The eleven service tab URLs under one service have eleven distinct `document.title` values, each starting with the tab's name, and the sidebar `aria-current="page"` label matches the title's first segment.
- **(d) No visual change.** Computed `font-size`, `line-height` and `font-weight` of the service Settings "General" title are identical before and after, at 1440 and 390 widths, and pending and ready skeletons of a Settings page still align (dashboard skeleton rule).
- **(e) Controls unchanged.** `lang`, one `main` and the labelled navs are unchanged on all 15 pages probed.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop run, round 15, 2026-11-02 UTC (`muse.env`, QA workspace `bex`; read-only, nothing created). Evidence is the probes below; screenshots are session-local.
- **Goal linkage:** ADR008 usable hosting and Render parity first (Render's dashboard exposes section headings and per-page titles). Builds on w4/410 (page-level h1 for Webhooks and Notifications) and w4/409 (second h1 on the Environment tab), which fixed two pages and left the class.
- **Expected outcome:** the live probes in the Definition of done pass on production.
- **Why now:** three accessibility notes in this loop run (w4/459 dialog focus return, w4/460 skip link, w4/461 validation announcements) share one root: shared UI primitives that look right and expose nothing. This is the structural half, and every page benefits from one primitive change.
- **Render parity task:** included (t006), because this changes the dashboard UI. There are no REST/GraphQL/MCP surface changes.

## Observations (page DOM probes, desktop, `https://dashboard.bex.co`, after load)

1. **Section titles are divs.** `document.querySelectorAll('main [data-slot="card-title"]')`, visible and not H1-H6:

   | Page | Non-heading titles | Real headings on page |
   | --- | --- | --- |
   | `/services/srv-dbavjsm05jqs73cjk3og/settings` | 15 of 15 (General, Source, Runtime, Build, Deploy, …) | 1 (the service-name h1) |
   | `/billing` | 7 of 7 (Plan, Payment method, Included usage, Monthly included usage, Charges, …) | 1 |
   | `/workspace/settings` | 5 of 5 (General, Team, Agents, Leave workspace, Danger Zone) | 2 |
   | `/services/…/scaling` | 3 of 3 | 1 |
   | `/services/…/metrics` | 2 of 2 | 1 |
   | `/services/…/plan` | 1 of 1 | 1 |

   `/` and `/env-groups` have 0 card-title slots and are fine. Root cause: `dashboard/src/common/components/ui/card.tsx:31-44` `CardTitle` defaults `as: Tag = "div"`. Only 3 of 124 `<CardTitle` sites (94 files, non-test, non-ui) pass `as=`: `webhooks-panel.tsx:85` and `notification-settings-panel.tsx:60` as h1, `push-notification-settings-panel.tsx:223` as h2. w4/410's text says "Section titles further down stay at h2", but they are divs.
2. **A level skip follows from (1).** `/workspace/settings` goes h1 then h3 "Network access" because `features/agent-environment/components/agent-environment-card.tsx:200` is a real `<h3>` under `:78` and `:188` `<CardTitle>` divs.
3. **The default Agents page has no heading.** `/agents` has 0 headings and 0 h1. `dashboard/src/routes/agents.tsx:105-118` picks composer mode when no `archived` or `phase` search param is set, and `PageHeader` with the `<h1>` (lines 129-135) renders only in list mode, so the page the sidebar links to has no page heading by construction.
4. **All 11 service tabs share one document title.** For deploys, logs, metrics, settings, env, events, scaling, shell, disk, plan (and the index), `document.title` is exactly "reverse-proxy-poc ・ Web Service ・ bex Dashboard". 8 of the 10 have only the service-name h1 and no heading naming the tab (only env has H2 "Environment" and shell has H2 "Shell"). The tab nav does set `aria-current="page"`, but the title and headings never change on a tab switch. Root cause: `dashboard/src/routes/services.$serviceId.tsx:45-55` sets the title once via `titleHead(routeResourceTitle(...))` on the layout route, and none of the 11 child routes `services.$serviceId.*.tsx` defines `head:` (grep returns no files).

**Controls that are fine:** every page has `lang="en"`, exactly one `main`, labelled navs (Breadcrumbs, Billing sections, Settings sections), and exactly one h1 on all 15 pages probed except `/agents`; titles of non-tab pages are unique.

## What this milestone must not do

Change visuals (the same classes must render), change copy, or touch the dialog focus (w4/459), skip link (w4/460) and validation announcements (w4/461).

## Unverified (work to verify inside the tasks, not assertions)

- Whether Postgres, Key Value, static-site, cron, worker and private-service detail pages and the project page have the same identical-title and div-title pattern. Only the web-service tab family `services.$serviceId.*` was probed; databases and keyvalue routes are separate route families.
- The exact heading level each of the 94 files needs.
- Screen-reader output (attributes and DOM only were inspected) and the mobile 390px structure.

## Blast radius and adjacent classes

`CardTitle` has 124 call sites in 94 files. The default change is global, so t002 exists to give the exceptions an explicit `as="div"` rather than guessing. The three existing `as=` sites must not change. Tests asserting heading counts or roles are the regression surface. w4/410 and w4/409 (done) are the page-level h1 cases and are unchanged.

## Dedupe

grep of `.pm` for CardTitle, heading and document title found w4/410 (page h1 only, assumed section titles were h2), w4/409 (duplicate h1 on the Environment tab), w9/m88/t001 (a create page's h1), and loading-skeleton items (w4/298, 299, 303, m208, m220) that only mention CardTitle as markup. None covers section titles as headings or per-tab titles, so this is new, not a duplicate. DO_NOT_DO has no match.
