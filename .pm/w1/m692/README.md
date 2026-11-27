# w1 · m692 — A service's Shell reconnects by itself when its connection drops or a deploy ends its instance, as Render's does

**Worker:** worker1 **Goal:** a service's Web Shell recovers by itself from a dropped connection or a stopped instance. Each try opens a fresh, re-authorized session. It ends only for the reasons Render's ends, with Render's status bar under the terminal. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's reconnect rule and status bar in `docs/render-artifacts/shell-page.md` | 30m | — |
| [t002](t002.md) | A reconnect policy in `ShellTerminal`, on for the service Shell only | 2h | t001 |
| [t003](t003.md) | What ends a session and what retries it | 1h | t002, w1/m691/t004 |
| [t004](t004.md) | Render's status bar: "Reconnecting…", the end lines and "Start a new shell" | 45m | t002 |
| [t005](t005.md) | Amend ADR035 § Fail-closed behavior and the ADR018 SSH row | 20m | t003 |
| [t006](t006.md) | Walk a deploy, a gateway restart and a hidden tab on dev-1, then production | 1h | t003, t004 |
| [t007](t007.md) | Render parity | 20m | t005, t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 6h50m**. Starts after [w1/m691](../m691/README.md): a reconnect to an instance that has gone follows m691's not-available rule.

## Definition of done

1. An open Shell whose instance a deploy replaces reconnects to a running instance by itself, without a click, and shows m691's toast.
2. A dropped connection (for example, the gateway pod restarting) reconnects by itself. The retries follow Render's rule: up to 5 tries, at min(1 s × 2ⁿ, 30 s), with typing resetting the count. A tab hidden at the drop reconnects when it is shown again.
3. The session ends for good, in Render's words, after:
   - an exit (`exit`, `exit 3`, but not 137 or 143);
   - a revoked access;
   - a suspension;
   - a plan or type that no longer allows a shell;
   - the fifth failed try.
4. Every try mints a new ticket, so authorization is checked again. The old stream still ends at its pod, as ADR035 says, and ADR035 and ADR018 say so.
5. The agent session's Terminal behaves as before.
6. Walked on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w1` (2026-11-26), item 2, verified before filing.
  - Render's public dashboard bundle was read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`). Find the code by the strings quoted here.
  - `ServiceShellStarter`'s socket `onclose` ends the session only when output was seen, the container is not gone, and the exit was clean: close code 1000 with no exit code, or an exit code other than 137 and 143. Every other close reconnects.
  - The retry: up to 5 tries (`lt=5`), at `Math.min(1e3*2**n, 3e4)` ms. Typing resets the count. A hidden tab (`document.visibilityState`) waits until it is shown. Each try calls the `startShell` mutation again.
  - "Container gone" means Render's patterns `cannot exec in a (deleted|stopped) state`, `cannot exec in a stopped container`, `task … not found`, `no running task found` and `container not created: not found`. These reconnect.
  - `index` holds the status bar: a spinner with "Reconnecting…", and the end lines "The shell disconnected." (after the last try), "The shell couldn't start.", "The shell exited." and "The shell exited with code {n}.", each beside a "Start a new shell" button.
  - No flag gates it.
- **bex today (checked 2026-11-26):**
  - `dashboard/src/common/components/shell-terminal.tsx` turns any close into "Session closed" (`:206-209`), and ends on any error frame (`:249-256`). Only the Reconnect button (`:279`) recovers.
  - The gateway ends a stream whose pod stopped (a deploy's old instance) as `instance_gone`, "the instance this session ran on stopped" (`lego/backend/internal/sshgateway/webshell/websocket.go:450-457`).
  - With auto-deploy, every push leaves each open Shell dead once its old instance stops.
  - `ShellTerminal` is shared with the agent session's Terminal (`dashboard/src/features/agent-sessions/components/session-terminal-sheet.tsx`), whose reference product is Devin, so the policy stays on the service Shell only.
- **Not a duplicate of:**
  - w4/m352, which keeps a session alive through a deploy until its old instance stops, and names the end.
  - w4/m352's end reasons, which this milestone keeps for the cases Render ends.
  - w1/m658, whose sandbox Shell is unbuilt and behind Render's `sandbox_shell` flag.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** deploys and gateway releases no longer end anyone's Shell. A user sees "Reconnecting…" and keeps working.
- **Why now:** every deploy of a service with an open Shell hits this today. It needs m691's always-named instance for the not-available hand-off.
- **Render parity task:** included (t007). The dashboard Shell changes, the gateway's end reasons are read differently, and ADR035 is amended. REST, GraphQL and MCP do not change.
