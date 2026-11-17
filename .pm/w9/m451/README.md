# w9 · m451 — Shell sessions appear in the workspace audit log as StartShellEvent and EndShellEvent

**Worker:** worker9 **Goal:** every SSH and dashboard Shell session into a service instance leaves Render's `StartShellEvent` and `EndShellEvent` rows, with Render's shell metadata, in the workspace audit log **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Decide and grant the gateway's audit write path | 30m | w9/m450/t002 |
| [t002](t002.md) | Native SSH records StartShellEvent and EndShellEvent | 1h | t001 |
| [t003](t003.md) | The web shell records StartShellEvent and EndShellEvent with the browser's user agent | 45m | t001 |
| [t004](t004.md) | REST, GraphQL, CSV and the dashboard render the shell events | 45m | t002, t003 |
| [t005](t005.md) | Production check: an SSH and a web shell session each leave a start and an end row | 30m | t004 |
| [t006](t006.md) | Render parity | 30m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006 |
| [t009](t009.md) | Closeout | 15m | t007, t008 |

## Definition of done

- On production, as the QA user on a paid service in a Pro workspace, `bex ssh srv-…` and the dashboard Shell each produce, within two minutes, a `StartShellEvent` row when the session opens and an `EndShellEvent` row when it closes, on REST `GET /v1/owners/{ownerId}/audit-logs`, GraphQL and the CSV export, with `metadata.service` (the `srv-` id), `metadata.instance`, `metadata.source_ip` and, for the web shell, `metadata.user_agent`.
- A session the gateway refuses after identifying the caller records `StartShellEvent` with `status: error`; an unidentified connection records nothing.
- No terminal content, command or environment reaches either row (ADR035's content-free boundary, tested).
- Agent-session (`ags-…`) SSH records neither event.
- ADR035 records the gateway's audit write path and its residual.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w9` (2026-11-16), filed by `/pmx them all to w9`. Evidence:
  - [render.com/docs/audit-logs](https://render.com/docs/audit-logs) (read 2026-11-16): `StartShellEvent` (SSH session started) and `EndShellEvent` (SSH session ended), from the CLI or the service's Shell page; shell metadata "can include `service`, `instance`, `source_ip`, and `user_agent`". Render's changelog added `EndShellEvent` on 2026-03-25. Both names are in the pinned enum.
  - bex: native SSH (`lego/backend/internal/sshgateway/nativessh/server.go`) and the web shell (`lego/backend/internal/sshgateway/webshell/websocket.go`) write only `ssh_sessions` (`lego/backend/internal/store/sshsessions.go`), which no tenant surface reads.
  - [w4/m321](../../w4/done/m321/README.md) t006 asked to "Decide `StartShellEvent` and `EndShellEvent` for SSH and the web shell (ADR035) … Record a decision or a follow-up for each"; its result records neither.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); a workspace admin can see who opened a shell on a production instance, as on Render.
- **Expected outcome:** shell access, the strongest human access path into a running instance, is visible in the workspace audit log and its export.
- **Why now:** w4/m321's required decision was skipped, and shell access has no tenant-visible audit at all; depends on w9/m450's `source_ip`/`user_agent` columns (t001 depends on `w9/m450/t002`).
- **Decision state:** all `/pm-brainstorm` **recommendations**, not user decisions: (1) the gateway write path is an INSERT-only grant on `audit_events` (t001; alternative: merge `ssh_sessions` at read time); (2) native SSH carries no user agent (Render: "when available"); (3) a refused session from an identified caller records `status: error`; (4) agent-session SSH is a cloud-agent surface whose reference product is Devin, so it records neither Render event.
- **Render parity task:** included. REST, GraphQL, the CSV export and the dashboard labels change; MCP has no audit-log tool on Render or bex.
