# w1 · m90 — Repo-less agent sessions get a real identity

**Worker:** worker1 **Goal:** every agent session is distinguishable in the sidebar rail, the recents list, the detail header, and the browser tab — including the repo-less (chat-only) sessions that today render with no title at all **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on             |
| ---- | --------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Derive a session display name and expose it on the view model                      | 45m | —                      |
| t002 | Detail header: title from the derived name; suppress the empty branch row          | 30m | t001                   |
| t003 | List rows: drop the empty separators and render the derived name                   | 30m | t001                   |
| t004 | Document title + breadcrumb reflect the session, not the constant "Session"        | 20m | t001                   |
| t005 | Render parity — agent-session identity across REST/GraphQL/MCP/UI                  | 30m | t002, t003, t004       |
| t006 | Simplify the code this milestone changed                                          | 30m | t005                   |
| t007 | Test coverage for both session shapes                                             | 40m | t005                   |
| t008 | Closeout                                                                          | 15m | t007                   |

## Definition of done

A repo-less session created from the `/agents` composer renders:

- a **non-empty `<h1>`** on `/agents/$id` (today the DOM is literally `<h1 class="truncate text-sm font-semibold"></h1>`),
- **no orphan `GitBranch` icon** in the header meta row,
- **no `· ·` empty separators** in the recents list or the sidebar rail,
- a browser tab title that distinguishes it from other sessions.

Repo-backed sessions are byte-for-byte unchanged in what they display. Both shapes are asserted by tests, and the assertions read the rendered text (not just that a component mounted) so an empty string fails.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-08-28, finding 1 — verified live in `dev-1` by running sessions through the UI and reading the DOM. Root cause is `dashboard/src/features/agent-sessions/components/session-detail-header.tsx:194`, which is `<h1>{session.repo}</h1>`; the list equivalents are `session-list.tsx:239-241` and `:298` (`{" · "}{s.repo}{" · "}`).
- **Goal linkage:** pillar 5 (ADR008), the ADR047 D9 session surface. Extends the surface `w1/m64` / `w3/m44` built rather than duplicating it — those shipped the list/composer/detail and the full-page chat for the **repo-backed** shape; this closes the repo-less shape they did not cover.
- **Expected outcome:** a tenant with more than one chat-only session can tell them apart. Today every one of them is an untitled row with two empty separators.
- **Why now:** repo-less is the **default zero-config path** — `validateCreate` explicitly supports it (no repo ⇒ `BEX_AGENT_DELIVER=0`, no clone, no PR) and the composer does not require a repo, so it is the first session a new tenant runs. It is also the only shape that works before a GitHub App connection exists.
- **Render parity:** **included.** Render has no coding-agent product, so per `docs/ADR018-render-parity.md` parity here is bex's own cross-surface discipline. The parity task's specific job is to confirm the display name stays a **UI-side derivation** and does not leak as a new API field on REST/GraphQL/MCP.
