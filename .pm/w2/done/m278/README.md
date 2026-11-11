# w2 · m278 — Idle sandboxes suspend and wake on exec, as on Render

**Worker:** worker2 **Goal:** a sandbox with no activity for its idle timeout (Render's default 900 s; `--idle-timeout` 300–86400, 0 disables) suspends: it stops billing compute while keeping its filesystem and state, and the next exec or file call resumes it transparently. **Status:** done — closed 2026-11-11: walked on production (t011); memory/process survival moves to w2/m322 (D1=A)

## Tasks (in order)

| id   | title                                                                   | est  | depends_on       |
| ---- | ----------------------------------------------------------------------- | ---- | ---------------- |
| t001 | Design: activity signal, suspend/resume mechanism on OpenSandbox, ADR — **DONE**   | 1h   | —                |
| t002 | Track sandbox activity (exec, file, connect) per sandbox — **DONE**                | 1.5h | t001             |
| t003 | Suspend after the idle timeout; resume on exec/file/connect — **DONE**             | 2h   | t002             |
| t004 | Honor `idleTimeoutSeconds` on create and report it (REST/GraphQL/MCP) — **DONE**   | 45m  | t003             |
| t005 | Billing stops while suspended; status `suspending`/`suspended`/`resuming` — **DONE** | 45m  | t003             |
| t006 | Render parity — **DONE**                                                           | 15m  | t004, t005       |
| t007 | Simplify — **DONE**                                                                | 15m  | t006             |
| t008 | Test coverage — **DONE**                                                           | 30m  | t006             |
| t010 | A resume restarts the runtime timeout; a suspended sandbox stays resumable for 30 days — **DONE** | 1h | t005 |
| t011 | Production walk: an idle sandbox suspends after its timeout and an exec resumes it with its files intact — **DONE** | 30m | t010 |
| t009 | Closeout — **DONE**                                                                | 10m  | t007, t008, t010, t011 |

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

## Work log — 2026-11-08 (`/loopx w2`)

- **Design (t001):** recorded in ADR014 § D4 and ADR042 D5.
  - **Render:** idle means no command execution or file transfer in progress, with a 900 s default; resume brings back files, memory and processes (a Firecracker snapshot); the timeout restarts on resume; a suspended sandbox is resumable for 30 days.
  - **bex activity signal:** bex-api's own exec and file paths (`authorizeExecTarget`, `authorizeFileTarget`), with a 60 s heartbeat while streams are open. Recorded in `sandbox_activity` (migration 0208).
  - **Suspend mechanism:** OpenSandbox `POST /pause` and `/resume` (rootfs commit plus pod recreate).
  - **What survives a suspension:** files, the id, plan, network policy, timeout and idle timeout. Memory, processes and `/tmp` do not.
  - Agent-session sandboxes are excluded.
- **Suspending (t002–t005):**
  - An `IdleSuspender` runs every 30 s with a single-statement CAS claim, so only one replica pauses a given sandbox.
  - Billing stops at `suspending`.
  - An exec or file call on a suspended sandbox resumes it and waits (REST up to 3 min, MCP 20 s). A slower resume answers `503 SANDBOX_RESUMING`; a failed one answers `SANDBOX_RESUME_FAILED`.
  - `idleTimeoutSeconds` defaults to 900; 0 disables it; 300–86400 is honoured; anything else is `400 SANDBOX_IDLE_TIMEOUT_INVALID`. It is reported on REST, GraphQL and MCP, and `SANDBOX_IDLE_TIMEOUT_UNSUPPORTED` is removed.
  - Explicit pause or resume in the wrong phase answers `409 SANDBOX_LIFECYCLE_CONFLICT`.
  - OpenSandbox `Pausing` now maps to `suspending` and `Allocated` to `creating` (both were `errored`).
- **Simplify (t007):** fixed a claim-wait bug and bounded the heartbeat writes; the claim is now one SQL statement; removed a duplicate status mapping.
- **Tests (t008):**
  - A stateful fake OpenSandbox covers the whole flow, the claim race, a stale claim, a failed resume and the heartbeat.
  - Real-DB tests cover the activity claim (exactly one of 8 wins) and confirm the meter stops at `suspending`.
  - `-race`, `make lint` and the real-DB sandbox/store/api suites pass.
- **Divergence (in ADR018) needing the user's sign-off:** memory, processes and `/tmp` do not survive a suspension (Render keeps them). Closing it needs memory checkpointing (`runsc checkpoint`/CRIU; the ADR042 D5 watch item). The workaround is `--idle-timeout 0`.
- **Implementable gap, now t010:** the timeout restart on resume and the 30-day resumable window.

## Blocked (2026-11-08)

t001–t008 and t010 are shipped (`be753e48d`, `9cb1c257a`). Two gates remain:

1. **User sign-off** on the one remaining divergence: memory, running processes and `/tmp` do not survive an idle suspension, while Render keeps them (ADR018 § Sandbox create flags; closing it needs `runsc checkpoint`/CRIU, the ADR042 D5 watch item; the workaround is `--idle-timeout 0`). Accept it as a recorded divergence, or fund memory checkpointing as a new milestone. Run `/unblock w2`.
2. **Production deploy** of `9cb1c257a` (starved, w6/m156). Then walk it with the QA user: create a sandbox with `--idle-timeout 300`; after 5 min it reports `suspended`; `bex sandboxes exec` resumes it and runs the command; a file written before the suspension is still there.

## Unblock — 2026-11-11 (`/unblock w2`)

- **D1=A (user decision, 2026-11-11):** fund memory and process checkpointing, so that an idle-suspended sandbox keeps its memory and running processes as Render's does. It is a new milestone, [w2/m322](../../blocked/m322/README.md). m278 closes on what it shipped: filesystem-preserving suspension, after the production walk.
  - **Render basis:** an idle sandbox "resumes when your agent sends its next command, with its files, memory, and running processes intact", using a runtime snapshot by default ([Render Sandboxes](https://render.com/sandboxes), [creating & configuring](https://render.com/docs/sandboxes-creating), [snapshots](https://render.com/docs/sandboxes-snapshots)).
- **Gates checked (2026-11-11):**
  - **Deploy:** `9cb1c257a` is live; the production deploy is current through `0a59e9343`.
  - **QA login:** `scripts/qa-login.sh` signs in on production again (rc 0), then logged out. The "no session established" recorded on w32/m284 no longer reproduces.
- **Next (worker):**
  1. t011, the production walk.
  2. t009, closeout. At closeout, record in ADR018's sandbox idle row that memory/processes/`/tmp` do not yet survive suspension: a **temporary** difference until w2/m322 ships, not a signed-off one.
- **Downstream:** w32/m285 waits on this decision (`--kind runtime`). Its board belongs to w32: run `/unblock w32`. With D1=A, runtime snapshots are planned (m322), not refused.
