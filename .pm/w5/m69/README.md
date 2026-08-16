# w5 · m69 — Perceived-latency polish: content skeletons + retire redundant pending config

**Worker:** worker5 **Goal:** replace the bare centered spinner on top-level list routes with content-shaped skeletons, and rationalize the per-route pending config the persistent-shell ship made partly redundant **Status:** todo

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------ | --- | ---------- |
| t001 | Content skeletons for the ~8 top-level list routes                        | 1h  | —          |
| t002 | Audit + rationalize the redundant `pendingMs: 0` / component-as-pending    | 45m | —          |
| t003 | Verify no flash / no double-mount across the audited routes               | 30m | t001, t002 |
| t004 | Render parity                                                             | 20m | t003       |
| t005 | Simplify                                                                 | 20m | t004       |
| t006 | Test coverage                                                            | 30m | t004       |
| t007 | Closeout                                                                 | 10m | t006       |

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
