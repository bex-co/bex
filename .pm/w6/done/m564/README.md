# w6 · m564 — Opening a closed agent session in Zed starts its sandbox again

**Worker:** worker6 **Goal:** a session whose sandbox closed after its idle timeout can still be opened in Zed: the open starts a fresh sandbox with the session's work, as a Devin session wakes when you come back to it, instead of the dashboard hiding the button. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Decide how a closed session wakes for an editor, and record it in ADR054](done/t001.md) — **DONE** | 45m | — |
| t002 | [`resume` on a closed session boots a sandbox for editing, from its kept disk or its branch](done/t002.md) — **DONE** | 2h30m | t001 |
| t003 | [The dashboard offers Open in Zed on a closed session and shows it starting](done/t003.md) — **DONE** | 1h30m | t002 |
| t004 | [Production check: a session closed at its idle timeout opens in Zed with its work](done/t004.md) — **DONE** | 45m | t003, t009 |
| t009 | [The session page's conversation stream does not hold a started sandbox open](done/t009.md) — **DONE** | 1h | t003 |
| t005 | [Render parity (Devin for agent sessions)](done/t005.md) — **DONE** | 30m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007, t009 |

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

## Unblocked (2026-11-23)

The gate cleared by itself (verified evidence, no user decision): the newest pin on `origin/main`, `87ea4b0ee5` (`chore(deploy): pin platform images to ac882b5f557f`, 2026-11-23 11:00Z), carries `e0754f8c23` (`git merge-base --is-ancestor e0754f8c23 ac882b5f557f`), and m563's `b5cbb23493` with it, so production runs the 5-minute close this check relies on. Next: t004 as the QA user, as § Parked says, then t008.

## Production check (2026-11-23, t004)

Pin `ac882b5f557f`: every `bex-system` Deployment ran `bex-operator@sha256:f5d75a7c48d1…` and the dashboard `bex-dashboard@sha256:1eb4c5a69615…`, the digests that pin writes. As the QA user (`bash scripts/qa-login.sh`, logged out afterwards) in workspace `bex`: session `ags-dc22jkehv52s73bqmmt0` (disposable) on the private fixture `bex-co/bex-hello-go-live`, `claude` on `anthropic/claude-haiku-5.5`, whose one turn left `w6-m564-probe.txt` uncommitted on purpose; SSH with a temporary key on the QA user (`ssk-dc22j8mhv52s73bqmmq0`, deleted afterwards).

- **Close: 12:06:14Z.** The turn ended at 11:56:27Z with "the working tree has uncommitted changes". The idle close kept the disk (2.38 GB, kept until the next day), ended the sandbox, and the session lost its `sshAddress`. It came at 12:06Z, not 12:01Z, because a view of the session page at 12:00:12Z wrote an agent-attach row that moved the idle clock (t009).
- **Start: 12:06:45Z.** Connect on the closed session said "The sandbox closed after the session sat idle…" and offered Start the sandbox. Pressed, the session read `resuming` ("Starting the sandbox… The Zed link appears here once it is up."), bound a new sandbox at 12:06:48Z and settled at 12:06:54Z to `completed` with `sshAddress`. The menu then offered Open in Zed (`zed://ssh/ags-dc22jkehv52s73bqmmt0@ssh.bex.co/workspace`) and the `ssh` command. `turns` stayed 1: the driver ran with `BEX_AGENT_EDITOR=1`, status `ready`, and no turn.
- **Connect: 12:07:09Z.** `ssh ags-dc22jkehv52s73bqmmt0@ssh.bex.co` found `/workspace` on `bex-agent/ags-dc22jkehv52s73bqmmt0` from the kept disk: `w6-m564-probe.txt` read "w6-m564 probe kept across the idle close" and was still untracked. SFTP listed `/workspace`, as Zed's server install needs. Last SSH disconnect 12:07:26Z; the page's stream ended when the tab left at 12:07:36Z.
- **Second close: 12:13:56Z.** The save began at 12:12:41Z, 5 minutes after the last disconnect, kept the disk again and ended the sandbox.
- **Found: an open session page held the sandbox.** A third start with the page left open (settled 12:14:32Z, no SSH at all) was still up at 12:21:46Z, the Completer logging `deferring teardown, editor SSH open`: the page's conversation stream (agent-attach row `ssn-dc22s9hknr1s73a6bld0`, open from 12:14:30Z) counted as an editor, and would have held it for the 4-hour cap. It closed at 12:27Z, 5 minutes after the tab left at 12:22:15Z. t009 fixes the idle close's reader; the stream the page leaves open is w6/171.
- **Reproduce.** Session state from `GET https://api.bex.co/v1/agent-sessions/<id>` with the cookie jar `qa-login.sh` prints; the close and start lines from `kubectl -n bex-system logs` of both `bex-api` pods (`agent-session retention: keeping the disk`, `kept`, `deferring teardown, …`); the gateway rows read-only on `bex-db` with `SELECT id, started_at, ended_at, result FROM ssh_sessions WHERE service_id = '<id>'`.

## Parked again (2026-11-23)

`2d68de52fc` ships t009: the idle close reads only SSH logins (`ssh_sessions.transport`, migration `0339`), so the session page's conversation stream neither holds a started sandbox nor moves its idle clock. t004's run above passed with the page closed; its last check needs that deploy: the gate is the pin carrying `2d68de52fc`. When it clears, sign in as the QA user, let a session in the `bex` workspace close at its idle timeout, start it from Connect, leave the session page open, connect and disconnect over SSH, and see the sandbox close 5 minutes after the disconnect (both `bex-api` pods log no `deferring teardown, editor SSH open` after it). Record the times here, then close with t008. The page still leaves the stream itself open (w6/171), which no longer keeps the sandbox.

## Unblocked again (2026-11-23)

The gate cleared by itself (verified evidence, no user decision): the newest pin on `origin/main`, `cded300699` (`chore(deploy): pin platform images to 9fdf7e0a1171`, 2026-11-23 17:33Z), carries `2d68de52fc` (`git merge-base --is-ancestor 2d68de52fc 9fdf7e0a1171`), so production's idle close reads SSH logins only. Next: t004's last check as the QA user, as § Parked again says, then t008.

## Production check with the page open (2026-11-23, t004's last check)

Pin `9fdf7e0a1171`: `bex-api` and `bex-ssh-gateway` ran `bex-operator@sha256:df6e69bd31bf…`, the digest that pin writes. As the QA user (`bash scripts/qa-login.sh`, logged out afterwards) in workspace `bex`: session `ags-dc27prmurl5c7380qoo0` (disposable, deleted afterwards) on the private fixture `bex-co/bex-hello-go-live`, `claude` on `anthropic/claude-haiku-5.5`, whose one turn left `w6-m564-probe.txt` untracked on purpose; SSH with a temporary key on the QA user (`ssk-dc27pmedfl0s73a7e8og`, deleted afterwards).

- **Close: 17:57:35Z.** The turn ended at 17:52:25Z (`dirty=true`). Nobody opened the page, and the idle close began the save at 17:57:35Z, 5 minutes later (t009: no page view moved the clock), kept the disk (2.38 GB, until the next day) at 17:58:35Z and ended the sandbox; `sshAddress` was gone at 17:58:37Z.
- **Start: 17:59:00Z.** On the session page, Connect said "The sandbox closed after the session sat idle…" and offered Start the sandbox. Pressed, the session read `resuming` at 17:59:02Z and settled at 17:59:09Z (`the editor's start settled, the sandbox is up`) to `completed` with `sshAddress`; the menu offered Open in Zed and `ssh ags-dc27prmurl5c7380qoo0@ssh.bex.co`. `turns` stayed 1. The page stayed open from here until 18:07:50Z.
- **Connect: 17:59:21Z.** `ssh` found `/workspace` on `bex-agent/ags-dc27prmurl5c7380qoo0` from the kept disk, `w6-m564-probe.txt` untracked and reading "w6-m564 probe kept across the idle close (second run)". The Completer logged `deferring teardown, editor SSH open` while it was connected. Disconnect: 18:00:51Z.
- **Second close: 18:05:55Z, with the page open.** The save began 5 minutes after the disconnect, kept the disk again and ended the sandbox at 18:07:00Z; `sshAddress` was gone at 18:07:03Z. The page's conversation stream (`ssh_sessions` row `ssn-dc27tq1ku8ps73ct6c9g`, `transport = agent_attach`, open from 17:59:04Z) was still open through the save and ended at 18:07:21Z when the sandbox went, before the tab left. Neither `bex-api` pod logged `deferring teardown, editor SSH open` for the session after 18:00:51Z.
- **Reproduce.** As in § Production check above; the gateway rows read-only on the `bex-db` replica with `SELECT id, transport, started_at, ended_at, result FROM ssh_sessions WHERE service_id = '<id>'`.

The definition of done holds on production. The stream the page leaves open stays w6/171's.
