# w6 · m565 — Each agent sandbox's billing line names its real size and its session

**Worker:** worker6 **Goal:** a person reading the billing page can tell what each agent sandbox charge was: its real CPU and memory, and the session (repository and branch) it ran for. Today they read `starter · ghcr.io/bex-co/bex-agent-sandbox · <uuid>`. **Status:** blocked (t001, t002, t004–t006 done in `a8d6ea3b12`; t003's production check and t007 wait for production to carry it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [An agent sandbox is labeled with its template's size, not `starter`](done/t001.md) — **DONE** | 45m | — |
| t002 | [Every agent sandbox's row names its session, even after the session moved on](done/t002.md) — **DONE** | 45m | — |
| t003 | [Past rows read their real size from the recorded weight](t003.md) | 45m | t001 |
| t004 | [Render parity](done/t004.md) — **DONE** | 30m | t002, t003 |
| t005 | [Simplify](done/t005.md) — **DONE** | 20m | t004 |
| t006 | [Test coverage](done/t006.md) — **DONE** | 45m | t004 |
| t007 | [Closeout](t007.md) | 10m | t005, t006 |

## Definition of done

- A new agent sandbox on `agent-4c-8g` is labeled `4c-8g · ghcr.io/bex-co/bex-agent-sandbox`, and one on `agent` is labeled `2c-4g · …`, on the billing page and on REST, GraphQL and MCP usage reads.
- Every agent sandbox row that `agent_session_sandboxes` links to a session reads `<repo> (<branch>)` (or `agent session <id>` with no repository), whether or not the session still names that sandbox.
- November's rows in the `bex` workspace read their real size: on production, no agent sandbox row with a recorded weight of 2422 or 4845 milli says `starter`.
- Amounts are unchanged.

## Source + Goal linkage

- **Source:** the recommendation the user approved on 2026-11-22 ("其他的按照你推荐的修"), from reading the `bex` workspace's billing page with them.
  - `createAgentSandbox` stamps `plan := PlanStarter` whatever the template (`lego/backend/internal/sandbox/service.go:1257`), and `starter` is Render's 0.5 CPU / 512 MB plan (`planResources`, `lego/backend/internal/sandbox/models.go`). On production 237 of the `bex` workspace's agent sandboxes ran 4 CPU / 8 GiB (weight 4845) and 284 ran 2 CPU / 4 GiB (2422), all labeled `starter`.
  - `SandboxUsageMetadata` (`lego/backend/internal/store/resource_names.go:145`) names a sandbox from `agent_sessions.sandbox_id` and `agent_session_dispatches.previous_sandbox_id` only, so a sandbox reaped after its session moved on falls back to the plan · image label. `agent_session_sandboxes` links 301 of November's 355.
- **Goal linkage:** ADR023 usage metering and ADR030 pricing: a bill a customer can read and check ([ADR008](../../../docs/ADR008-vision.md)).
- **Expected outcome:** the billing page's agent sandbox lines say what ran and for which session, so a cost like November's $231 can be traced without a database query.
- **Why now:** the same page is where m563 and m566's savings will be checked, and today it hides which sessions cost what.
- **Render parity:** included. Render's billing page lists each sandbox charge by the sandbox; the label must be the same on REST, GraphQL, MCP and the dashboard.
