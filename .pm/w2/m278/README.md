# w2 · m278 — Idle sandboxes suspend and wake on exec, as on Render

**Worker:** worker2 **Goal:** a sandbox with no activity for its idle timeout (Render's default 900 s; `--idle-timeout` 300–86400, 0 disables) suspends: it stops billing compute while keeping its filesystem and state, and the next exec or file call resumes it transparently. **Status:** todo

## Tasks (in order)

| id   | title                                                                   | est  | depends_on       |
| ---- | ----------------------------------------------------------------------- | ---- | ---------------- |
| t001 | Design: activity signal, suspend/resume mechanism on OpenSandbox, ADR   | 1h   | —                |
| t002 | Track sandbox activity (exec, file, connect) per sandbox                | 1.5h | t001             |
| t003 | Suspend after the idle timeout; resume on exec/file/connect             | 2h   | t002             |
| t004 | Honor `idleTimeoutSeconds` on create and report it (REST/GraphQL/MCP)   | 45m  | t003             |
| t005 | Billing stops while suspended; status `suspending`/`suspended`/`resuming` | 45m  | t003             |
| t006 | Render parity                                                           | 15m  | t004, t005       |
| t007 | Simplify                                                                | 15m  | t006             |
| t008 | Test coverage                                                           | 30m  | t006             |
| t009 | Closeout                                                                | 10m  | t007, t008       |

## Definition of done

- With no `--idle-timeout`, a sandbox idle for 900 s goes `suspending` → `suspended` and stops accruing `sandbox_compute_seconds`.
- `bex sandboxes exec` on a suspended sandbox resumes it (`resuming` → `running`) and runs the command. Files written before the suspension are still there, and so are running processes, if the substrate can preserve them. If it cannot, ADR records what is preserved, and that is checked against Render.
- `--idle-timeout 300` suspends after 5 min; `--idle-timeout 0` never idle-suspends. Detail shows the value on REST, GraphQL and MCP. The `SANDBOX_IDLE_TIMEOUT_UNSUPPORTED` refusal is gone.
- ADR018's sandbox idle row is ✅.

## Source + Goal linkage

- **Source:** w2/m263 t001 (2026-11-05). The Render CLI v2.31+ sends `idleTimeoutSeconds`, and Render's pinned OpenAPI defaults it to 900 s. bex refuses positive values because OpenSandbox has no activity signal or wake-on-exec (ADR042 D5, ADR014 D4). User decision D2=A (2026-11-07): align with Render.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a forgotten sandbox stops costing money after 15 minutes idle, as it does on Render, and work resumes on the next command.
- **Why now:** D3=A lets paid sandboxes run without a timeout. Idle suspension is what keeps that from turning orphaned sandboxes into runaway cost (the w5/m99 incident: 403 h, $88.50).
- **Render parity included:** REST/GraphQL/MCP/CLI behavior.
