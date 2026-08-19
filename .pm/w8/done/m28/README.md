# w8 · m28 — Polish `/agents` as a prompt-first workspace

**Worker:** worker8 **Goal:** make `dashboard.bex.co/agents` feel like starting a coding task, not browsing a CRD list — composer as the room, recents as quiet work, GitHub-empty as a CTA instead of a five-step dead end **Status:** done (Render parity: dashboard-only; no `lego/` / GraphQL document / Connected-agents change; ADR018 accurate — agent sessions remain a bex extension)

## Tasks (in order)

| id   | title                                                                  | est | depends_on     |
| ---- | ---------------------------------------------------------------------- | --- | -------------- |
| t001 | Composer as the room: hierarchy, labeled Start, copy — **DONE**        | 45m | —              |
| t002 | Toolbar: agent select, repo chip, GitHub-empty CTA — **DONE**          | 45m | t001           |
| t003 | Recents rows on the default view; drop the phase filter — **DONE**     | 60m | t001           |
| t004 | Mention chips, keyboard hint, first-run examples, matching skeletons — **DONE** | 45m | t002, t003     |
| t005 | Render parity — **DONE**                                               | 30m | t004           |
| t006 | Simplify — **DONE**                                                    | 30m | t005           |
| t007 | Test coverage — **DONE**                                               | 60m | t005           |
| t008 | Closeout — **DONE**                                                    | 15m | t006, t007     |

## Definition of done

On the default `/agents` view (no `?archived`, no `?phase`):

- The prompt box is the visual center (centered, labeled **Start** / **Starting…**, no competing page title + subtitle + “What should the agent work on?” stack).
- Claude / Gemini / Codex is a toolbar select, not buried in Configuration; the selected repo is a persistent chip (or an **Add repository** chip that opens `@`).
- A workspace with zero GitHub App repos shows a Connect-GitHub CTA in the composer **before** the person writes a prompt — not a post-submit `@` nudge that then says “connect GitHub in workspace settings.”
- History is a list of full-row recents (title + human status phrase + repo + age; entire row clickable). The five-column table and the ten-value **Filter by phase** select are gone from this view. Archived / All may keep a denser list.
- Sidebar and page use the same membership word (**Recent** or **Current**, not both).
- Default empty does not repeat the composer instructions in a dashed Bot card. Loading uses a composer + recents skeleton, not the services card-grid `ListPageSkeleton`.
- Create payload is byte-identical (`createAgentSession` fields unchanged). No `lego/` or GraphQL document changes. One-rail invariant still holds. Settings → Connected agents is untouched.

## Source + Goal linkage

- **Source:** designer review of `https://dashboard.bex.co/agents` (2026-08-18) in the preceding chat; user handoff “ok, hand off to /pm for w8” the same day. Grounded in the shipped UI (`dashboard/src/routes/agents.tsx`, `features/agent-sessions/components/{new-session-composer,session-list,empty-state}.tsx`, `common/components/dashboard-layout/agent-sessions-nav-section.tsx`) and ADR047 D9/D9a (Devin-shaped composer + one rail).
- **Goal linkage:** ADR008 pillar 5 / ADR047 D9 — `/agents` is the primary human entry to cloud coding-agent sessions. The surface shipped (w1/m64) and the rail was corrected (w5/m64); this is product-quality polish of that entry, not a new capability.
- **Expected outcome:** a first-time visitor can start a session without discovering `@` + Configuration + a CRUD table; a returning visitor sees recents that match the rail’s human status language. Observable in the dashboard test suite plus a logged-in walk of `/agents` (empty GitHub, empty sessions, populated Current, Archived).
- **Why now:** the create + history page is live on prod and is the differentiator; every further session-detail milestone compounds a first impression that still reads as an admin list. The review is fresh and scoped; w8 has no open milestone.
- **Render parity:** included as the standing UI-surface closeout. Render has no cloud coding-agent product (ADR047 is a bex extension; ADR018 has no agent-sessions row). Verified 2026-08-19: empty `lego/` diff, `agent-sessions.graphql` unchanged, Connected agents untouched, ADR018 still accurate. Closeout evidence is the dashboard unit suite (no logged-in prod/`dev-8` walk — no dashboard session in this environment).
- **Anti-goal boundary:** do not add a second sidebar, session folders, or a Kanban; do not surface hibernation / pins / snapshot bytes / egress on this page; do not merge with Connected agents; do not change REST/GraphQL/MCP create/list contracts; do not reopen the sandboxes dashboard surface (`DO_NOT_DO.md` #18 carveout is ADR047 sessions, not E2B sandboxes UX).
