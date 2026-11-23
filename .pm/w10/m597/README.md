# w10 · m597 — `ssh ags-…@ssh.bex.co` into a session whose sandbox closed wakes it and opens the shell, as SSH into a sleeping Devin session does

**Worker:** worker10 **Goal:** a member who runs a session's copied `ssh` command after its sandbox closed gets a shell, not a banner telling them to come back. The gateway asks bex-api to start the sandbox as the key's owner (the same `Service.Resume` the dashboard's Connect > Start the sandbox calls, ADR054 D8), says so while it starts, and opens the shell once it is up. A caller who may view the session but not start it keeps w10/043's banner, with the reason when admission refused. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: where the wake runs (handshake or first channel), the verb's request and answer, the wait and its ceiling, what each refusal tells the client, what the dashboard and `sshAddress` offer on a closed session; ADR054 D7 and D8, ADR035 and ADR018 amended | 45m | — |
| [t002](t002.md) | bex-api: a signed gateway verb that runs `Service.Resume` as the SSH key's owner | 1h30m | t001 |
| [t003](t003.md) | Gateway: admit a wakeable login, hold its channels while the start settles with progress on stderr, then open the shell; refusals keep the banner | 2h | t002 |
| [t004](t004.md) | Surfaces: what a closed session offers for SSH (`sshAddress`, the dashboard's Connect menu), per t001 | 1h | t001 |
| [t005](t005.md) | Security review of the wake verb and the held login, in the gateway-verb review chain | 1h | t002, t003 |
| [t006](t006.md) | Live check: plain `ssh` into a closed session in `tian-personal` on production, session deleted afterwards | 45m | t003, t004, t005 |
| [t007](t007.md) | Render parity (Devin) | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 8h45m**.

## Definition of done

1. **A closed session wakes.** `ssh ags-…@ssh.bex.co` into a `completed` or `failed`, unarchived session whose sandbox closed, by a caller who may operate it and holds a fresh `can_create`, starts its sandbox through `Service.Resume`. The client is told on stderr that the sandbox is starting, and the shell opens in the started sandbox once the session settles. Zed's and `sftp`'s channels on such a connection wait for the same settle. A login that finds the sandbox already starting (`resuming`) waits for it instead of being told to come back.
2. **Refusals keep their banner.** A caller who may view but not resume the session (no `can_operate`, no fresh `can_create`, or a bex API key whose grant does not cover starting it) gets w10/043's banner. A workspace admission refuses (payment method, billing, the live-sandbox cap) gets the banner with admission's reason. Canceled and archived sessions keep their banners. A caller who may not view the session, a missing session and a key without sandbox execution keep the bare `Permission denied (publickey)`. A workspace login requirement (two-factor, Google sign-in) is still checked, and refused, before anything starts.
3. **A start that ends without a sandbox says so.** When the start fails, or the session settles with no sandbox, or the start's own ceiling passes, the waiting channel prints why and exits non-zero, the connection closes, and nothing is left `resuming`.
4. **Bounded.** The wake runs only after the client proved possession of the key and the login requirements passed. The wait holds a session-limiter slot as an open shell does and ends with the connection, the start's ceiling or a drain. The gateway counts wakes and their outcomes in `bex_ssh_gateway_*` metrics without identifying labels.
5. **Recorded.** ADR054 D7 and D8 name the new verb, its secret and what it lets a gateway credential do; ADR035 § Fail-closed behavior and the ADR054 D8 **Limits** paragraph no longer say SSH is refused on a closed session. A security review covers the verb as the key-grant and two-factor verbs were reviewed.
6. **Live.** On production, in `tian-personal`, plain `ssh` into a session whose sandbox closed opens a shell, and the session is deleted afterwards.

## Source + Goal linkage

- **Source:** inbox note [w10/049](../done/049.md), filed by w10/043's Devin check on 2026-11-23 and promoted by `/loopx w10` on 2026-11-23.
- **Triage evidence (2026-11-23, `9fdf7e0a11`):**
  - **The premise holds.** `SSHResolver.ResolveSSHSession` returns `errNoSandboxToOpen` for any session `openableSandbox` turns down (`lego/backend/internal/agentsessions/ssh.go:76-78`), and the gateway prints it as an auth banner counted `rejected_no_sandbox` (`lego/backend/internal/sshgateway/nativessh/server.go:217`, `toldRefusal` `:899`). Nothing on the SSH path starts a sandbox.
  - **The start already exists.** `Service.Resume` (`lego/backend/internal/agentsessions/editor.go:54`) needs `can_operate` and a fresh `can_create`, admits a fresh sandbox like a turn (`admitDispatch`), moves the session to `resuming` and starts the sandbox in the background (`runEditorStart`). The Completer settles it back to `completed`/`failed` with the sandbox, and a start with no sandbox converges within `editorStartBound` (16 minutes). ADR054 D8 (w6/m564) records it for Open in Zed and the dashboard's Connect > Start the sandbox.
  - **What the gateway lacks.** Its database role only reads `agent_sessions` (ADR054 D7), so it cannot start anything itself. Its two bex-api verbs only ask questions: `agentsession.InternalTwoFactorPath` and `agentsession.InternalKeyGrantPath`, signed with `BEX_SANDBOX_EXEC_SECRET` over bex-api's internal `:8091` (`lego/backend/cmd/api/main.go:1453-1458`, `lego/backend/cmd/ssh-gateway/main.go:200-205`).
  - **The handshake cannot hold the start.** Target authorization runs in `VerifiedPublicKeyCallback` under `HandshakeTimeout` (10 s, `server.go:247`), and a fresh sandbox takes longer. The login has to be admitted with a target that is not yet a pod and wait on its channels, where `serveConn` keeps a sticky target (`server.go:525-529`) and `serveMultiChannel` (`:687`) opens each channel.
  - **Devin's behavior.** "Sleeping Sessions Wake on Preview or SSH Traffic. Opening a preview URL or SSHing into a session whose machine has gone to sleep now wakes the machine and completes the request instead of showing 'not running'" ([Devin release notes](https://docs.devin.ai/release-notes/2026), 2026-09-25, read 2026-11-23). ADR018 records no signed-off divergence for it, so the wake ships.
  - **More than one sitting:** a new signed bex-api verb, the gateway's held login and its metrics, the closed session's SSH surfaces, three ADRs and a security review, gateway and real-DB tests and a production check.
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), ADR008 pillar 5), where Devin is the reference product. It finishes ADR054 D8's wake for the one entry point that still refuses: a plain `ssh`.
- **Expected outcome:** a copied `ssh ags-…@ssh.bex.co` works whenever the session can be started, the way Open in Zed and the Connect menu already do, so a member never has to send a message or open the dashboard just to get a shell.
- **Why now:** since w6/m563 a person's sandbox closes 5 minutes after its turn, so most `ssh` commands run later than that meet a closed sandbox. w10/043 made the refusal readable, and this is the step after it. The CLI's `bex agent ssh` (w6/169) can rely on the gateway's wake once this lands instead of calling `resume` itself. The milestone includes the Render parity task because it changes what the SSH surface does for agent sessions and may change `sshAddress` on REST, GraphQL, MCP and the dashboard (t004).
