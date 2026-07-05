# w2 · m3 — E2B-compatible sandboxes, idle-hibernated

**Worker:** worker2 **Goal:** Turn the opensandbox runtime's real pause/resume into hosted execution environments for agents, hibernated when idle (`sleep = free`). Delivers pillar 5. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Sandbox CRUD API mapping to opensandbox create/pause/resume/kill | 30m | — |
| t002 | E2B-compatible API shapes (so E2B SDK/tooling transfers) | 30m | t001 |
| t003 | Idle-hibernate: pause after inactivity, wake on connect | 30m | t001 |
| t004 | Expose sandbox-spawn as MCP verb + acceptance | 25m | t001, w2/m1/t001 |

## Definition of done

An agent spawns an E2B-shaped sandbox, executes in it, has it auto-hibernate when idle (occupying nothing → overcommit), and resumes on next use — reusing the opensandbox runtime's real pause/resume rather than a new mechanism.

## Source

`docs/vision.md` pillar 5 (E2B-compatible sandboxes); `CLAUDE.md` opensandbox runtime (`BEX_RUNTIME=opensandbox`, `BEX_OPENSANDBOX_URL`).
