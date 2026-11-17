# w3 · m305 — Workspace Settings has Render's sections: audit-log CSV export, registry credentials and notification overrides

**Worker:** worker3 **Goal:** workspace-scoped settings live in Workspace Settings where Render puts them, and a Pro admin can export audit logs as Render's CSV **Status:** blocked — t001–t004, t006–t008 done; the 2026-11-17 live checks found the CSV's status/metadata and the Workspace Settings skeletons off, fixed on main; t005's re-check and t009 wait on the production deploy of the fix

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit Logs section moves to Workspace Settings — **DONE** | 1h | — |
| t002 | Export a date range of audit logs as CSV — **DONE** | 1.5h | t001 |
| t003 | Container Registry Credentials section moves to Workspace Settings — **DONE** | 1h | — |
| t004 | Notification Overrides lists the services that differ from the workspace default — **DONE** | 1h | — |
| t005 | Pending skeletons match the new Workspace Settings layout | 45m | t001, t002, t003, t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 40m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- On dev-3: a Pro workspace admin exports a chosen date range (≤ 90 days) as CSV whose rows equal REST `GET /v1/owners/{ownerId}/audit-logs` for that range, in Render's columns; a Hobby admin sees the plan notice; a non-admin sees no export
- Audit Logs and Container Registry Credentials are Workspace Settings sections and no longer appear in account settings; old links land on the new sections
- Workspace notification settings list the services overriding the default, matching `GET /v1/notification-settings/overrides`
- en/zh strings; pending skeletons match the ready layouts at 1440 and 390; dashboard `yarn lint` and `yarn test` green

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-11-09/10, item 4 (filed by `/pmx them all to w3`); render.com/docs/audit-logs, render.com/docs/notifications, `docs/render-artifacts/workspace-lifecycle.md:16`; bex `features/audit/hooks/use-audit-log.ts:94-99`, `settings-page/index.tsx:92`, `workspace-notifications-card.tsx:188-213`
- **Goal linkage:** Render parity first (ADR008) on the dashboard
- **Expected outcome:** Pro-plan compliance export works as on Render, and workspace settings stop being split across the account page
- **Why now:** audit logs are already a paid Pro feature (w8/m138) without the export Render's Pro includes; w3/m271 just shipped workspace notification policy, and the overrides list completes it
- **Recommendation in place of an open decision:** the July dashboard walk classified bex's audit table as a deliberate superset; the proposal recommended amending that under ADR008's parity-first rule (keep the table, add Render's export and placement). The user has not been asked separately
- **Render parity:** included — dashboard UI changes

## Decision record (2026-11-10, loopx w3)

- **The July "superset" classification is superseded.** `docs/render-artifacts/dashboard-walk/workspace.md` rated bex's in-app audit table under account settings a deliberate superset with no gap. Under ADR008's parity-first rule (user decision 2026-10-17) a missing Render control is a gap whatever bex adds beside it, so the export and Render's placement shipped and the table stays as a bex extension. This is the proposal's recommendation applied by the rule, not a separate user decision.
- **Where the live checks run.** The DoD's dev-3 walk moved to production: dev-3 could not onboard a tenant after the local cluster recovered (t005 progress), and the production workspace `bex-canary` is a paid admin workspace, a stronger check of the plan and role gates.

## Blocked (2026-11-10, loopx w3)

- **Gate:** the checks need the dashboard built from this milestone's commit (`feat(dashboard): … (w3/m305)`).
- **Release condition:** a `chore(deploy): pin platform images to <sha>` on main whose sha contains the commit. Then, signed in as the QA user (`scripts/qa-login.sh --serve`) on `dashboard.bex.co` in `bex-canary`: (1) hold the `workspace.settings` lazy chunk with a Playwright route, navigate client-side to `/workspace/settings`, and compare each `[data-skeleton-region]` with the ready section's box at 1440 and 390 (record tops and heights in t005); (2) export the last 7 days as CSV and check its rows against `GET /v1/owners/{ownerId}/audit-logs` for the same range; (3) see Container Registry Credentials and Notification Overrides on the page and neither on `/settings`. Log out, then run t009.
- No user decision needed.

## Unblocked (2026-11-17, loopx w3)

- **Affected:** t005, t009. **Source:** verified evidence.
- **Finding:** the production pin `336b647b1` (`chore(deploy): pin platform images to 6984548a30f8`, dashboard `bex-dashboard@sha256:1eeffddef4b5…`) contains this milestone's commit `557f1cf90`: `git merge-base --is-ancestor 557f1cf90 6984548a30f8` holds. The release condition in § Blocked is met.
- **Next:** as the QA user in `bex-canary` on `dashboard.bex.co`, run checks (1)–(3) of § Blocked at 1440 and 390, record them in t005 and this README, then t009. Later skeleton work on this page ([w4/684](../../../w4/done/684.md), [w4/686](../../../w4/done/686.md)) is in the pin too, so the comparison reads today's frames.

## Live checks (2026-11-17, loopx w3, pin `6984548a30f8`)

The QA user ran the three checks in `bex-canary` (Scale, admin, two workspaces). Placement holds; the skeleton and the CSV did not.

- **(3) Placement holds.** Workspace Settings lists Registry Credentials, Notifications (the overrides card under the policy card) and Audit Log in its rail and body. `/settings` has neither a `#registry-credentials` nor an `#audit-logs` section; its rail is Account · Integrations · Access credentials · Security · Danger zone.
- **(2) The CSV did not equal REST.** The export of 2026-11-10 → 2026-11-16 had 8489 rows and REST paged to 8489 rows with the same order, timestamps, actors and events. Two columns differed:
  - `status`: the 8 denied rows read `denied`, where REST and Render's enum say `error` (REST keeps the denial as `metadata.outcome: "denied"`);
  - `metadata`: every row carried bex's GraphQL fields (`resource`, `actorMethod`, `relation` on allowed rows) instead of REST's Render map (`service`, `outcome`, `source_ip`, …).
  - The cause: the export built its rows from GraphQL `auditLogs`, bex's own dialect, rather than from Render's REST list.
- **(1) The skeleton did not match.** Holding the `workspace.settings` chunk (§ Blocked's method) never shows the route skeleton: `pendingComponent` shares the route's lazy chunk, so a held chunk paints the generic `RoutePending` spinner. The route was held instead by giving it a pending `loader` through `window.__TSR_ROUTER__` (chunk loaded, client navigation from `/`). The inline skeleton was read on a direct load with `api.bex.co/graphql` held. Pending → ready heights:

  | Region | 1440 | 390 | Cause |
  | --- | --- | --- | --- |
  | rail | 418 → 456 (10 → 11 links) | 10 → 11 options | no Build Pipeline item; the inline rail also lacked Audit Log |
  | general | 210 → 233 | 210 → 409 | generic `CardSkeleton`; the ready grid stacks at 390 |
  | notifications | 721 → 717 | 795 → 811 | the overrides card's description wraps at 390 |
  | registry-credentials | 158 → 306 | 158 → 426 | two bars; the ready empty state and a description narrowed by the Add button |
  | audit-logs | 1348 → 1354 | 1414 → 1524 | header bars, the export row's bars (the ready button wraps), half a pixel per row |
  | danger-zone | 154 → 434 | 194 → 534 | no Delete card (two workspaces) |

  Every other region matched within 4 px at both widths. `/settings`' route skeleton is generic as well (account 384 → 2001 at 1440); see the decision below.

## Fixed (2026-11-17, loopx w3)

- **Export:** the CSV now pages Render's REST list itself (`GET /v1/owners/{ownerId}/audit-logs`, cookie session, `limit=1000` by cursor; CORS allows the dashboard origin), so each row is REST's: `status` is `success|error` and `metadata` is REST's map. In the browser, the same read from `dashboard.bex.co` answered 200 with 1000 rows.
- **Skeletons:** the route skeleton and the page's inline skeleton draw one body (`WorkspaceSettingsSkeletonSections`) and their rail lists the sections they draw (`useWorkspaceSettingsSkeletonSections`: Build Pipeline for billing managers, Audit Log for admins, both kept while unknown; the Delete card when there is another workspace). New card skeletons for General and Delete workspace; the registry list's loading frame is its empty state's box (also in the panel's own loading state); the overrides, registry and audit skeletons render their static titles, descriptions, labels, hint and button boxes, so they wrap as the ready cards do; the audit placeholder row reserves the source cell's font-mono line.
- **Local check** (`yarn local-bex` + Vite, the route held the same way, 20 audit events patched in): general 233/233 and 409/409.5, notifications 717/717 and 811/811, registry 306/306 and 426/426, audit 1353.5/1353.5 and 1523.5/1523.5, Delete card 256/256 and 316/316 at 1440/390. The stub's own data differs from production in the other sections, so those were read on production only.

## Decision record (2026-11-17, loopx w3)

- **`/settings`' route skeleton is not m305's.** m305 removed the Registry Credentials and Audit Log cards from it; its regions and rail now match the ready page's five sections and card counts. Its heights come from the generic `CardSkeleton` frames of w5/m79 t011, which w4/686 left unaudited. Filed as [w3/067](../../067.md). t005's account-page criterion is narrowed to the sections m305 moved. Basis: the mismatch predates this milestone, and another item now owns it.
- **Loader-less routes never paint their route skeleton on a cold navigation** (the chunk finding above). Filed as [w3/066](../../done/066.md); it does not change what m305 checks, because the inline skeleton and the forced-loader probe both paint this page's frame.

## Blocked (2026-11-17, loopx w3)

- **Gate:** the fixes above need a production deploy: a `chore(deploy): pin platform images to <sha>` on main whose sha contains the newest `fix(dashboard): … (w3/m305)` commit (`git log --grep 'w3/m305' origin/main`, then `git merge-base --is-ancestor <commit> <sha>`).
- **Then**, as the QA user in `bex-canary` on `dashboard.bex.co`:
  1. From `/`, set `window.__TSR_ROUTER__.routesById['/workspace/settings'].options.loader` to a held promise, navigate client-side to `/workspace/settings`, and read each outermost `[data-skeleton-region]` against the ready `#<id>` at 1440 and 390. Repeat on a direct load with `api.bex.co/graphql` held. Every region within 4 px except data-dependent counts (Team rows, a workspace with registry credentials), and the rail lists the same sections.
  2. Export a closed past range and compare it with REST for the same range: same row count and every column equal (`status`, and `metadata` parsed as JSON).
  3. Log out, then run t009.
- No user decision needed.
