# w5 · m36 — Dashboard-walk follow-through: close the round-14 gap list

**Worker:** worker5 **Goal:** Every page-level affordance the authenticated dashboard walk (w5/m32) proved Render has and bex lacks is closed — service root lands on Deploys, Logs gets a URL-owned time range, Environment gets a safe Export, Metrics gets an event timeline, Team gets member search, Projects gets search + type filters. Absorbs inbox notes `w5/015`–`020`. **Status:** todo

## Tasks (in order)

| id   | title                                                              | est | depends_on                         |
| ---- | ------------------------------------------------------------------ | --- | ---------------------------------- |
| t001 | Service root lands on Deploys, not Events (← `015`)                | 15m | —                                  |
| t002 | Logs page: URL-owned time-range selector (← `016`)                 | 45m | —                                  |
| t003 | Environment tab: Export action that never leaks secrets (← `017`)  | 30m | —                                  |
| t004 | Metrics page: event-timeline overlay (← `018`)                     | 45m | —                                  |
| t005 | Team table: accessible member search (← `019`)                     | 30m | —                                  |
| t006 | Project resources: URL-owned search + type filters (← `020`)       | 45m | —                                  |
| t007 | Render parity                                                      | 30m | t001, t002, t003, t004, t005, t006 |
| t008 | Simplify                                                           | 20m | t007                               |
| t009 | Test coverage                                                      | 30m | t007                               |
| t010 | Closeout                                                           | 15m | t009                               |

## Definition of done

Each of the six walk-documented divergences is closed and matches the captured Render behavior in `docs/render-artifacts/dashboard-walk/` (services.md, workspace.md): opening `/services/<id>` lands on Deploys with Events still directly reachable; the Logs time-range selection survives refresh/share via the URL; the Environment Export emits a deterministic format and never exposes masked secret values; Metrics shows events within the selected range with Render-equivalent filtering; Team search is keyboard-accessible with distinct empty/no-match states; project resource search + All/Services/Env Groups filters compose, survive refresh, and stay scoped to the selected environment. `yarn test` green; notes `w5/015`–`020` moved to `w5/done/`.

## Source + Goal linkage

- **Source:** inbox notes `w5/015`–`020`, all filed 2026-07-15 by w5/m32's authenticated live Render + mock-cluster bex page-by-page walk (evidence: `docs/render-artifacts/dashboard-walk/` + paired screenshots); bundled per the w8/m16 note-absorption precedent — individually sub-hour, together ~3.5h of one coherent theme.
- **Goal linkage:** Render dashboard parity (docs/ADR018-render-parity.md UI column) — the walk is the evidence base; these are the only page-level affordance gaps it found in shipped page families.
- **Expected outcome:** the service, team, and project pages match Render's navigation and discoverability affordances page-for-page; the walk's gap list reads zero open UI items.
- **Why now:** the captures are fresh — evidence rots as both dashboards evolve; six loose notes are exactly the backlog-refill w5/m32 was run to produce; w5 has only three open milestones.
- **Render parity:** included — UI-surface feature work; t007 re-verifies each page against the captured walk artifacts and flags any drift as follow-up.
