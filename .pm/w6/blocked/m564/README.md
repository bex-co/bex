# w6 · m564 — Opening a closed agent session in Zed starts its sandbox again

**Worker:** worker6 **Goal:** a session whose sandbox closed after its idle timeout can still be opened in Zed: the open starts a fresh sandbox with the session's work, as a Devin session wakes when you come back to it, instead of the dashboard hiding the button. **Status:** blocked (t001–t003, t005–t007 done; t004 waits for production)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Decide how a closed session wakes for an editor, and record it in ADR054](done/t001.md) — **DONE** | 45m | — |
| t002 | [`resume` on a closed session boots a sandbox for editing, from its kept disk or its branch](done/t002.md) — **DONE** | 2h30m | t001 |
| t003 | [The dashboard offers Open in Zed on a closed session and shows it starting](done/t003.md) — **DONE** | 1h30m | t002 |
| t004 | [Production check: a session closed at its idle timeout opens in Zed with its work](t004.md) | 45m | t003 |
| t005 | [Render parity (Devin for agent sessions)](done/t005.md) — **DONE** | 30m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- On a completed session with no live sandbox, the dashboard's Open in Zed starts a sandbox and then offers the `zed://ssh/…/workspace` link and `ssh` command once it is reachable. It does not hide the button.
- The started sandbox's `/workspace` holds the session's kept disk when retention kept one (w10/m491), else a fresh checkout of the session's `bex-agent/*` branch. It has the session's environment, secrets and egress, and runs no agent turn.
- REST, GraphQL and MCP `resume` start the same sandbox, with the same result and errors.
- The sandbox then follows the idle rule of m563: an open editor keeps it, and it closes at the idle timeout after the last SSH disconnect.
- Verified on production on a session closed by its idle timeout.

## Source + Goal linkage

- **Source:** the recommendation the user approved on 2026-11-22 ("其他的按照你推荐的修"): with a 5-minute close (m563), a person who opens Zed after reading a result more often finds the sandbox gone. Today `SSHResolver.ResolveSSHSession` refuses with `AGENT_SESSION_NOT_ATTACHABLE` "agent session has no live sandbox to open" (`lego/backend/internal/agentsessions/ssh.go`), the dashboard offers the button only while a sandbox is live (`OpenInZedButton`, `dashboard/src/features/agent-sessions/components/session-detail-header.tsx`), and `Resume` (`lego/backend/internal/agentsessions/service.go:1683`) only wakes a sandbox that still exists.
- **Evidence (production, 2026-11-22):** of the Zed opens after a turn ended with a known delay, 20 of 25 came within 5 minutes, 2 within 15, 1 within 30 and 2 within 2 hours, so a few opens per week would meet a closed sandbox.
- **Reference product:** Devin. A sleeping session wakes when you send it a message ([docs.devin.ai/admin/billing/usage](https://docs.devin.ai/admin/billing/usage), read 2026-11-22); bex already does that through `Steer`. t001 checks what Devin does when a person opens a sleeping session's editor and records it. GitHub Codespaces likewise starts a stopped codespace when you connect to it.
- **Goal linkage:** ADR047 cloud coding-agent sessions; ADR054 Open in Zed.
- **Expected outcome:** the short idle timeout costs a person a sandbox start (seconds) instead of the editor.
- **Why now:** m563 shortens the close from 2 hours to 5 minutes in the same queue.
- **Render parity:** included, against Devin: `resume` on REST, GraphQL, MCP and the dashboard.

## Parked (2026-11-23)

`e0754f8c23` ships t001–t003 and t005–t007: `resume` (REST `resumeSessionId`, GraphQL `resumeAgentSession`, MCP `resume_agent_session`) starts a closed session's sandbox from its kept disk, else its branch, with no agent turn; the session reads `resuming` until the driver reports its checkout `ready`, then its finished phase with the sandbox and `sshAddress`; the sandbox closes on a person's idle timeout; the dashboard's Connect menu on a closed session offers Start the sandbox ([ADR054 D8](../../../../docs/ADR054-open-in-zed.md#d8--a-closed-session-wakes-for-an-editor-w6m564-2026-11-22)). t004 needs that deploy: the gate is the pin carrying `e0754f8c23`. When it clears, sign in as the QA user (`bash scripts/qa-login.sh`; its account needs an SSH public key whose private half this machine holds, else that is the gate), let a session in the `bex` workspace close at its 5-minute idle timeout, start it from Open in Zed, connect with `ssh ags-…@ssh.bex.co`, check `/workspace` holds the branch checkout (or the kept disk), disconnect and see it close 5 minutes later; record the close, start, connect and second close times here, then close with t008.
