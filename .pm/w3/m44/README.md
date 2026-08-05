# w3 · m44 — Agent-session UI: full-page Devin-style chat

**Worker:** worker3 **Goal:** turn `/agents/$agentSessionId` from a card-grid detail page into ONE full-page chat that looks and behaves like Devin's session view — conversation is the whole main pane with a docked composer, PR inline, evidence behind a panel, sessions in a sidebar. **Status:** t001–t010 **DONE** (implementation + parity + simplify + tests); full-page Devin-style chat rendered + verified in the local dev loop. A duplicate-transcript bug from a dev double-mount was fixed (collapseDoubledParts, with a regression test). **t011 closeout pending** ship/deploy.

## Tasks (in order)

| id   | title                                                                                              | est | depends_on                       |
| ---- | -------------------------------------------------------------------------------------------------- | --- | -------------------------------- |
| t001 | Full-page chat layout: full-height scrollable transcript + top header (title, PR badge #N, "…" menu) | 60m | —                                — **DONE** |
| t002 | Docked chat composer: SteeringComposer becomes the bottom input, keeping live-POST vs redispatch routing | 45m | t001                             — **DONE** |
| t003 | Message flow: user right-aligned bubbles + avatar, agent plain full-width prose; terminal status line | 45m | t001                             — **DONE** |
| t004 | "Worked for Ns" / "Thought for Ns" groups: duration label, collapsed, expand to a vertical-timeline of steps | 60m | t003                             — **DONE** |
| t005 | Inline PR card rendered within the conversation flow (title, `<repo>#N · +A −D · bot`, review action) | 45m | t001                             — **DONE** |
| t006 | Evidence relocation: side-panel toggle / "…" menu instead of a permanent right column               | 45m | t001                             — **DONE** |
| t007 | Sessions sidebar on the session view: "New session" + a Recent list (phase/status + PR ref); `/agents` still reachable | 60m | t001                             — **DONE** |
| t008 | Render parity                                                                                       | 30m | t002, t003, t004, t005, t006, t007 — **DONE** |
| t009 | Simplify                                                                                            | 20m | t008                             — **DONE** |
| t010 | Test coverage                                                                                       | 60m | t008                             — **DONE** |
| t011 | Closeout                                                                                            | 10m | t010                             |

## Definition of done

- `/agents/$agentSessionId` renders as a full-page chat: a full-height, scrollable conversation as the whole main pane, with the composer docked at the bottom (no fixed-height conversation card, no permanent PR/Evidence right column).
- User turns are right-aligned bubbles with an avatar; agent turns are plain full-width prose (markdown/inline-code/links, no agent bubble/avatar).
- Activity + reasoning collapse into "Worked for `<Ns>`" / "Thought for `<Ns>`" groups (collapsed by default, duration shown); expanding shows a vertical-timeline of the individual steps with a connector line.
- The draft PR renders as an inline card in the conversation; Evidence lives behind a side-panel toggle or the "…" menu; both are reachable, neither is a permanent side column.
- The bottom composer sends a follow-up: a live session POSTs a chat turn (useChat `sendMessage`); an idle/terminal session redispatches via steer/resume — with disabled-reasons preserved.
- A left sidebar lists sessions ("New session" + Recent with a phase/status line + PR ref) and switching sessions stays in the chat view; `/agents` remains reachable.
- A terminal status line ("… went to sleep" / session-ended dot) ends a settled transcript.
- Same-origin only; `yarn typecheck && yarn lint && yarn test` green; rendered-verified in the local dashboard loop (`yarn dev:local` + the `local-bex` agent-stream stub).

## Source + Goal linkage

- **Source:** user directive 2026-08-04 with a Devin session screenshot (app.devin.ai — "Fix typo in tianpan-v3") as the target; a follow-on to `w1/m64` (shipped + deployed: `/agents` list + composer, the detail page, the `useChat` conversation column on the m43 stream, the AI-SDK-v6 transport with per-reconnect attach-ticket re-mint, and the ChatGPT/beancount-style grouped-activity/plan/thought rendering — commits through `793f04db`).
- **Goal linkage:** ADR008 pillar 5 (cloud coding-agent sessions) + [docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) § D9 (the conversation surface); feeds ADR048 mobile.
- **Expected outcome:** a Devin-grade session product surface — the conversation is the experience, not a widget beside metadata cards.
- **Why now:** the conversation column + transport already shipped in `w1/m64`, so the remaining gap to a Devin-grade surface is this layout/UX restructure; the fast local dashboard loop added this session (`yarn dev:local` + the `local-bex` agent-stream stub) makes it iterable without prod deploys. **Render parity task INCLUDED** — this is a user/tenant-facing dashboard surface; parity here is UX comparison against Devin/Render's session view (agent sessions are a bex extension — note UX-parity gaps, not API drift; the underlying REST/GraphQL/MCP data is unchanged).

## Open decisions (defaults chosen; revisit if needed)

- **Durations for "Worked/Thought for Ns":** check whether the m43 v1 transcript carries per-part/step timestamps; if not, derive elapsed from stream-arrival timing first, and only add timing to the driver's emitted parts (a backend change) if the derived value is too coarse. Approximate is acceptable for v1 (resolved in t004).
- **PR + Evidence placement:** confirmed — PR moves inline into the conversation, Evidence behind a panel/menu toggle; drop the always-visible side cards.
