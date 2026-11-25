# w4 · m632 — An agent told to set up bex follows `https://api.bex.co/agents.md`, as Render's agents follow `render.com/agents.md`

**Worker:** worker4 **Goal:** any coding agent given one URL can configure bex's MCP server and skills for itself and verify the connection, with a per-agent setup contract for Claude Code, Cursor, Codex, OpenCode, Pi, Claude Desktop and Codex Desktop. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Write `agents.md`, the seven setup contracts and `tasks.md` | 1h30m | w4/m629/t005, w4/m631/t005, w4/729 |
| [t002](t002.md) | bex-api serves them at `https://api.bex.co/agents.md` and `/agents/<tool>.md` | 45m | t001 |
| [t003](t003.md) | A test keeps the contracts true | 45m | t002 |
| [t004](t004.md) | Live: an agent sets bex up from `https://api.bex.co/agents.md` alone | 1h | t003 |
| [t005](t005.md) | File the bex.co pointer as a follow-up | 15m | t004 |
| [t006](t006.md) | Render parity | 25m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 5h35m**.

## Definition of done

`https://api.bex.co/agents.md`, the seven `https://api.bex.co/agents/<tool>.md` contracts and `/agents/tasks.md` answer 200 `text/markdown; charset=utf-8` without auth, mirror Render's procedure and field names with bex's values, and carry the configured MCP URL. A CI test fails when a contract names a skill, tool, plugin command or MCP URL bex does not have. A headless Claude Code given only the URL sets bex up and `list_workspaces` answers. The bex.co pointer is filed as a gated follow-up.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4`, 2026-11-24 (the aspect: how a coding agent connects to bex, as Render's agent docs describe), each item re-verified by an independent agent on 2026-11-24/25 before filing; filed by `/pmx them all to w4` at `origin/main` `50a5ddfdfc`.
- **Evidence (verified 2026-11-24/25):** `render.com/llms.txt` "Start here" routes setup to `render.com/agents.md` (six steps) and seven `render.com/agents/*.md` setup contracts plus `render.com/agents/tasks.md`, all served as `text/markdown`. `bex.co/agents.md` answers 404; bex.co documents Claude Code, Cursor and headless clients but not Codex, OpenCode, Pi or the desktop apps, and has no procedure. No board item or doc covers it.
- **Goal linkage:** ADR008 AI-native (agents are first-class users); Render parity of the agent setup path.
- **Expected outcome:** an agent sets bex up from one URL, as it sets Render up.
- **Why now:** it ties w4/m629 (OAuth), w4/m631 (plugin) and w4/729 (`npx skills add`) into one entry point, so it comes after them.
- **Render parity:** included. The proposal's recommendation, not a user decision: serve from `api.bex.co` because the dashboard's `/agents/*` is taken by cloud-agent sessions and bex.co's source is unavailable (w21/blocked/008); Render serves its files from its marketing host, and the bex.co pointer is filed as a gated follow-up (t005).
