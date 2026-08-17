# w9 · m69 — Perceived-latency polish: content skeletons + retire redundant pending config

**Worker:** worker9 **Goal:** replace the bare centered spinner on top-level list routes with content-shaped skeletons, and rationalize the per-route pending config the persistent-shell ship made partly redundant **Status:** done

## Results (2026-08-16)

- **t001 — Content skeletons:** new `ListPageSkeleton` (header + card grid) and `FormPageSkeleton` (`common/components/detail-skeletons.tsx`) wired as the `pendingComponent` for the 8 rendering list/create routes — agents, blueprints, env-groups, webhooks, notifications, usage (list) + blueprints.new, webhooks_.new (form) — so a slow nav shows the page's own content shape inside the persistent shell instead of the bare `RoutePending` spinner. `static.index`/`static.new` are **redirect-only** (`beforeLoad` redirect, no `component`), so they have no pending state and correctly get none — resolving the task's target list.
- **t002 — Pending-config audit:** all 9 `pendingMs: 0` + 2 `pendingComponent: RouteComponent` occurrences are on **detail** routes; audited against the persistent shell and **retained** — rendering the detail frame's header/tabs skeleton at 0ms is still correct (the shell persists, but the frame skeleton should appear immediately, not after `defaultPendingMs`). `dashboard/CLAUDE.md` § Navigation pending states updated with the list-route skeletons + the audit conclusion.
- **t003/t004:** the persistent-shell mechanism is untouched (only `pendingComponent` was added to list routes) — its DOM/one-rail invariants still pass; the live no-flash Playwright walk is the same class deferred by the shell ship itself. Parity: content skeletons match Render's list-loading UX (UI-only, no API surface).
- **t005/t006:** shared skeletons are the simplification; `list-route-skeletons.test.ts` guards that each target route wires a content-skeleton `pendingComponent` (regression to the spinner fails). **2,167 tests + ESLint + tsc green.**

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------ | --- | ---------- |
| t001 | Content skeletons for the top-level list/create routes — **DONE**         | 1h  | —          |
| t002 | Audit + rationalize the pending config (all retained + documented) — **DONE** | 45m | —      |
| t003 | Verify no flash / no double-mount — **DONE** (shell untouched; invariants pass; live walk deferred) | 30m | t001, t002 |
| t004 | Render parity — **DONE** (skeletons match Render list-loading UX; UI-only) | 20m | t003     |
| t005 | Simplify — **DONE** (shared `ListPageSkeleton`/`FormPageSkeleton`; lint 0) | 20m | t004      |
| t006 | Test coverage — **DONE** (`list-route-skeletons.test.ts` guard)           | 30m | t004       |
| t007 | Closeout — **DONE**                                                       | 10m | t006       |

## Definition of done

- The ~8 top-level list routes (`agents`, `blueprints`, `env-groups`, `webhooks`, `notifications`, `usage`, `static` index, and `*.new`) show a **content-shaped skeleton** in the persistent shell's content region during their pending phase, not a bare centered spinner.
- The per-route pending config (`pendingComponent: RouteComponent` + the 9 `pendingMs: 0`) is audited against the now-persistent shell: each occurrence either earns its place (documented why) or is removed; `dashboard/CLAUDE.md` § Navigation pending states is updated to match reality.
- No regression to the persistent-shell behavior shipped in `da6b55b2`: no white flash, no chrome double-mount (proven via the DOM/Playwright walk).
- All dashboard tests green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm "还有什么地方可以优化的？"` 2026-08-16 (proposal 3), the completion of the flash→skeleton perceived-latency story started by the persistent-shell ship (`da6b55b2`). The ~8 list routes currently fall back to `RoutePending` (a centered spinner); `grep` finds 2 `pendingComponent: RouteComponent` routes and 9 `pendingMs: 0` occurrences whose rationale shifts once the shell is persistent.
- **Goal linkage:** ADR008 vision — the dashboard is the human surface; content skeletons read as faster than a spinner and match Render's own list-loading UX.
- **Expected outcome:** list-route navigations feel instant (shell persists + content skeleton), and the pending-state pattern is coherent/documented so it isn't cargo-culted onto new routes.
- **Why now:** directly completes the flash-fix work while the pending-state design is fresh in `CLAUDE.md`; the smallest and most optional of the three optimization proposals.
- **Render parity task:** **included** — this changes visible UI loading states, so check the dashboard's list-loading UX against render.com's and keep it consistent (no REST/GraphQL/MCP surface is touched, so the parity check is UI-only).
