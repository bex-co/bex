# w2 · m83 — Simplify the agent composer and de-duplicate Recents

**Worker:** worker2 **Goal:** one way to attach a repository and one Recents surface on `/agents`, instead of the three repo affordances and two session lists the page accreted across four milestones **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Collapse the two repo-attach controls into one affordance — **DONE** | 45m | — |
| t002 | Resolve the duplicate Recents between the rail and the page — **DONE** | 45m | — |
| t003 | Re-check the `/agents` skeletons against the simplified ready state — **DONE** | 30m | t001, t002 |
| t004 | Render parity — `/agents` surface consistency — **DONE** | 20m | t001, t002, t003 |
| t005 | Simplify the code this milestone changed — **DONE** | 30m | t004 |
| t006 | Test coverage for the consolidated controls — **DONE** | 30m | t004 |
| t007 | Closeout — **DONE** | 15m | t006 |

## Definition of done

- The composer offers **exactly one** control for attaching a repository. Today it has "Add repository", "Mention a repository or session", and a hint advertising `@` — three affordances for one job, all confirmed rendered simultaneously in `dev-1`.
- `/agents` shows **one** Recents surface. Today the sidebar rail renders "Agent sessions → Recent" and the page renders its own "Recent" list with Recent/Archived/All tabs, both on screen at once.
- The `/agents` and detail skeletons still match their ready states at desktop **and** narrow-mobile, per the repo rule in `CLAUDE.md` / `dashboard/CLAUDE.md`.
- No capability is lost: archived/all filtering and mention-based repo/session insertion remain reachable.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-08-28, findings 5 and 6 — observed directly in `dev-1`. The composer's rendered controls are `Agent`, an unlabeled model select, `Add repository`, `Mention a repository or session`, `Advanced`, `Start session`, plus the hint "Enter to start · Shift+Enter for a new line · @ for a repo".
- **Goal linkage:** pillar 5 (ADR008) and ADR047 D9a, which established the **one-rail** convention for this page; the page-level list is the leftover D9a did not remove.
- **Expected outcome:** a first-time user has one obvious way to attach a repo, and does not see the same session list rendered twice on one screen.
- **Why now:** this surface accreted across `w1/m64` (list + composer + detail), `w4/m36` (`@` mention), `w3/m44` (full-page chat) and `w5/m64` (the rail). Each addition was reasonable alone; the consolidation pass is the cheapest it will ever be, and doing it before pillar 5 opens up avoids teaching the redundancy to more users.
- **Render parity:** **included** — this changes a tenant-facing UI surface. Render has no agent-session product, so the task is bex's own cross-surface discipline: confirm no API/GraphQL/MCP capability becomes unreachable from the UI as controls are merged.

## Implementation decisions (2026-08-29)

- The one visible attachment control opens the existing mention picker. It shows the selected repository in place, while typed `@` insertion and prior-session references remain supported by the same editor and picker.
- The dashboard rail is the canonical Recents surface. The default `/agents` page is composer-only; Archived and All moved into the rail, while explicit `?archived=` and `?phase=` URLs render the full history view. Legacy `?archived=true` canonicalizes to `archived`.
- Composer and filtered-history pending states now mirror their separate ready geometries at desktop and narrow-mobile. The detail layout did not change, so its conversation skeleton required no edit.
- The GraphQL/REST/MCP create and list surfaces were audited read-only. Existing UI paths still cover repository, branch, agent/model configuration, egress, task/session references, archive membership, and phase filtering; no backend surface changed.
- Browser closeout covered default, Archived, All, legacy archive, and phase URLs at both breakpoints. `yarn typecheck && yarn lint && yarn test` passed with 388 files and 2,848 tests.
