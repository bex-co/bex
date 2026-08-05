# w3 · m45 — Agent-session composer: Devin-style @-mention prompt box + sidebar polish

**Worker:** worker3 **Goal:** bring the CREATE surface and the sessions sidebar up to the same Devin grade as m44's session view — the `/agents` main pane becomes one prompt box (task textarea + in-input toolbar: `@` mention picker, Configuration popover, Send), repo selection moves into a typed `@` mention picker with fuzzy filtering + readiness preview, the Advanced fields relocate into a Configuration popover, and the sessions sidebar gains search/More/human status phrases + a New-session shortcut. **Status:** done — shipped 265137ec, deploy green, prod-verified (prompt box + @ picker categories + sidebar status phrases live on dashboard.bex.co/agents with real data).

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on             |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------------------- |
| t001 | Prompt-box create surface: /agents main pane = composer with in-input toolbar; drop visible form fields | 60m | —                      — **DONE** |
| t002 | `@` mention picker: category listbox → typed token (@repos:/@sessions:) → fuzzy list + readiness preview | 90m | t001                   — **DONE** |
| t003 | Configuration popover: agent/model/endpoint/egress relocated from Advanced; typed error anchoring        | 45m | t001                   — **DONE** |
| t004 | Sidebar polish: Recent search + More/view-all, human status phrases + GitHub PR links, New-session shortcut | 60m | —                      — **DONE** |
| t005 | Navigation integrity: /agents deep-linkable, standalone list reachable via More, m44 session view intact | 30m | t001, t004             — **DONE** |
| t006 | Render parity                                                                                           | 30m | t002, t003, t005       — **DONE** |
| t007 | Simplify                                                                                                | 20m | t006                   — **DONE** |
| t008 | Test coverage                                                                                           | 60m | t006                   — **DONE** |
| t009 | Closeout                                                                                                | 10m | t008                   — **DONE** |

## Definition of done

- The `/agents` main pane is a single prompt box (large task textarea) with a slim in-input toolbar — `@` mention button, Configuration popover trigger, Send — and NO visible repo/branch/agent/Advanced form fields; the sessions list lives in the m44 `SessionSidebar` (the page reads like Devin's org home).
- Typing `@` in the composer opens a category listbox (v1: **Repositories**, **Sessions**); selecting a category inserts a typed token (`@repos:` / `@sessions:`) and swaps to that category's fuzzy-filtered item list (name + owner subtitle) with arrow/Enter keyboard navigation and, for repos, a readiness preview footer (GitHub-App-connected / default branch). Picking an item embeds a mention chip; the chip's repo becomes the created session's repo.
- Creating with no `@repo` mention nudges inline at the `@` button ("pick a repository") instead of submitting; branch auto-derives as `bex-agent/<slug>` from the task (editable in Configuration); `bex-agent/*` validation still enforced.
- The Configuration popover round-trips agent (claude/gemini/codex), model, modelEndpoint, and egress allowlist, and anchors the typed create errors (`egress {reason,entry}`, model-endpoint) exactly as the old Advanced section did; 503 still renders the house callout.
- Sidebar: the Recent header has a Search affordance and a More/view-all action (reaching the standalone list); each session row shows a human status phrase ("PR is ready" / "Working…" / "Failed" / "Canceled") derived from phase + PR presence, with the PR number linking directly to GitHub; a keyboard shortcut focuses/opens New session; the sidebar remains scoped to `/agents*` routes only.
- `/agents` and `/agents/$id` deep links keep working; the m44 session view is unchanged.
- `yarn typecheck && yarn lint && yarn test` green; the flow is render-verified in the local dev loop (`yarn dev:local` + the `local-bex` stub).

## Source + Goal linkage

- **Source:** user directive 2026-08-05 after live Playwright research of the user's own Devin org (app.devin.ai — org home, a session page, the wiki page); evidence: `.playwright-mcp/devin-{home,at-mention,at-repos,session-sidebar,wiki-sidebar}.png`. Follow-on to `w3/done/m44` (the full-page Devin-style chat, shipped + prod-verified). Companion backend gaps filed as `w3/014` (PR diff stats) and `w3/015` (transcript part timestamps).
- **Goal linkage:** ADR008 pillar 5 + [docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) § D9 (the product surface); feeds ADR048 mobile — this composer IS the mobile assign surface.
- **Expected outcome:** assigning work to an agent feels like Devin — one prompt box, `@bex-co/repo` to scope it, a compact Configuration popover — instead of a settings form; the sidebar reads sessions at a glance ("PR is ready" → the PR).
- **Why now:** m44 shipped the Devin-grade session view, making the form-style create page the visibly weakest surface; the research is fresh with pixel evidence; the local dev loop makes it iterable without prod deploys. **Render parity INCLUDED** — user/tenant-facing dashboard surface: parity = UX comparison vs Devin (bex extension) + confirming no REST/GraphQL/MCP drift (the create input shape is unchanged; the UI just gathers it differently).

## Research findings (encoded from the live Devin walk, 2026-08-05)

- **New Session screen** is JUST a chat input: one large textbox ("Ask Devin to build features…"), toolbar INSIDE the input (`@` attach/mention, Configuration popover, mode selector, Send + more-send-options). No form fields anywhere — context arrives via `@` mentions or Configuration.
- **`@` picker** is two-level: a category listbox (Repositories / Files / Skills / Devin Sessions / Macros / Playbooks / Secrets) → selecting inserts a typed token (`@repos:`) and swaps to that category's items (name + owner subtitle), fuzzy-filtered as you type (verified: `@repos:tianpan` matched `android-tianpanco-release`), with a readiness preview footer ("Not set up on Devin's machine — Indexing disabled"). Picking embeds a chip that scopes the session.
- **Sidebar** = stable top nav + a FEATURE-SCOPED middle section: "Recent" sessions (own Search + More buttons; rows = title + human status phrase, PR number as a direct GitHub link) appears only on the sessions feature; on Wiki the middle section is that feature's own context (verified: Recent absent). "New session" carries a keyboard shortcut (O).
