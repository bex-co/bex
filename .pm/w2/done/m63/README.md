# w2 · m63 — Devin-style dashboard sidebar: icon-rail collapse + drag-to-resize + persisted state

**Worker:** worker2 **Goal:** the dashboard's left sidebar behaves like Devin's (app.devin.ai): collapsible to a slim icon rail (toggle button, Cmd/Ctrl+B, rail click), drag-resizable within a bounded range with snap-to-collapse, and the state + width persist across reloads with no hydration flash. **Status:** done (2026-08-04 — all tasks done; live DoD PASSED on the local dev stack: icon-rail collapse on the main + contextual sidebars, drag-resize within [192,384] with clamp + snap-to-rail + drag-out re-expand, double-click/Cmd+B/header toggle, reload restores state+width from the signed cookie with SSR first-paint proof (`data-state="collapsed"` + `--sidebar-width: 340px` present in server HTML — no flash), mobile off-canvas Sheet intact; `yarn test` 1928 green, typecheck + lint green)

## Behavioral spec (from live research of app.devin.ai, 2026-08-04)

Captured by driving the real Devin app with Playwright (screenshots in `.playwright-mcp/devin-sidebar-{expanded,collapsed}.png`, gitignored — the durable spec is this list):

- Sidebar carries `data-state="expanded" | "collapsed"`; widths are CSS vars (`--sidebar-width: 18.75rem`, rail `3.25rem`). Collapsed rail keeps the same DOM mounted and hides labels via `group-data-[state=collapsed]` variants; nav items become icon-only buttons with tooltips.
- The divider is a WAI-ARIA window splitter: `role="separator"`, `aria-orientation="vertical"`, `tabindex=0`, `aria-controls`, `aria-valuemin/max/now`. Visually a 1px hairline, `cursor: col-resize`; on hover/drag it turns accent-colored and widens to 3px without shifting layout.
- Resize is continuous only within a band (Devin: 300–400px). Dragging below a threshold (~50% of min) snaps to the collapsed rail; dragging outward from the rail snaps back to expanded. The sidebar can never rest at an unusable in-between width.
- Double-click on the separator toggles expanded ⇄ collapsed. Cmd/Ctrl+B toggles. A panel-left icon button in the sidebar header toggles; when collapsed, the toggle appears in the content header instead.
- Persistence is one cookie storing a **signed width** (`360` = expanded at 360px, `-360` = collapsed remembering 360px), so SSR renders the correct state/width with no flash.

## Current state (mapped 2026-08-04)

`dashboard/src/common/components/ui/sidebar.tsx` is the full shadcn kit — `collapsible="icon"` CSS, Cmd+B, `SidebarRail`, `sidebar_state` cookie write all exist — but all three consumers (`dashboard-layout/dashboard-sidebar.tsx:82`, `project-sidebar.tsx:44`, `service-sidebar.tsx:46`) use `collapsible="offcanvas"`, `SidebarRail` only click-toggles (no drag), and the cookie is written but never read back (`dashboard-layout/index.tsx:29` passes no `defaultOpen`), so every load resets to expanded.

## Tasks (in order)

| id   | title                                                                                 | est | depends_on | status       |
| ---- | ------------------------------------------------------------------------------------- | --- | ---------- | ------------ |
| t001 | Icon-rail collapse: switch the three sidebars to `collapsible="icon"` + tooltip audit | 45m | —          | — **DONE**   |
| t002 | Drag-to-resize `SidebarRail` with bounded range, snap-to-collapse, dblclick, ARIA     | 60m | t001       | — **DONE**   |
| t003 | Persist signed-width cookie + SSR read-back (no hydration flash)                       | 45m | t002       | — **DONE**   |
| t004 | Render parity: verify dashboard-surface consistency, document deliberate divergence   | 20m | t003       | — **DONE**   |
| t005 | Simplify: run `/simplify` over the changed sidebar code                                | 20m | t004       | — **DONE**   |
| t006 | Test coverage: collapse/resize/persistence behavior tests                             | 30m | t004       | — **DONE**   |
| t007 | Closeout                                                                               | 15m | t006       | — **DONE**   |

## Definition of done

On a running dashboard: the left sidebar collapses to an icon rail (labels hidden, icons with tooltips remain) via the header toggle, Cmd/Ctrl+B, and rail click; dragging the sidebar edge resizes it continuously within the bounded range; dragging below the snap threshold collapses it to the rail and dragging outward re-expands it; double-clicking the divider toggles; after a reload the sidebar reappears in the same state **and** width with no visible flash (SSR-hydrated from the cookie); mobile keeps the existing offcanvas Sheet behavior. `yarn test`, typecheck, and lint green.

## Render parity outcome (t004)

Dashboard-chrome only — no REST/GraphQL/MCP surface changed, so the parity check
is scoped to the UI. Render's own dashboard ships a **fixed-width, non-collapsible
left nav** (no icon rail, no drag-resize, no persisted width). bex now diverges
**deliberately**, adopting Devin's (and Claude.ai's / ChatGPT's) richer pattern:
icon-rail collapse + bounded drag-resize + signed-width cookie persistence. This
is an additive UX enhancement, not a regression — every existing nav entry,
grouping, contextual (project/service) sidebar, and the mobile off-canvas Sheet
behave as before; only the collapse affordance and resizability are added. No
follow-up parity work is owed. (The `SessionSidebar` on the agent-chat page is a
separate hand-rolled `w-72` rail, tracked for the same treatment in `w2/017`.)

## Source + Goal linkage

- **Source:** user-directed live research of Devin's sidebar (this session, 2026-08-04: Playwright drive of app.devin.ai — drag ranges, snap thresholds, ARIA separator semantics, signed-width cookie) plus a code map of `dashboard/src/common/components/ui/sidebar.tsx` and its three consumers.
- **Goal linkage:** dashboard UX quality for the human surface (`dashboard/CLAUDE.md`); complements the w3/m44 Devin-style agent-session chat by bringing the surrounding chrome up to the same interaction standard.
- **Expected outcome:** users can reclaim horizontal space (icon rail) or tune the nav width to taste, and the choice sticks across sessions — matching the interaction pattern of the best current agent dashboards (Devin, Claude.ai).
- **Why now:** the shadcn primitives already in the repo do 80% of the work (icon-collapse CSS, Cmd+B, cookie write) but are wired to offcanvas-only with dead persistence — small, contained changes to one primitive file + three consumers deliver the whole pattern while the w3/m44 chat work is fresh and before more layout code accretes against the fixed-width assumption.
- **Render parity note:** included (t004) even though this is dashboard-chrome-only (no REST/GraphQL/MCP change): the task verifies no dashboard behavior regressions, checks render.com's own sidebar behavior once, and records the deliberate Devin-style divergence rather than silently diverging.
