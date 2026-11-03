# w9 · m170 — ADR095 phase 3 — agent-shared output files and the Files tab

**Worker:** worker9 **Goal:** the agent can share files back (`share_file`) and users download them from the conversation, the Files tab or the API. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `share_file` in the in-sandbox bex MCP + gateway route | 75m | — |
| t002 | Dashboard: `share_file` cards and the Files tab | 75m | t001 |
| t003 | Expose outputs to API callers | 30m | t001 |
| t004 | Render parity (Devin reference) | 25m | t002, t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- `share_file` stores a file from /workspace, /tmp or /attachments as an output attachment through the gateway (session-pod identity, no credential in the sandbox); path escapes, directories and oversize files are refused.
- The conversation shows a file card for each shared file; the Files tab lists inputs and outputs with downloads and a matching skeleton.
- REST/GraphQL list outputs per turn; `outcome.files` added if w21's `outcome` exists, otherwise the coordination is recorded.
- Live: a session asked to "write a CSV report and share it" produces a downloadable file in the dashboard.

## Source + Goal linkage

- **Source:** [ADR095](../../../docs/ADR095-agent-attachments.md) (accepted for implementation by user 2026-11-03: "/pm ADR095 to w9"); Devin gap list in [ADR094](../../../docs/ADR094-coding-agent-mcp.md).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 5 (cloud coding agents); Devin parity for agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)).
- **Expected outcome:** parity with Devin's `source: devin` attachments — reports, CSVs and screenshots come back without being committed.
- **Why now:** ADR095 D12 phase 3; depends on m168 storage.
- **Dependency:** m168; dashboard pieces reuse m169 components.
- **Parity:** included: REST/GraphQL/dashboard surfaces; Devin is the reference.
