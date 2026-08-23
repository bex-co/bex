# w5 · m78 — Agent sessions dashboard: QA bug fixes from dev-5 walk

**Worker:** worker5 **Goal:** the three product bugs surfaced by the 2026-08-22 Playwright QA pass on `dev-5` `/agents` are closed — chat-only sessions are not blocked by a GitHub banner, Archived navigation is not duplicated, and a steering redispatch shows a provisioning/wait state instead of a false "stream unavailable" error **Status:** done

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Gate the GitHub empty callout to repo-backed session creation only                     | 30m | — — **DONE**          |
| t002 | Remove duplicate Archived navigation (sidebar vs list tabs)                            | 25m | — — **DONE**          |
| t003 | Steering redispatch: show provisioning/wait, not "stream unavailable"                | 35m | — — **DONE**          |
| t004 | Simplify — `/simplify` over the milestone's changed code                               | 20m | t001–t003 — **DONE**  |
| t005 | Test coverage — composer callout, nav dedup, steering state priority                  | 35m | t001–t003 — **DONE**  |
| t006 | Closeout — verify DoD, sync status, move to done/                                     | 15m | t005 — **DONE**       |

## Definition of done

On a live `dev-5` dashboard (`http://localhost:50050/agents`): a signed-in user with no GitHub repos connected can start a **chat-only** Claude session without the "Connect GitHub to start" banner blocking the composer; the `/agents` list shows **one** Archived entry point (not sidebar + tabs); after steering a session into redispatch, the conversation column shows a provisioning/waiting state until the stream is attachable — not "The conversation stream is unavailable right now." Unit tests cover the three fixes; dashboard suite green.

## Source + Goal linkage

- **Source:** Playwright QA on local `dev-5` agent sessions, 2026-08-22 (session transcript + `.playwright-mcp/qa-agents-*.png`); env gaps (502 model proxy, OpenBao k8s auth, sandbox quota) were repaired out of band — this milestone tracks **product** bugs only.
- **Goal linkage:** pillar 5 / ADR047 D9 dashboard surface (`w1/m64`, `w3/m44`); closes UX defects that block or mislead on the primary agent-session inner loop.
- **Expected outcome:** local and prod `/agents` QA no longer flags these three issues; chat-only sessions match capabilities (`modelKeyReady` without `gitHubConnected`).
- **Why now:** m77 hot-path work shipped; the QA pass is the first structured regression list against the live dev-5 overlay — fix before the next agent-session milestone batch.
- **Render parity omitted:** dashboard-only UX (no REST/GraphQL/MCP field or semantics change).
