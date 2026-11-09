# w7 · m296 — Agent sessions record who started them: a "started by me" filter and editable tags (Devin parity)

**Worker:** worker7 **Goal:** every agent session says who started it on every surface, the `/agents` list opens on the sessions you started, and a session's tags can be replaced after create, as in Devin. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `createdBy` on the session read model (REST, GraphQL, MCP); backfill automation-started rows | 1h | — |
| t002 | Creator filter on the session list: `mine` and repeatable `createdBy` on REST, GraphQL and MCP | 1h15m | t001 |
| t003 | Replace a session's tags after create (Devin `PUT …/tags`) on REST, GraphQL and MCP; amend ADR094 D3 | 1h15m | — |
| t004 | Dashboard `/agents`: "Started by me" default with an All toggle, creator on rows and header, tag editor | 1h30m | t002, t003 |
| t005 | Render parity (Devin): creator, filter and tags agree across REST, GraphQL, MCP and the dashboard | 30m | t004 |
| t006 | Simplify the code this milestone changed | 30m | t005 |
| t007 | Test coverage for creator, creator filter and tag replacement | 45m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- A session started from the dashboard, the REST API, MCP, an automation or an in-sandbox spawn returns `createdBy` on `GET /v1/agent-sessions/{id}`, GraphQL `agentSession` and MCP `get_agent_session`. The value is the starting subject; for an automation or a child session it is the person whose authority the session runs with. Rows from before ADR102's `created_by` column return it only where it can be derived exactly (automation-started rows), never guessed.
- `GET /v1/agent-sessions?mine=true` and `?createdBy=<subject>` (repeatable), GraphQL `agentSessions(mine:, createdBy:)` and MCP `list_agent_sessions` `mine` / `createdBy` return only matching sessions, paginate correctly, and use an index.
- `PUT /v1/agent-sessions/{id}/tags` with `{"tags": [...]}` replaces the whole tag set (Devin v3 semantics) under the existing tag limits, with the same authorization as the other session verbs. GraphQL and MCP expose the same operation, and ADR094 D3 no longer says tags are read-only.
- `/agents` opens on the non-archived sessions the viewer started (Devin's sidebar default), an "All" toggle shows everyone's, each row and the session header show who started it, and the header has a tag editor. Pending skeletons match the loaded layout at desktop and narrow-mobile widths.
- The backend real-DB suite, `make lint`, and dashboard `yarn lint` + `yarn test` pass, and the change is live on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w7` item 5 (2026-11-08), approved by the user on 2026-11-09 (`/pm for "Agent sessions record who started them…" for w7 and then loopx w7`). Devin evidence, checked 2026-11-08:
  - [Devin 2024 release notes, Dec 24](https://docs.devin.ai/release-notes/2024): "By default, sessions in your sidebar are filtered to non-archived sessions that you started."
  - [Devin v3 list sessions](https://docs.devin.ai/api-reference/v3/sessions/organizations-sessions): a `user_ids` filter and a `user_id` field on each session; `PUT …/sessions/{devin_id}/tags` "Replace all tags on a session."
- **Current bex state (main `9100f3bcc`):**
  - ADR102 D9 (w21/m279) already records the creator. Migration `0212_agent_session_created_by` adds `agent_sessions.created_by`, written at create from `core.SubjectFrom(ctx)` (`agentsessions/service.go:646`). Automations fire as their creator (`automation_scheduler.go:194`) and spawns act as the parent's creator (`spawn.go:119`).
  - The column is used only for spawn authority. `View` (`models.go:244`) has no `createdBy`, and `ListRequest` (`models.go:395`) has no creator filter.
  - Tags and title are "create-time … read-only afterwards" (`store/agentsessions.go:193`, ADR094 D3). No route changes them (`rest.go`).
  - The dashboard shows a starter only for automation sessions (`session-detail-header.tsx:479`). The automations list already has a `mine` filter on every surface (`automations.go:218`), which t002 mirrors.
- **Goal linkage:** cloud coding agents with Devin as the reference product (ADR047; ADR008 "AI-native").
- **Expected outcome:** in a workspace with several members, each member opens `/agents` on their own sessions and can re-tag them. API and MCP clients can list "my sessions" and see who started any session.
- **Why now:** automations (ADR101) and child sessions (ADR102) both shipped this week and multiply the sessions in a shared workspace list. The column already exists, so exposing it is cheap now. It is also the base that Devin-style personal Memory and per-user budgets would build on.
- **Render parity:** included (t005). This milestone changes REST, GraphQL, MCP and dashboard surfaces of agent sessions; the reference product is Devin (ADR047).
- **Out of scope:** renaming a session's title (file separately once Devin's behavior is verified), Devin's session folders, bulk archive, and personal Memory.
