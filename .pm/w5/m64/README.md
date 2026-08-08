# w5 · m64 — One nav rail: fold the agent-sessions sidebar into `DashboardSidebar` (Devin-consistent)

**Worker:** worker5 **Goal:** `/agents` renders **one** left rail, not two. Today `agents.tsx` mounts `<DashboardLayout>` (which already renders `DashboardSidebar`) and then renders a **second** bespoke `<aside class="w-60 border-r lg:flex">` (`features/agent-sessions/components/session-sidebar.tsx`) inside the page body — the only route in the dashboard that does this. Every other contextual page (`/project/$projectId*`, `/services/$serviceId*`) swaps the rail's _contents_ **inside** `DashboardSidebar`'s own route switch. This milestone brings `/agents*` onto that same one-rail contract and shapes it per Devin: global nav **plus** a contextual `Recent` sessions section in a single `<Sidebar collapsible="icon">`. **Status:** implemented + locally verified (t001–t008 done); awaiting ship + prod walk to close (t009)

## Implementation status (2026-08-07)

Implemented and verified on the local `dev:local` + `local-bex` stack — dashboard **typecheck + lint + 293 files / 1944 tests green**:

- **One rail.** `AgentSessionsNavSection` renders inside `DashboardSidebar` (a third branch beside `ProjectSidebar`/`ServiceSidebar`, augmenting rather than replacing the nav); `session-sidebar.tsx` deleted; both `SessionSidebar` usages removed from `agents.tsx` and the two in `agents_.$agentSessionId.tsx`.
- **Affordances ported** onto the shared `Sidebar` primitives: New session + bare `O` shortcut, `Recent` search over title+repo, More → `?view=list`, human status phrases, and the GitHub PR link as a **sibling** `SidebarMenuAction` (never nested in the row anchor).
- **Visual fix found by the live walk:** the PR badge landed on the *title* line and collided with it (the primitive's default `top-1.5`); repositioned onto the status line, matching Devin. Screenshots: `.playwright-mcp/bex-agents-one-rail.png` (before), `bex-agents-rail-fixed.png`, `bex-agents-detail.png`, `bex-agents-collapsed.png`.
- **Collapsed + mobile:** icon mode keeps nav icons and drops the list (Devin's own answer); below `lg` the group now rides the mobile Sheet — an improvement on the old rail, which was `hidden … lg:flex` and unreachable there.
- **Regression guard:** `dashboard/src/routes/__tests__/one-rail-invariant.test.ts` fails if any route module grows an `<aside>` or imports a `*-sidebar`; verified it flags the pre-fix `agents.tsx`. `DashboardLayout`'s now-dead `sidebar` override prop removed (t007) so there is one way to do this.
- **t006 (Render parity):** zero changes under `lego/`, zero GraphQL document changes, no ADR018 row affected — confirmed by `git status`.
- **Coverage moved, not dropped:** the retired `session-sidebar.test.tsx`'s component cases are re-hosted in `dashboard-layout/__tests__/agent-sessions-nav-section.test.tsx` (8 tests, incl. contextual scoping across four non-agent routes and a single-poller assertion); its `agentSessionStatusPhraseKey` block moved to `features/agent-sessions/lib/__tests__/mapper.test.ts`.

**Open (t009):** the defect was reported against deployed `dashboard.bex.co/agents`, and the fix is uncommitted — the live confirmation can only happen after `/ship` + a green deploy.

## Tasks (in order)

| id   | title                                                                                       | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Agent-sessions branch in `DashboardSidebar`: nav groups + contextual `Recent` section, one rail | 60m | — — **DONE** |
| t002 | Port the session-rail affordances onto the shared `Sidebar` primitives                        | 60m | t001 — **DONE** |
| t003 | Delete the second rail from `/agents` + `/agents_.$agentSessionId`; keep the escape hatches    | 45m | t001, t002 — **DONE** |
| t004 | Collapsed icon-rail + below-`lg` behavior                                                     | 30m | t003 — **DONE** |
| t005 | Record the one-rail convention in ADR047 D9                                                   | 30m | t003 — **DONE** |
| t006 | Render parity                                                                                 | 30m | t004, t005 — **DONE** |
| t007 | Simplify                                                                                      | 30m | t006 — **DONE** |
| t008 | Test coverage                                                                                 | 45m | t006 — **DONE** |
| t009 | Closeout                                                                                      | 15m | t007, t008       |

## Definition of done

- On `/agents` and `/agents/<id>` the rendered page contains **exactly one** left rail — asserted by a test that counts sidebar landmarks in the layout, not by eyeball.
- That rail carries the workspace nav groups **and** the `Recent` sessions list, in Devin's order (nav above, contextual list below), inside the one `<Sidebar collapsible="icon">` so ⌘B collapses it like every other page.
- `features/agent-sessions/components/session-sidebar.tsx`'s bespoke `<aside>` no longer renders in the page body on any route.
- No affordance regresses from w3/m45 t004: `Recent` search, the More/view-all target (`?view=list`), human status phrases with the GitHub PR link, the `O` new-session shortcut, and the active-session highlight all still work.
- Collapsed (icon) mode drops the session list and keeps the nav icons. **Below `lg` the outcome improved on the plan:** rather than reproducing the old rail's `hidden … lg:flex` dead end, the group rides `SidebarProvider`'s mobile Sheet, so sessions are reachable from the drawer.
- The session **detail** view keeps its right-hand evidence/PR panel — this milestone moves nothing to the right side.
- Dashboard suite green (typecheck + lint + tests). **The plan named `session-sidebar.test.tsx` as an updated file; it was deleted instead** — its component cases moved to `dashboard-layout/__tests__/agent-sessions-nav-section.test.tsx` and its mapper block to `lib/__tests__/mapper.test.ts`, so no coverage was lost.

## Source + Goal linkage

- **Source:** user directive 2026-08-07 ("`/agents` has two side bars, which is incorrect … show those sessions in the sidebar (shared with other nonAgent items) just like devin" → "we need to be consistent with devin for nav bar"), grounded in a live Playwright study of the user's Devin org the same day (evidence `.playwright-mcp/devin-home.png`, `devin-session.png`, `devin-review.png`, `devin-automations.png`, `devin-wiki.png`, `devin-recent-menu.png`, `devin-collapsed.png`). Direct follow-on to **w3/m45 t004**, which built the sessions rail as a separate `<aside>`; this corrects where that rail lives.
- **Goal linkage:** `docs/ADR047-cloud-coding-agent-sessions.md` D9 (dashboard session surface) — pillar 3/4 of `docs/ADR008-vision.md` (AI-native surface). Navigation coherence is a precondition for the agent-sessions surface reading as part of the product rather than a bolted-on app.
- **Expected outcome:** one rail everywhere in the dashboard, with the agent-sessions page finally obeying the same `DashboardSidebar` contract as Project and Service pages. Reclaims ~240px of horizontal space on `/agents` and removes the "which sidebar am I in?" ambiguity the user reported.
- **Why now:** the defect is live on `dashboard.bex.co/agents` and is structural, not cosmetic — every further agent-sessions UI milestone (m77's transcript surface next) builds on this layout and would compound the divergence. The fix is also cheap _now_ because `DashboardSidebar` already has the route-switch mechanism (`projectId`/`serviceId` branches, `dashboard-sidebar.tsx:70-80`); this adds a third branch rather than inventing a mechanism.
- **Render parity task — included, with a noted reference swap.** The milestone changes a user-facing surface (dashboard UI), so the standing task stays. But render.com ships **no** agent-sessions product, so there is no Render behavior to diff against: the parity reference for _this_ surface is ADR047 + the captured Devin evidence, and the Render-side check reduces to confirming this milestone introduces **no** REST/GraphQL/MCP drift (it should touch zero backend surface — a pure dashboard layout change). t006 asserts exactly that.

## Deliberate boundaries

- **`ProjectSidebar`/`ServiceSidebar` keep replacing the nav; the agents branch augments it.** Devin's rail keeps global nav above the contextual list; bex's project/service rails swap the whole rail for a back link + section nav. That divergence is **out of scope here** — converting them is a much larger IA change touching every project/service page. t005 records the distinction as an intentional convention so the next reader does not "fix" one to match the other.
- **The contextual list stays scoped to `/agents*`** — matching Devin, whose `Recent` slot is per-section (sessions on session routes, pull requests on `/review`, absent on `/automations` and `/wiki`). Sessions do **not** appear on Projects/Services/Settings pages.
- **No right-hand-panel work.** Devin's second panel is a right-side workspace pane; bex already has the evidence/PR panel there from w3/m44. Untouched.
- **Not the sandboxes surface.** `/agents` is ADR047 cloud coding agent sessions; the w5 sandboxes dashboard surface remains excluded per the standing user directive (2026-07-30) and `DO_NOT_DO.md` #18's scope.
