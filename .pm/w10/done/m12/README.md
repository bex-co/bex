# w10 · m12 — Full bump of the Claude ACP agent to `@agentclientprotocol/claude-agent-acp`

**Worker:** worker10 **Goal:** The Claude agent profile runs the maintained, renamed adapter `@agentclientprotocol/claude-agent-acp` at its latest exact pin (0.94.0 as of 2026-10-22) instead of the deprecated `@zed-industries/claude-code-acp@0.16.2`. The driver's own `@agentclientprotocol/sdk` client moves to the matching major, and live sessions (new, resume, cancel) keep working end to end. **Status:** done (2026-10-30; DoD amended by user decision 2026-10-29, see t006)

## Tasks (in order)

| id   | title                                                                         | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Audit the 0.16.2 → 0.94.0 adapter and claude-agent-sdk 0.2.44 → 0.3.x changes — **DONE** | 45m | —          |
| t002 | Swap the Claude release profile to `claude-agent-acp` (package + executable) — **DONE**  | 45m | t001       |
| t003 | Bump the driver's `@agentclientprotocol/sdk` 0.14.1 → 2.x — **DONE**                     | 1h  | t001       |
| t004 | Session continuity across the bump: transcript flush, state paths, resume — **DONE**     | 45m | t002       |
| t005 | Docs and comments sweep for the rename — **DONE**                                        | 20m | t002       |
| t006 | Build the agent image and run a live Claude turn on `dev-10` — **DONE**                  | 1h  | t003, t004 |
| t007 | Simplify — **DONE**                                                                      | 20m | t005, t006 |
| t008 | Test coverage — **DONE**                                                                 | 30m | t007       |
| t009 | Closeout — **DONE**                                                                      | 10m | t008       |

## Definition of done

- `lego/backend/internal/agentsession/agent-profiles.json` pins the Claude profile to `@agentclientprotocol/claude-agent-acp@<exact latest>` with `executable: /usr/local/bin/claude-agent-acp`. `git grep claude-code-acp` returns only historical `.pm/**/done/` records and the intentional `@latest`-rejection case, if one is kept.
- `lego/agent-image/driver/package.json` pins `@agentclientprotocol/sdk` to the 2.x major. `npm test` and `npm run typecheck` pass in the driver.
- The agent image builds, and `dist/install-profiles.js` verifies that the new executable belongs to the pinned package at that version.
- `cd lego/backend && go test ./internal/agentsession/... ./internal/agentsessions/...` and `make lint` pass.
- A real Claude turn on `dev-10` shows a tool call, a plan, a diff and terminal output as typed parts. Cancel stops the adapter's process group. Resume of that session (`loadSession`) restores the transcript. Results are recorded in t006. If no model key is available, t006 records the blocker and the milestone stays open (it does not close on code alone).

**DoD amendment (2026-10-29).** **User decision 2026-10-29:** the DeepSeek (OpenRouter platform route) production evidence is sufficient; the real-Claude-model run is waived.

## Source + Goal linkage

- **Source:** user request 2026-10-22 ("full bump"). npm marks `@zed-industries/claude-code-acp@0.16.2` deprecated: _"renamed to @agentclientprotocol/claude-agent-acp. Please migrate to continue receiving updates."_ The successor is at 0.94.0 (bin `claude-agent-acp`; deps `@agentclientprotocol/sdk@2.0.0`, `@anthropic-ai/claude-agent-sdk@0.3.303`).
- **Goal linkage:** ADR047 cloud coding-agent sessions (ADR008 AI-native pillar). Its D7 release-locked agent profiles must track maintained adapters.
- **Expected outcome:** Claude sessions run a supported adapter that still receives updates (newer Claude models, tool and permission fixes). The release profile, image build check and docs all name the same package.
- **Why now:** the pinned package is a dead end, frozen at 0.16.2 and roughly 78 minor versions behind its successor. Every month of drift widens the claude-agent-sdk gap (0.2.44 → 0.3.303) that t004's transcript and resume compatibility must bridge. codex-acp 1.4.0 already uses SDK `^1.3.0` against our 0.14.1 driver, so the driver SDK bump is due in any case.
- **Render parity omitted:** agent sessions are a bex-native surface with no render.com equivalent. This milestone changes no REST, GraphQL, MCP or dashboard contract, only the in-sandbox adapter and driver internals.
