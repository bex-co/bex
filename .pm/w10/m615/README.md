# w10 · m615 — An agent session's page has a Terminal that opens a shell in its sandbox and wakes a closed one, as Devin's Terminal tab does

**Worker:** worker10 **Goal:** a member who may operate an agent session gets a shell in its sandbox from the session's page, with no local terminal and no registered SSH key, the way a service's Web Shell works. Opening it on a session whose sandbox closed starts the sandbox, says so while it starts, and opens the shell once it is up, as Devin's Terminal tab does ("opening Files or Terminal wakes Devin"). Canceled and archived sessions say why there is no Terminal. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: where the Terminal lives on the session page, the ticket and who wakes the sandbox, the wait and what the browser shows meanwhile, the API surfaces, what each refusal says; ADR047 or ADR054, ADR035 and the ADR018 ledger amended | 45m | — |
| [t002](t002.md) | bex-api: mint a Terminal ticket for an agent session, starting a closed one through `Service.Resume` as the caller | 1h30m | t001 |
| [t003](t003.md) | Gateway: the Web Shell opens an agent session's sandbox from its ticket, holds the WebSocket with progress while the sandbox starts, then bridges the shell | 2h | t002 |
| [t004](t004.md) | Dashboard: the session page's Terminal, its starting state and its refusals, in every locale, at 1280 px and 390 px | 2h | t001, t002 |
| [t005](t005.md) | Security review of the agent-session Terminal ticket and the held WebSocket, in the gateway review chain | 1h | t002, t003 |
| [t006](t006.md) | Live check: open the Terminal on a closed session in `tian-personal` on production, session deleted afterwards | 45m | t003, t004, t005 |
| [t007](t007.md) | Render parity (Devin) | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 9h45m**.

## Definition of done

1. **A Terminal on the session page.** A session that is neither canceled nor archived offers a Terminal on its page, placed where t001 decided after Devin's session workspace. Opening it on a session whose sandbox is up opens a shell in that sandbox (`/workspace`, the sandbox's user), over the gateway's Web Shell WebSocket with a short-lived ticket, as a service's Web Shell does. The private SSH key never enters the browser, and bex-api never gains `pods/exec`.
2. **A closed session wakes.** Opening it on a `completed` or `failed`, unarchived session whose sandbox closed, by a caller who may operate it and holds a fresh `can_create`, starts the sandbox through `Service.Resume`. The Terminal says the sandbox is starting, keeps saying so while it starts, and opens the shell once the session settles with a sandbox. Opening it on a session already starting waits for that start. Two tabs opening it start one sandbox.
3. **Refusals say why.** A caller who may view but not start the session, a bex API key whose grant does not cover it, and a workspace admission refusal (payment method, billing, the live-sandbox cap) each get a message saying why, as the SSH path's banners do (ADR054 D9). Canceled and archived sessions say why there is no Terminal. A caller who may not view the session gets the same answer as for a missing session. A start that ends without a sandbox says so and offers to try again.
4. **Bounded.** The ticket is single-use and short-lived, the WebSocket re-authorizes on the gateway's revalidation interval, the wait ends at the start's own ceiling (`agentsessions.SSHWakeTimeout`), with the browser or with a drain, and holds a session-limiter slot as an open shell does. The gateway counts the Terminal's wakes and their outcomes with no session, workspace or subject label.
5. **Recorded.** t001's placement and wake design are in ADR047 or ADR054, ADR035 § Browser Web Shell names the second target kind, `lego/backend/AGENTS.md` lists any new env var, and a security review covers the ticket and the held WebSocket as ADR124 covered the SSH wake.
6. **Live.** On production, in `tian-personal`, the Terminal on a session whose sandbox closed starts it and opens a shell, at 1280 px and 390 px, and the session is deleted afterwards.

## Source + Goal linkage

- **Source:** inbox note [w10/057](../done/057.md), filed by w10/m597 t007's Devin walk on 2026-11-23 and promoted by `/loopx w10` on 2026-11-23.
- **Triage evidence (2026-11-23, `b3d242e57f`):**
  - **The premise holds.** The gateway's Web Shell resolves only services: `webshell.Server{Apps: appService}` (`lego/backend/cmd/ssh-gateway/main.go:297-300`), while native SSH gets the composite resolver that routes `ags-…` to `agentsessions.SSHResolver` (`main.go:201`, `lego/backend/internal/sshgateway/composite.go`). Its ticket binds a `srv-…` service (`shellticket.Claims.ServiceID`, `lego/backend/internal/shellticket/shellticket.go`), minted only by `apps.Service.CreateShellSession` (`lego/backend/internal/apps/service.go:1931`, REST `POST /v1/services/{id}/shell-ticket`, GraphQL `createShellSession`). The session header offers Files and Connect only (`dashboard/src/features/agent-sessions/components/session-detail-header.tsx:352-354`), and the browser terminal is the service-only `WebShellTerminal` (`dashboard/src/features/services/components/web-shell-terminal.tsx`).
  - **What already exists.** The sandbox exec target (`agentsessions.SSHResolver.ResolveSSHSession`, `ssh.go:62`) and the wake: `Service.Resume` starts a closed session's sandbox, and its refusal params tell `closed` from `starting` (`errNoSandboxToOpen`, `ssh.go`). w10/m597 held an SSH login while the sandbox starts (`lego/backend/internal/sshgateway/nativessh/wake.go`), but its verb re-reads an SSH key by fingerprint, which a browser does not have. The dashboard's session hooks still carry a `resume` mutation (`dashboard/src/features/agent-sessions/hooks/use-agent-session-mutations.ts`).
  - **Devin's behavior.** "The session workspace has a new Terminal tab for running commands on Devin's machine." and "The Files tab can now be opened while Devin is asleep, and opening Files or Terminal wakes Devin." ([release notes](https://docs.devin.ai/release-notes/2026), 2026-10-05, read 2026-11-23). Since 2026-11-13 "the workspace pane remembers per session whether it was open, and a session starts with it closed." ADR018 records no signed-off divergence for it, so the Terminal ships.
  - **Not a `DO_NOT_DO.md` conflict.** #18's carve-out of 2026-08-09 puts access to an agent-session sandbox in scope (ADR054), and the browser-hosted Shell terminal was reopened on 2026-07-17. This milestone adds the browser path to the same sandbox target and does not widen the `srv-…` contract (single channel, no SFTP or forwarding).
  - **More than one sitting:** a ticket and a wake that need no SSH key, the Web Shell's second target kind and its held WebSocket, a dashboard surface with starting and refusal states in eight locales, ADR amendments and a security review, gateway, real-DB and dashboard tests, and a production check.
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), ADR008 pillar 5), where Devin is the reference product. It closes the last Devin difference on the Connect surface w10/m597's walk found.
- **Expected outcome:** anyone who may operate a session gets a shell in its sandbox from the browser, whether the sandbox is up or closed, with no `ssh` client and no key to register.
- **Why now:** since w6/m563 a person's sandbox closes 5 minutes after its turn, and w10/m597 made SSH wake it. The browser is now the only way in that cannot, and the one a member without a local terminal depends on. The milestone includes the Render parity task because it adds a dashboard surface and a ticket mint on the API.
