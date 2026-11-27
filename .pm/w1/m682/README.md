# w1 · m682 — A project's page stacks every environment as its own section, as Render's does

**Worker:** worker1 **Goal:** `/project/<id>` draws each environment as Render's section (name, protection, counted switcher, search, tables, empty state), serves Render's `/project/<id>/environment/<envId>` URL, and carries Render's header. **Status:** todo (t001–t008, t010–t012 done; t009 walked on the local stub, production walk pending)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's project page, environment settings page, New menu and Move Services modal in `docs/render-artifacts/project-environments.md` — **DONE** | 30m | — |
| [t002](done/t002.md) | The environment section: h2 name, protected icon and description, counted All / Services / Env Groups switcher remembered per environment — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Each section's search, resource table, env-groups table and no-match state — **DONE** | 1h15m | t002 |
| [t004](done/t004.md) | Render's empty states, with role-disabled buttons — **DONE** | 45m | t002 |
| [t005](done/t005.md) | The page stacks every environment, with the Add environment card and Render's header — **DONE** | 1h30m | t003, t004 |
| [t006](done/t006.md) | `/project/<id>/environment/<envId>` scrolls to and outlines its section; old `?env=` links land there — **DONE** | 1h | t005 |
| [t007](done/t007.md) | Project Settings opens the Name card in edit mode for `?edit=name` — **DONE** | 20m | — |
| [t008](done/t008.md) | The project page's pending frame matches the stacked page at 1440 and 390 px — **DONE** | 45m | t005 |
| [t009](t009.md) | Walk the project page on dev-1 and on production | 45m | t006, t007, t008 |
| [t010](done/t010.md) | Render parity — **DONE** | 20m | t009 |
| [t011](done/t011.md) | Simplify — **DONE** | 15m | t010 |
| [t012](done/t012.md) | Test coverage — **DONE** | 45m | t010 |
| [t013](t013.md) | Closeout | 10m | t011, t012 |

Total: **13 tasks, about 9h50m**.

## Definition of done

1. `/project/<id>` draws every environment as its own section in project order, each with its name, protection, counted All / Services / Env Groups switcher (remembered per environment), search, resource table, env-groups table and Render's empty state; project-only resources keep a section after them.
2. `/project/<id>/environment/<envId>` scrolls to and outlines that section; an old `?env=<id>` link lands on the same section.
3. The header has Render's eyebrow, the name links to `/project/<id>/settings?edit=name`, which opens the Name editor, and + Add environment shows Render's paywall at the plan limit; the rename dialog is gone.
4. A Developer in a protected environment sees Create new service and Move existing services disabled with the reason.
5. The pending frame matches the ready page at 1440 and 390 px, in all eight locales; walked on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), filed to w1 by `/pm` the same day after re-reading Render's dashboard bundle. Render's public dashboard bundle, read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`; chunk hashes change with every Render deploy, so find the code by the strings quoted here): the `Project` chunk maps `environments` to one section each (h2, protected icon, the counted switcher stored per environment, "Search resources in <env>", "<env> is empty", the Add environment card); `page-templates` holds the header ("Edit project name", `?edit=name`, "Get more environments with <plan>"). [render.com/docs/projects](https://render.com/docs/projects) says "click the ••• menu at the top right of that environment's section on the project's page".
- **bex today (checked 2026-11-26):** one URL-selected environment (`environments-panel.tsx`, `selectEnvironmentId`, `?env=`), as Render showed in July ([`dashboard-walk/workspace.md`](../../../docs/render-artifacts/dashboard-walk/workspace.md), w5/m41) and as ADR018's Projects & environments row (◐) records.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); AGENTS.md: when an existing design conflicts with parity, amend the design.
- **Expected outcome:** a user following Render's docs finds each environment's section, its menu and its New button on bex's project page, and every resource of the project is visible without switching environments.
- **Why now:** Render replaced the layout bex copied in July. w1/m683 (environment settings), w1/m684 (New menu) and w1/m685 (Move Services) attach to this section, and w3/m681 t003 adds Generate Blueprint to its menu.
- **Not duplicated (checked 2026-11-26 across every workstream):** no open item touches the project page's layout; w3/m681 adds one menu item.
- **Render parity task:** included (t010): dashboard UI changes, no API change.

## Walks (t009)

- **Local, 2026-11-26** — `yarn local-bex` (its seeded `storefront` project: a protected Production, Staging, and two project-only resources) with the dashboard's dev server, in Chromium at 1440 and 390 px, instead of `dev-1` (no cluster was needed for a dashboard-only change):
  - DoD 1: both environments stacked in order, Production's h2 with the protected icon and `Protected environment`, the counted switcher (`All (4)`, `Services (3)`, `Env Groups (1)`), per-section search, the resource table and the env groups table (Name, Env Vars, Secret Files, Updated), and the `Unassigned` section after them. With Staging's members removed in the read, `Staging is empty`, the longer Kickstart sentence, `Create new service` and `Move existing services`, at both widths.
  - DoD 2: `?env=env-localstaging&q=api&kind=databases` redirected to `/project/prj-local0001/environment/env-localstaging`, which scrolled to Staging and outlined it; two seconds later the outline cleared, and the next wheel scroll put `/project/prj-local0001` in the address bar with the scroll position kept (no remount). `/environment/evm-nope` landed on the project page.
  - DoD 3: the name link opened Project Settings with the Name field focused and `?edit=name` cleared; the stub's Hobby workspace at two environments showed `Add environment 2/2`, which opened `Get more environments with Pro`.
  - DoD 5: the pending frame (Project read held) against the ready page: the header rule, the first section's h2 and its switcher row at the same y at 1440 and 390 px; the search box sits 24 px lower on a protected environment, whose `Protected environment` line the frame cannot know.
  - DoD 4 (a Developer in a protected environment) by component tests, since the stub has no second role.
- **Production:** not yet — needs a pin that carries this milestone.
