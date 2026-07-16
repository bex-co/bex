# w5 · m36 — Dashboard-walk follow-through: close the round-14 gap list

**Worker:** worker5 **Goal:** Every page-level affordance the authenticated dashboard walk (w5/m32) proved Render has and bex lacks is closed — service root lands on Deploys, Logs gets a URL-owned time range, Environment gets a safe Export, Metrics gets an event timeline, Team gets member search, Projects gets search + type filters. Absorbs inbox notes `w5/015`–`020`. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Service root lands on Deploys, not Events (← `015`) — **DONE** | 15m | — |
| t002 | Logs page: URL-owned time-range selector (← `016`) — **DONE** | 45m | — |
| t003 | Environment tab: Export action that never leaks secrets (← `017`) — **DONE** | 30m | — |
| t004 | Metrics page: event-timeline overlay (← `018`) — **DONE** | 45m | — |
| t005 | Team table: accessible member search (← `019`) — **DONE** | 30m | — |
| t006 | Project resources: URL-owned search + type filters (← `020`) — **DONE** | 45m | — |
| t007 | Render parity — **DONE** | 30m | t001, t002, t003, t004, t005, t006 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout — **DONE** | 15m | t009 |

## Definition of done

Each of the six walk-documented divergences is closed and matches the captured Render behavior in `docs/render-artifacts/dashboard-walk/` (services.md, workspace.md): opening `/services/<id>` lands on Deploys with Events still directly reachable; the Logs time-range selection survives refresh/share via the URL; the Environment Export emits a deterministic format and never exposes masked secret values; Metrics shows events within the selected range with Render-equivalent filtering; Team search is keyboard-accessible with distinct empty/no-match states; project resource search + All/Services/Env Groups filters compose, survive refresh, and stay scoped to the selected environment. `yarn test` green; notes `w5/015`–`020` moved to `w5/done/`.

## Source + Goal linkage

- **Source:** inbox notes `w5/015`–`020`, all filed 2026-07-15 by w5/m32's authenticated live Render + mock-cluster bex page-by-page walk (evidence: `docs/render-artifacts/dashboard-walk/` + paired screenshots); bundled per the w8/m16 note-absorption precedent — individually sub-hour, together ~3.5h of one coherent theme.
- **Goal linkage:** Render dashboard parity (docs/ADR018-render-parity.md UI column) — the walk is the evidence base; these are the only page-level affordance gaps it found in shipped page families.
- **Expected outcome:** the service, team, and project pages match Render's navigation and discoverability affordances page-for-page; the walk's gap list reads zero open UI items.
- **Why now:** the captures are fresh — evidence rots as both dashboards evolve; six loose notes are exactly the backlog-refill w5/m32 was run to produce; w5 has only three open milestones.
- **Render parity:** included — UI-surface feature work; t007 re-verifies each page against the captured walk artifacts and flags any drift as follow-up.

## Resolution

Completed 2026-07-15. The authenticated browser re-walk verified all six closures against the round-14 captures:

- `/services/<id>` redirects to Deploys while `/events` remains directly reachable.
- Logs restores a shared `range` URL parameter, applies the concrete history bounds, and explains that live tail remains unbounded by the history window.
- Environment exposes an all-or-nothing deterministic dotenv export; the action is disabled when OpenBao is unavailable and never emits masks or a partial file.
- Metrics shows the shared service-event feed inside the selected range with All, Deploys, Lifecycle, and Configuration filters.
- Team member search filters identity fields and exposes a distinct accessible no-match state without hiding pending invites.
- Project resource search and type filters compose with the selected environment, persist through the URL and refresh, and include linked environment groups.

The simplify pass retained the shared event hook, range parser, and project filter helpers; no further behavior-preserving consolidation improved the diff. Validation passed with `yarn lint`, 207 Vitest files / 1,209 tests, `yarn build`, and the real dev-5 browser walkthrough. `docs/ADR018-render-parity.md` and the dashboard-walk evidence record the closed UI gap list. Source notes `w5/015`–`020` are present in `w5/done/`.
