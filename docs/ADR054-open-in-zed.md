# ADR054 — Open in Zed: SSH remoting into agent-session sandboxes

**Status:** Accepted (2026-08-09), implemented in **w2/m65** — composite `ags-…` target resolver gated on `can_view_sensitive`, sandbox-only multi-channel (`BEX_SSH_MAX_CHANNELS_PER_CONN`) + `sftp-server` subsystem bridge, `GRANT SELECT ON agent_sessions` for the scoped gateway role, editor-aware Completer teardown deferral, `agentSession.sshAddress` across REST/GraphQL/MCP, and the dashboard Open-in-Zed control; backend + dashboard suites green. Amends [ADR035](ADR035-ssh.md) (single-channel invariant, subsystem ban, stale `can_operate` prose — folded in there); complements — does not replace — [ADR047](ADR047-cloud-coding-agent-sessions.md) D8's ACP IDE attach. Production activation still rides ADR035's gates (`scripts/ssh-activate.sh --check` before `BEX_SSH_HOST`); a live end-to-end Zed acceptance against a real cluster remains the milestone's final task. **Amended 2026-11-22 (w6/m564):** [D8](#d8--a-closed-session-wakes-for-an-editor-w6m564-2026-11-22) — Open in Zed on a session whose sandbox closed at its idle timeout starts the sandbox again, as opening a sleeping Devin session wakes it; D1's phase gate and D5's surfacing widen to a finished session's sandbox. **Amended 2026-11-23 (w6/168):** D8's editor sandbox keeps pushing the session branch after its start settles; the Git mint, and only it, admits it.

---

## Context

### The ask

The dashboard agent-session page (`dashboard.bex.co/agents/ags-…`) should offer an **Open in Zed** button that launches the Zed editor with the session's sandbox `/workspace` open as a remote project — the Devin-style "editor workspace tab" that ADR047 D9 names as a differentiator and defers. The user inspects and edits the agent's working tree with a real editor, terminals included, while (or after) the agent works.

Zed ships exactly the client half of this today:

- A browser-clickable hotlink: `zed://ssh/[<user>@]<host>[:<port>]/<path>` ([Zed remote development docs](https://zed.dev/docs/remote-development)).
- Zed shells out to the user's local OpenSSH binary. It opens **one TCP connection as an SSH ControlMaster** (`ssh -N -o ControlMaster=yes -o ControlPath=…`) and multiplexes everything over it: the Zed protocol, every terminal, every task — each is **another concurrent `session` channel on the same connection** (`crates/remote/src/transport/ssh.rs`).
- On first connect it installs `~/.zed_server/zed-remote-server-{channel}-{version}` on the remote (version must exactly match the client), then runs it via an `exec` request: `env RUST_LOG=… ~/.zed_server/zed-remote-server-… proxy --identifier …`. The proxy speaks over the exec channel's stdio — it listens on no TCP port.
- Binary install strategies, in order: remote download (`curl -f -L https://zed.dev/… -o …`); if that fails, **local download then upload over SSH** — preferring the **SFTP subsystem** (batch `put`), falling back to classic `scp -C -r`.
- It needs no TTY for the protocol channel, no agent forwarding, no X11; port forwarding only if the user opts into `port_forwards`.

### What bex already owns

The ADR035 gateway was built as a reusable protocol-to-`pods/exec` boundary, and almost all of it transfers:

| Gateway capability | State for Zed |
| --- | --- |
| Two-phase public-key auth against identity-scoped `ssh_keys`, deleted-key re-check, `core.Identity{Method:"ssh"}` (`nativessh/server.go`) | reuse unchanged |
| `TargetResolver` seam — one method, username in, `SSHInstanceTarget{Namespace,PodName,Container}` out (`sshgateway/resolver.go`) | add a composite dispatcher; `srv-…` path unchanged |
| `exec` with an arbitrary command as one `/bin/sh -lc` argv element, channel-as-stdin streaming, exit-status propagation (`nativessh/session.go`) | reuse unchanged — covers Zed's probing, install commands, and the `proxy` process |
| `KubeExecutor` — SPDY `pods/exec` through the API server, per-`Execute` stream, stateless (`sshgateway/exec.go`) | reuse unchanged; **no sandbox-ingress network change needed** (exec rides the API server, not the pod IP) |
| `SessionLimiter`, metrics, `ssn-…` audit rows, retention sweep | reuse; audit row's service-id column carries the `ags-…` id |
| Sandbox pod arithmetic: pod `<sandboxID>-0`, namespace `<workspaceID>-sandbox`, container `sandbox` (`agentsessions/service.go` `withTicket`) | reuse the same derivation, from a live store read instead of a ticket |
| `pods/exec` RBAC in sandbox namespaces — the `bex-tenant-ssh-gateway` ClusterRole is already bound per `<ws>-sandbox` namespace by the NamespaceReconciler (`store/namespaces.go`) | already in place (w3/m33 lineage) |
| Dashboard Connect/copy-command UI (`ServiceConnectButton`, `ConnectCodeRow`) | replicate on the session header |

Three things block Zed today, and they are the substance of this ADR:

1. **One `session` channel per connection.** `serveConn` accepts the first channel and actively rejects the rest (`rejectAdditionalChannels`). Zed's ControlMaster model requires many concurrent channels — including an initial master connection that opens **zero** channels.
2. **No sandbox target.** `parseSSHUsername` hard-rejects anything but `srv-…`; `ResolveSSHSession` resolves only running App instances; the scoped `bex_ssh_gateway` Postgres role cannot read `agent_sessions`; ADR035 lists "hosted sandboxes" as a non-goal.
3. **No binary-install path.** The sandbox's agent-phase egress does not include `zed.dev`, so remote download fails; the gateway bans subsystems (SFTP) and rejects the SCP server protocol, so Zed's upload fallback fails too.

ADR047 D8's "Phase 2+ native ACP IDE attach" is the _other_ Zed integration — Zed's agent panel driving the session over ACP. It is complementary: ACP attach steers the agent; this ADR opens the **files**. Neither substitutes for the other.

---

## Decision

Reuse the ADR035 gateway wholesale and extend it in five bounded places. No sshd enters any image; bex-api still never gains `pods/exec`; the `srv-…` path stays byte-identical.

### D1 — Target grammar: `ags-<xid>@ssh.bex.co` via a composite resolver

The SSH username grammar gains a second form: the agent-session id. A composite `TargetResolver` dispatches on `ids.KindOf` of the username's id prefix:

- `srv-…` → the existing `apps.Service.ResolveSSHSession`, unchanged.
- `ags-…` → a new `agentsessions` SSH resolver that (a) authorizes (D2), (b) reads the session row (`GetAgentSession`), (c) requires a live sandbox — phase ∈ {`creating`, `running`, `resuming`, `redispatching`} and `sandbox_id ≠ ''` (_widened by D8: a `completed`/`failed`, unarchived session's sandbox too, and `resuming` only once it settles_) — and (d) returns `SSHInstanceTarget{Namespace: <workspaceID>-sandbox, PodName: <sandboxID>-0, Container: "sandbox"}`, the exact derivation `withTicket` already uses for attach tickets. No pod name or namespace is ever parsed from caller input.
- Anything else → `ErrBadRequest` after the authorization attempt, preserving the 403-before-400/404 rule.

The resolver runs at handshake time inside `VerifiedPublicKeyCallback`, exactly like the App path: key possession proven → identity attached → target authorized and resolved → encoded into the connection's permissions. There is **no ticket** on this path; the SSH key plus a live OpenFGA check replace it, mirroring how native SSH already relates to the Browser Web Shell's ticket.

Instance suffixes (`ags-…-<instance>`) are rejected: a session has exactly one pod.

### D2 — Authorization: `can_view_sensitive`, added to the `agent_session` FGA type

A shell in the sandbox reads everything the sandbox holds: the gateway-refreshed Git token, the session's model API key, the full working tree. Per the sink principle (codex round-4 #8, commit `40b75be9`: "the relation belongs to the sink, not the transport" — `apps/service.go` `ResolveSSHSession`), the gate must match what the access exposes, not what the transport is. `can_operate` — which gates attach/steer — is too weak: a contributor holds `can_operate` and `can_manage_ssh_keys` and could otherwise enroll a key and `printenv` credentials that attach never reveals.

So the `agent_session` FGA type gains `can_view_sensitive: can_view_sensitive from workspace` (developer/admin), and the `ags-…` resolver gates on `AuthorizeOn(RelCanViewSensitive, "agent_session:<id>")`. This is deliberately the same relation the App SSH/Web Shell sink uses.

Note: ADR035 §Decision step 4 and §Audit still say `can_operate`; the code moved to `can_view_sensitive` in round 4. The amendment in this ADR fixes that stale prose while it touches those sentences.

Audit is unchanged in shape: one `ssn-…` row per connection (subject, workspace, `ags-…` id in the service-id column, remote address, start/end, result), no command or stream content, `BEX_AUDIT_RETENTION_DAYS` sweep.

### D3 — Multi-channel connections for sandbox targets only

For connections whose resolved target is an agent-session sandbox, the gateway drops the one-channel rule:

- Each `session` channel is served by its own goroutine running the existing self-contained `serveSession` → its own SPDY `pods/exec` stream. Channels share only the immutable target and the stateless executor — the per-channel code needs no change.
- A connection that opens no channels (Zed's `-N` ControlMaster master) is valid and simply holds until closed or until the session timeout.
- A new per-connection channel cap bounds fan-out: `BEX_SSH_MAX_CHANNELS_PER_CONN` (default `16`; `0` disables the exception and restores single-channel). The existing `BEX_SSH_MAX_SESSIONS` / `_PER_IDENTITY` limiter continues to count **connections** — per-channel `Acquire` against the 5-per-identity cap would break Zed by design (server proxy + N terminals), which is exactly why the bound is a separate per-connection semaphore rather than a limiter change.
- `srv-…` connections keep `rejectAdditionalChannels` verbatim.

The 4-hour `BEX_SSH_SESSION_TIMEOUT` applies to the whole connection; Zed's proxy reconnects (`--reconnect`) when the master drops, so an expiry mid-session degrades to a reconnect, not data loss (Zed persists unsaved buffers locally).

Still banned on every path, sandbox included: `direct-tcpip`, remote/agent/X11 forwarding, environment requests. Zed does not need them; a user's `port_forwards` setting will not work against bex and the docs say so.

### D4 — Server binary install: bridge the SFTP subsystem; keep egress closed

The sandbox's agent-phase egress allowlist stays exactly as ADR047 defines it — `zed.dev` is **not** added, so Zed's in-sandbox `curl` download fails closed. Zed then falls back automatically to local-download-then-upload, and the gateway makes that work:

- For `ags-…` targets only, a `subsystem: sftp` request is honored by exec-ing the distro's `sftp-server` binary in the sandbox pod (`/usr/lib/openssh/sftp-server`, added to `lego/agent-image/Dockerfile` via the `openssh-sftp-server` package — the image already carries `openssh-client`), with the channel bridged as stdio like any other exec. This is not generic subsystem support: the subsystem name must be exactly `sftp`, the exec'd argv is the fixed server path (never caller input), and `srv-…` targets continue to reject all subsystems.
- The classic SCP server protocol (`scp -t`/`-f`) stays rejected; SFTP is the supported upload path.
- Uploads land in `/home/bex/.zed_server/` on the sandbox's writable rootfs (uid 10001, `HOME=/home/bex`). They survive Resume (same pod) and die with teardown, which is correct: the session is the unit of persistence.

Optionally, the agent image may bake a pinned `zed-remote-server` (the musl static build) at the versioned path Zed probes, as a fast path for clients on that exact version — Zed requires an exact version match, so this is an optimization, never the mechanism. The upload path is what keeps arbitrary client versions working.

### D5 — Surfacing: `sshAddress` on the session view; the dashboard button

- The agent-session `View` (the single shape REST `/v1/agent-sessions*`, GraphQL `AgentSession`, and MCP project identically) gains `sshAddress: "ags-<xid>@<BEX_SSH_HOST>"`, assembled in the presenter exactly like `serviceDetails.sshAddress` (`apps/service.go` `view`), present only when `BEX_SSH_HOST` is set and the sandbox is live per D1's phase gate. `BEX_SSH_HOST` unset ⇒ field absent everywhere ⇒ feature invisible (byte-identical default, same activation gates as ADR035 — `scripts/ssh-activate.sh --check` must pass first).
- The session detail header (`session-detail-header.tsx`) gains a Connect-style control mirroring `ServiceConnectButton`: an **Open in Zed** action that is a plain anchor to `zed://ssh/<sshAddress>/workspace`, plus a `ConnectCodeRow` with the copyable `ssh <sshAddress>` command for any editor or terminal (VS Code Remote-SSH, plain OpenSSH — the same address works for a single interactive shell either way; multi-channel editors other than Zed are untested and out of scope). This is the dashboard's first custom-scheme `href`; it is an intentional external-protocol anchor, not a navigation target, so `safe-next.ts`-style http(s) guards do not apply to it.
- Shown only while the phase gate holds; terminal sessions render the unavailable explanation instead of inventing an address, matching the service Shell page convention. _Amended by D8: a finished session whose sandbox closed offers to start it instead. Amended by D9 (2026-11-23): `sshAddress` shows on every session that is neither canceled nor archived, because a login to it opens a sandbox that is up, waits for one that is starting and wakes one that closed; the Connect menu always offers the `zed://` link and the `ssh` command._

### D6 — Lifetime: an open SSH connection defers Completer teardown

Fire-and-forget semantics create a footgun: the Completer tears the sandbox down within ~15s of the driver reporting a finished turn — which would kill a user's live Zed session mid-edit. Rather than a new keep-alive channel, the Completer reuses the audit table the gateway already writes: before tearing down a finalized session, it checks `ssh_sessions` for an open row (no end time) targeting that `ags-…` id and defers teardown while one exists. Guards:

- Rows older than `BEX_SSH_SESSION_TIMEOUT` are ignored (a crashed gateway replica cannot pin a sandbox forever); deferral is therefore bounded by the 4-hour cap.
- Only an SSH login's rows count (`ssh_sessions.transport = 'ssh'`, migration `0339`; w6/m564 t009, 2026-11-23), and since D10 the session page's Terminal's (`web_shell`), a shell a person types in as in an editor's terminal. The conversation stream (agent attach) also writes a row for the session id, since ADR077 round 17, and is a reader of the transcript, not an editor. Read as one, a dashboard tab left open on a sandbox started for an editor (D8), whose live stream never ends, held it for the 4-hour cap, and every page view moved the idle clock. Production showed both on 2026-11-23.
- Deferral affects **teardown only** — the session still finalizes (`completed`/`failed`), the transcript is still captured (ADR051), delivery still happens (since [ADR090](ADR090-agent-owned-delivery.md), 2026-10-22, delivery is the agent's own commit, push and draft PR; the Completer only records it). The sandbox lingers for the editor; `sandbox_id` is cleared when teardown eventually runs.
- Explicit `Cancel` overrides deferral: the user asking to kill the session wins over an open editor.

Compute-seconds metering (ADR047 D7) keeps running while the sandbox lingers — editor time is sandbox time, and it is the caller's own workspace paying for its own open editor.

### D7 — Least-privilege deltas, enumerated

1. `GRANT SELECT ON agent_sessions TO bex_ssh_gateway` in `sshgateway/dbrole/dbrole.sql` (re-applied via `scripts/ssh-gateway-db-role.sh`); the scoped-role negative test (`TestGatewayScopedRoleAllowsOwnSurfaceDeniesTheRest`) updates to match. This is the only new grant — the role still cannot read transcripts' content tables beyond what it has, tenant Secrets, or anything else.
2. No RBAC change: the gateway's namespaced `pods/exec` grant already exists in every `<ws>-sandbox` namespace (bound by the NamespaceReconciler for sandbox-exec, w3/m33). The structural manifest test that forbids cluster-wide gateway grants keeps holding.
3. No network-policy change: SPDY `pods/exec` traverses the Kubernetes API server, not the pod network, and `zed-remote-server proxy` speaks stdio over the exec channel — no listening port, no new Cilium ingress. The gateway's existing 8787 ingress (agentattach) and 8082 (credential broker) are untouched.
4. bex-api is untouched at the exec boundary: it gains a presenter field (D5), and since D10 it mints a session's Terminal ticket (the Web Shell's, ADR035), starting a closed sandbox through `Resume` as the caller. The gateway remains the sole holder of `pods/exec`.
5. _Added by D9 (w10/m597, 2026-11-23)._ One gateway-to-bex-api verb that changes something: `POST /v1/agent-session-wake` (`agentsession.InternalSessionWakePath`) on bex-api's cluster-internal `:8091`, signed with `BEX_SANDBOX_EXEC_SECRET` under its own `wake-session` MAC domain and a single-use nonce. Before it, that secret let a gateway only ask bex-api questions (email verification, login requirements, an API key's grant) and mint the credentials a sandbox's own traffic needs. With it, a gateway can make bex-api run `Service.Resume` as the owner of a registered SSH key it names by fingerprint: bex-api re-reads the key and acts only as its owner, with every check that owner's own resume gets (`can_operate`, fresh `can_create`, the login requirements, admission), and holds a key enrolled by a bex API key to its grant (Agents manage and Sandbox execution). It starts a sandbox and nothing else: no exec, no credential, no turn. Reviewed in [ADR124](ADR124-security-review-ssh-session-wake.md).

### D8 — A closed session wakes for an editor (w6/m564, 2026-11-22)

Since w6/m563 a person's session closes its sandbox 5 minutes after its turn ([ADR059 § 2026-11-22](ADR059-agent-sandbox-hibernation.md#2026-11-22--idle-close-follows-devin-5-minutes-automations-at-once)), so an editor opened later meets no sandbox: in production 5 of 25 Zed opens after a turn came more than 5 minutes after it. Until now the resolver refused it (`AGENT_SESSION_NOT_ATTACHABLE`), the dashboard hid the button, and `resume` refused a session with no sandbox.

**Reference.** Devin: "The Files tab can now be opened while Devin is asleep, and opening Files or Terminal wakes Devin" ([docs.devin.ai/release-notes](https://docs.devin.ai/release-notes/overview), 2026-10-05, read 2026-11-22), and "Opening a browser preview of a sleeping session restarts the servers behind the preview automatically" (2026-10-16). A sleeping session also wakes on the next message, which bex does through `Steer`. GitHub Codespaces likewise starts a stopped codespace when you connect to it.

**The verb.** `resume` (REST `POST /v1/agent-sessions` with `resumeSessionId`, GraphQL `resumeAgentSession`, MCP `resume_agent_session`, all one `Service.Resume`) now means _give the session a sandbox an editor can open_. It needs `can_operate` and a fresh `can_create`, as before, and never runs a turn:

| The session | `resume` |
| --- | --- |
| a turn runs or starts (`creating`, `running`, `redispatching`) or a start is under way (`resuming`) | returns it unchanged |
| `completed`/`failed`, its sandbox still up (inside its idle window) | marks the sandbox as kept for an editor and restarts its idle clock |
| `completed`/`failed`, no sandbox | starts one: admitted like a turn (payment method, billing, live-sandbox cap), then `resuming` |
| `canceling`/`canceled` | `AGENT_SESSION_NOT_RESUMABLE` |
| archived | `AGENT_SESSION_ARCHIVED` |

OpenSandbox's resume of a suspended sandbox is gone from this path (`ResumeAgentSessionSandbox` deleted): nothing suspends an agent sandbox since hibernation retired ([ADR059 § Superseded](ADR059-agent-sandbox-hibernation.md#2026-10-25--superseded-hibernation-retired)).

**What starts.** The sandbox the session's next turn would get, running no turn: the kept disk when retention kept one ([ADR047 D12](ADR047-cloud-coding-agent-sessions.md#d12--a-message-to-a-running-session-is-queued-stop-turn-ends-only-the-current-turn-w22m403-2026-11-14), w10/m491), claimed as a turn's dispatch claims it, else a fresh checkout of the session's `bex-agent/*` branch on its repository environment's prebuilt image or setup steps. It gets the session's secrets, egress tier and agent environment, and the driver runs with no prompt and `BEX_AGENT_EDITOR=1`: it checks out the workspace, writes the status `ready` and waits. It carries no dispatch-turn label, so no turn's cleanup takes it for its own. It is one create: a pool with no room fails the start rather than holding a person in a wait no turn record keeps alive.

**States.** While the sandbox starts the session reads `resuming` (state `working`). It is a live phase, so the checkout's fetch can mint Git credentials once the sandbox is bound, a message sent meanwhile is queued and runs after the start (never racing it for the kept disk), and Stop has no turn to end (`AGENT_SESSION_NO_TURN_RUNNING`). The SSH resolver and `sshAddress` wait for the settle. When the driver reports `ready` (or `failed`, or nothing within the turn ceiling) the Completer returns the session to the phase it was resumed from — `failed` when it carries a failure reason, else `completed` — with the sandbox: the state a finished turn's sandbox is in during its idle window. `agent_sessions.editor_sandbox_id` (migration `0335`) records the sandbox resume started or kept; while it equals `sandbox_id` the sandbox is an editor's.

**Closing.** The idle rule of ADR059 2026-11-22 applies from the settle: an open editor SSH session keeps it (an SSH login only: the session page's conversation stream does not, D6); it closes after a person's timeout (5 minutes, or the Enterprise workspace's own) after the later of the settle and the last disconnect. An editor's sandbox gets that timeout whoever started the session — an automation's, a child's or a failed one too — because a person asked for it. Its close reads the checkout in the sandbox, not the turn's recorded facts, harvests no transcript (it ran no turn), and keeps the disk when it holds undelivered work. The kept disk it booted from was spent when it bound (as a turn's is), so that close may keep the same turn's disk again (`BeginAgentSessionRetention` admits a `continued` turn for the editor's sandbox), edits included, and the next message continues from it.

**Reaching it.** An editor may open any session's sandbox while it is up: D1's resolver and D5's `sshAddress` accept a `completed`/`failed` session that names a sandbox and is not archived, besides a turn's live phases. Before, a finished turn's sandbox waited out its idle window out of reach. Capabilities (`GET /v1/agent-sessions/capabilities`, GraphQL `agentSessionCapabilities`, MCP `get_agent_session_capabilities`) gain `sshEnabled`, so a client offers a start only where `BEX_SSH_HOST` is set. The dashboard's Connect menu on a closed session offered **Start the sandbox**, showed that it was starting, then the `zed://` link and `ssh` command. _Superseded by D9 (2026-11-23): connecting starts it, so the menu offers the link and the command at once._

**A restart mid-start converges** — ADR059's orphaned `resuming` row cannot recur. The Completer watches `resuming` like any live phase. A row whose sandbox is bound settles from the driver's status (`ready`, `failed`, nothing past the turn ceiling, or a lost sandbox, which is dropped). A row with no sandbox past the start's own ceiling plus a minute (16 minutes) returns to its finished phase; whatever the dead start created is then an orphan of a finished session, which the inventory reconcile ends, and its claim on the kept disk lapses with the claim lease. The start binds only to a session still `resuming` with no sandbox and not archived (a compare-and-set), and otherwise ends the sandbox it made, so a cancel, an archive or a converged row always wins. There is no mint race: the driver retries its checkout's Git calls for 30 seconds while the binding lands, as a turn's does.

**Pushing from it (w6/168, w6/175, 2026-11-23).** A person who edits in a finished session's sandbox and runs `git push` in its terminal pushes the session branch, as a Devin machine's terminal pushes whenever the machine is awake. Until then the turn's end or the settle cut the push off: the session was `completed` again, and `agentsession/mint.go`'s live-phase allowlist refused every credential, so the gateway answered `remote: forbidden`. The Git mint (`Minter.Mint`, which the gateway's Git proxy calls) now also admits a session that is `completed` or `failed` and not archived, when its sandbox is one a person may be at:

- **an editor's sandbox** (w6/168): `sandbox_id` equals `editor_sandbox_id`, the sandbox `resume` started or kept for a person;
- **any sandbox a person's SSH login holds open** (w6/175): an `ssh_sessions` row for the session that has no end, is an SSH login (`transport = 'ssh'`), or since D10 the session page's Terminal (`web_shell`), and started within the gateway's 4-hour session cap (`agentsession.SSHLoginCap`). That is the read the idle close makes (D6, `AgentSessionSSHActivity`). It covers a finished turn's own sandbox in its idle window, where most Zed opens land: 20 of 25 in w6/m564's production sample came within 5 minutes of a turn.

The caller must still be the session's current sandbox, the exact Pod (`<sandbox_id>-0` and its UID), on the session's own `bex-agent/*` branch. This widens, on purpose and only this far, the window [ADR055](ADR055-security-review-round4.md) F12 closed:

- **Only Git.** The model mint, the `gh` route's read and write mints, the PR verbs, the share, result and spawn routes and the egress grant keep the live-phase allowlist (`CurrentSandboxCaller`). No agent runs once the turn ended, so nothing in the sandbox needs a model or acts as the agent.
- **Only where a person may be.** With no SSH login or Terminal (D10) into the session open, a finished turn's own sandbox is refused, unless `resume` kept it for an editor. The conversation stream (agent attach) is a reader, not a login, and does not count. Neither does a row older than the 4-hour cap, which a crashed gateway replica never closed. The turn sandbox an editor's start replaced is refused, because it is no longer the session's sandbox. A turn after it gets a new sandbox, which ends the editor binding.
- **Why the mint reads the login (w6/175).** The other way was the gateway calling `resume` (through the D9 wake verb) on every login into a finished session whose sandbox is up, which would mark it as the editor's. Reading the login also covers a login opened while the turn ran and still open when it finished, which `resume` never marks. It needs no new gateway-to-bex-api call per login, and it admits a turn's sandbox only while the login is open, not until the idle close.
- **What it can do.** The token never enters the sandbox: the gateway holds it for one proxied exchange. A push may update only the session's branch (`validateReceivePack`), never the default branch or a tag. That is the write the session's own turns already make. A finished turn's sandbox, unlike an editor's start, may still run processes the agent left behind, and while a login is open they could push too. They can make no write the turn itself could not.
- **Who holds a shell there.** Only someone with `can_view_sensitive` on the session reaches it over SSH (D2). That is a developer or an admin, the same roles `resume`'s `can_create` needs, and starting an editor's sandbox also took `can_operate` and a fresh `can_create` (`resume`).
- **How long.** An editor's sandbox, until the idle close: a person's timeout (5 minutes, or the Enterprise workspace's own) after the later of the settle and the last SSH disconnect. A turn's own sandbox, while the login is open. An open SSH login holds either, each connection for at most the gateway's 4-hour session cap (D6). An archive or a cancel refuses the mint at once.

**Limits.** A terminal in a finished session's sandbox gets no model credential and no `gh` route, so an agent CLI or `gh` run there by hand is refused. A plain `ssh ags-…@ssh.bex.co` into a closed session wakes it (D9); before 2026-11-23 it was refused with a banner that said how to start it (w10/043).

### D9 — An SSH login wakes a closed session (w10/m597, 2026-11-23)

D8 gave an editor a way to start a closed session's sandbox, but only through `resume`: the dashboard's **Start the sandbox**, or an API call. A plain `ssh ags-…@ssh.bex.co`, the command the session page copies and the CLI's `bex agent ssh` runs, was refused with a banner (w10/043). Since w6/m563 a person's sandbox closes 5 minutes after its turn, so most such commands met a closed sandbox.

**Reference.** Devin: "Sleeping Sessions Wake on Preview or SSH Traffic. Opening a preview URL or SSHing into a session whose machine has gone to sleep now wakes the machine and completes the request instead of showing 'not running'" ([docs.devin.ai/release-notes/2026](https://docs.devin.ai/release-notes/2026), 2026-09-25, read 2026-11-23). bex has no preview URL on an agent session yet, so only the SSH half applies. [ADR018](ADR018-render-parity.md) records no divergence: every choice below follows Devin.

**Where the wake runs.** In the gateway's `VerifiedPublicKeyCallback`, after the client proved possession of the key, the key was re-read, its owner's email was found verified, the resolver passed `can_view_sensitive` (cached and fresh) and the workspace's login requirements, and a bex API key owner's grant was found to cover Sandbox execution. Never in `PublicKeyCallback`, which a client may call as a key query. The resolver's `AGENT_SESSION_NOT_ATTACHABLE` refusal now says, in its params, whether the session is `closed` (completed or failed, not archived, no sandbox) or `starting` (a turn's live phase with no sandbox bound yet, or `resuming`), and names its workspace. A canceled or archived session says neither and keeps its banner.

**The verb.** For a `closed` session the gateway calls `POST /v1/agent-session-wake` on bex-api's `:8091` (D7 item 5) with the session id, the key's owner, the key's fingerprint and the client's address. bex-api re-reads the key by fingerprint and refuses unless its owner is the subject named, holds a bex API key owner to its grant (Agents manage and Sandbox execution, as its own REST resume and its shell need), and runs `Service.Resume` as `{owner, method ssh, the key's sign-in}`, with the client's address on the audit row. The answer is a 200 either way: the phase resume left the session in (`resuming` while the sandbox starts, its own phase when one is up or a turn runs), `forbidden` (the owner may not start it), or `refused` with resume's own code and message (payment method, billing, the live-sandbox cap, or an archive or cancel since the gateway read it). A login into a `starting` session does not call it.

**The held login.** The gateway admits the login with a target that names the session and no pod (`bex.waking`), and polls every 2 seconds, with the same key read and resolver the login passed, until the session settles. Every session channel opened before then (a shell, an exec, Zed's channels, its `sftp`) is accepted and held: its request is answered, its stderr says `bex: starting the sandbox of agent session ags-…. This connection continues once it is up.`, and every 30 seconds that it is still starting. Once the sandbox is up each held channel re-authorizes as any channel does at open (key, email, resolver, grant) and runs its command in the started sandbox; channels opened later open as on any connection. The revalidation watchdog skips while the login is held, because nothing runs and the wait re-reads the key and the authorization itself. A Zed `-N` master that opens no channel simply waits.

**The ceiling and the end.** The wait ends when the session settles, when `agentsessions.SSHWakeTimeout` passes (16 minutes, the editor start's own bound, past which the Completer has settled the session either way), or with the connection: the client hanging up, the 4-hour session cap, or the gateway's drain bound, as an open shell ends. When it ends without a sandbox, each held channel prints why (`bex: the sandbox of agent session ags-… did not start. Connect again to try again.`, the ceiling, a cancel or archive meanwhile, or the generic `authorization no longer valid`) and exits with status 1, and the gateway closes the connection once the client hung up or `clientCloseGrace` passed. Nothing is left `resuming`: the Completer settles the start as for any `resume` (D8).

**What the client is told.**

| The login | Result | `session_wakes_total` |
| --- | --- | --- |
| a closed session, the owner may start it | held, the shell opens once the sandbox is up | `opened` (or `no_sandbox`, `timed_out`, `abandoned`) |
| a session already starting | held the same way, nothing else starts | as above |
| a closed session, no `can_operate`, no fresh `can_create`, or a key whose grant does not cover the start | w10/043's banner: not running, send it a message | `refused` |
| bex-api refused the start (admission, archived or canceled meanwhile) | banner: `not running, and its sandbox could not start: <resume's message>` | `refused_start` |
| bex-api gave no answer (down, unwired, an older release's 404) | w10/043's banner | `unavailable` |
| canceled, archived | their banners (w10/043) | — |
| may not view it, no such session, a key without Sandbox execution | the bare `Permission denied (publickey)` | — |

Every banner is counted `authentications_total{result="rejected_no_sandbox"}`; a held login is `accepted`. `bex_ssh_gateway_session_wakes_total{result}` counts each login once, with no session, workspace or subject label.

**Bounds.** A held login holds its session-limiter slot (5 per identity, 100 per process) and each held channel a channel-limiter slot, as an open shell does. Logins that repeat start one sandbox: `resume` returns a `resuming` session unchanged, and `BeginAgentSessionEditorStart` is a compare-and-set. A start is admitted like a turn, so the live-sandbox cap holds. The wake is off where the multi-channel carve-out is (`BEX_SSH_MAX_CHANNELS_PER_CONN=0`) or no verb is wired (`BEX_SANDBOX_EXEC_SECRET` unset): the login then gets w10/043's banner. `BEX_AGENT_SESSION_WAKE_API_URL` overrides the verb's URL.

**Surfaces.** `sshAddress` shows on every session that is neither canceled nor archived, sandbox or not, on REST, GraphQL and MCP alike (one presenter): the address always connects, at once or after the wake. The dashboard's Connect menu offers the `zed://` link and the `ssh` command whenever it shows, with a spinner on its trigger while a sandbox starts and the hint that a closed sandbox starts on connect; **Start the sandbox** is gone. Zed waits for it: its master connection authenticates at once (Zed allows 17 seconds), and its probe, upload and server commands have no timeout of their own (`crates/remote/src/transport/ssh.rs`, read 2026-11-23). `resume` stays on every API surface. The CLI's `bex agent ssh` and `bex edit ags-…` (and `bex agent open`, which opens bex desktop) read `sshAddress` and run `ssh`, so they wake a closed session too, and a canceled or archived session's refusal uses the gateway's words (w6/169).

### D10 — The session page's Terminal wakes a closed session (w10/m615, 2026-11-23)

D9 made `ssh` wake a closed sandbox, but only for a caller with a registered SSH key and a local terminal. The session page had Files and Connect; a member without a local terminal had no way into the sandbox at all, and the Browser Web Shell ([ADR035](ADR035-ssh.md#browser-web-shell)) reached only `srv-…` services.

**Reference.** Devin: "The session workspace has a new Terminal tab for running commands on Devin's machine." and "The Files tab can now be opened while Devin is asleep, and opening Files or Terminal wakes Devin" ([docs.devin.ai/release-notes/2026](https://docs.devin.ai/release-notes/2026), 2026-10-05, read 2026-11-23). Devin's workspace pane holds the Terminal tab beside the conversation; since 2026-11-13 "the workspace pane remembers per session whether it was open, and a session starts with it closed", and "workspace tabs no longer open on their own as Devin works". [ADR018](ADR018-render-parity.md) records no divergence: every choice below follows Devin.

**Placement.** A **Terminal** button in the session header, beside Files, opens a sheet on the right that holds the terminal, as Files does ([ADR095 D10](ADR095-agent-attachments.md)): bex's session page is one full-page conversation, and the sheet is its side pane. It is 768 px wide on a desktop and the full width at a phone's. Below 640 px the header row has no room for another button beside Files, Connect, Stop and Cancel, so the header's "…" menu opens the same sheet there. The sheet never opens on its own as the agent works, and closing it ends the shell.

**Remembered per session (w10/060, 2026-11-24).** As Devin's workspace pane "remembers per session whether it was open, and a session starts with it closed", the page keeps in the browser's local storage which sheet a session had open, Files or Terminal, for the 50 sessions left most recently. A session never opened starts with both closed. A reload or a return visit reopens the one left open, right after hydration, since the server cannot read the browser's storage. Closing a sheet forgets it. Another tab's choice does not open or close this tab's sheet. A reopened Terminal behaves according to its sandbox:

- **Up, or already starting** (a turn's live phase, a `resuming` start, or a finished turn's sandbox before its idle close): the shell opens at once, as Devin's Terminal tab shows a live session. It then holds the sandbox as any open Terminal does, until the sheet closes, the tab goes or the 4-hour cap ends it. Holding it extends compute that is already running; nothing new starts.
- **Closed** (a completed or failed session with no sandbox): the sheet says the sandbox has closed and waits for **Start the sandbox**, which mints the ticket and so starts it. A page load never starts metered sandbox compute by itself; a browser restoring a dozen tabs would otherwise start a dozen sandboxes. Devin documents waking only on an action: "opening Files or Terminal wakes Devin" (2026-10-05), opening a preview or SSH traffic (2026-09-25), typing in the composer (2026-09-16). Its sleeping Desktop tab shows the last screenshot with a one-click "Wake up Devin" (2026-09-09). No note says a restored pane wakes it (release notes read 2026-11-24).

A Terminal the person opens with the button or the menu starts a closed sandbox at once, as before. A Devin-style workspace pane that holds more than Files and the Terminal is out of scope.

**The ticket.** `shellticket.Claims` gains `SessionID` (`ags`), exclusive with `ServiceID` and `InstanceID`: `Verify` refuses a ticket naming both, neither, or a session field that is not an `ags-…` id. It is the same flavor: the same `BEX_SHELL_TICKET_SECRET`, MAC domain, 90-second lifetime and single-use nonce claimed across every gateway replica. A ticket a bex API key minted names `sandboxes.exec` (Sandbox execution), the action native SSH holds an `ags-…` login to, and the gateway holds the shell to that grant for as long as it runs. The gateway routes a ticket by the field it names, never by an id's prefix: `SessionID` goes to the agent-session resolver (D1), `ServiceID` to `apps.Service` exactly as before. An older gateway reads a session ticket as malformed (no service) and refuses it.

**Who wakes the sandbox: bex-api, at the mint.** A browser has no SSH key, so D9's verb, which re-reads a key by fingerprint, cannot start a sandbox for it. bex-api holds the caller's identity when it mints the ticket, so the mint starts a closed sandbox itself, through `Service.Resume` as the caller; there is no new gateway verb. The mint (`CreateShellSession`) needs `can_view_sensitive` on the session (D2), then:

| The session | The mint |
| --- | --- |
| its sandbox is up (a turn's live phase, or `completed`/`failed` and unarchived, with a sandbox) | a ticket, `sandbox: up` |
| a turn's sandbox or a `resume` start is under way (`creating`, `running`, `redispatching` or `resuming` with no sandbox bound) | a ticket, `sandbox: starting`; nothing else starts |
| `completed`/`failed`, unarchived, no sandbox | `Service.Resume` as the caller (`can_operate`, a fresh `can_create`, admission), then a ticket, `sandbox: starting` |
| canceled or canceling, or archived | `409 AGENT_SESSION_NOT_ATTACHABLE` with the resolver's own refusal, D9's banner text |

Two tabs, a retry or a second member opening it start one sandbox: `Resume` returns a `resuming` session unchanged, and `BeginAgentSessionEditorStart` is a compare-and-set. The start's audit row is `Resume`'s, as the caller.

**The wait: the gateway holds the WebSocket.** For `sandbox: starting` the browser opens the WebSocket at once. The gateway verifies the ticket and consumes its nonce, then resolves the session as the ticket's subject; a resolver refusal that says `starting` (D9's params) is held:

- It sends `{"type":"starting"}`, then `{"type":"starting","elapsed":N}` every 30 seconds, and `{"type":"ready"}` once the sandbox is up, then bridges the shell as it does for a service. The dashboard shows "Starting the sandbox…" from the mint's answer on, keeps saying so, and shows the shell after `ready`. Frames typed meanwhile are dropped; resizes reach the shell.
- It polls every 2 seconds with the ticket subject's own resolution (`can_view_sensitive`, cached and fresh, and the workspace's login requirements) and the key's grant, as D9's held login does, and accepts at most three unanswered reads in a row.
- The ceiling is the start's own, `agentsessions.SSHWakeTimeout` (16 minutes). The wait also ends with the browser, the subject's access, the key's grant, the Terminal's 4-hour cap (`BEX_SSH_SESSION_TIMEOUT`, as an SSH connection to a session) or the gateway's drain bound, as an open shell ends.
- It holds a session-limiter slot from before the wait, as an open shell does. The `ssh_sessions` row and the shell's audit events are written once the shell opens.

A wait that ends without a sandbox sends an error frame and closes. `reason: no_sandbox` when the start ended without one, `wake_timeout` at the ceiling, `revoked` when access or the grant ended, `timeout` at the cap or the drain, `unverified` after three unanswered reads; a cancel or archive meanwhile sends the resolver's message. The dashboard names each reason in the reader's language and offers **Reconnect**, which mints again and so starts the sandbox again. A session whose start already ended when the WebSocket connects gets `no_sandbox` at once.

**The shell.** `/bin/sh` with a TTY in the sandbox container, in `/workspace`, as the sandbox's user (uid 10001), over one `pods/exec` stream, the same exec an `ssh` shell gets. The revalidation watchdog re-resolves the session and re-asks the grant every `BEX_SSH_REVALIDATE_INTERVAL`; a sandbox that closes under the shell ends it with `instance_gone` ("the sandbox stopped").

**An open Terminal keeps the sandbox, as an editor does (D6).** The idle close counts the Terminal's `web_shell` rows with an SSH login's (`AgentSessionSSHActivity`): an open Terminal holds the sandbox, and its close moves the idle clock. The conversation stream's rows still do not count. While a Terminal is open, `git push` in it pushes the session branch, from a finished turn's own sandbox too, as an open SSH login allows (D8 _Pushing from it_, w6/175): the Git mint reads the same activity.

**What the client is told.**

| The case | Mint | Gateway |
| --- | --- | --- |
| sandbox up | ticket, `up` | the shell |
| starting, or closed and the caller may start it | ticket, `starting` | `starting`, then `ready` and the shell |
| closed; no `can_operate`, no fresh `can_create`, or a key without Agents manage | `409 AGENT_SESSION_NOT_ATTACHABLE`, D9's banner ("not running … send the session a message, then connect again"), `params.sandbox: closed` | — |
| admission refuses the start (payment method, billing, the live-sandbox cap) | `Resume`'s own code and message | — |
| canceled, archived | `409 AGENT_SESSION_NOT_ATTACHABLE` with their banners | their banner, if it happens meanwhile |
| may not view the session, or no such session | `403`, the same for both | `this agent session's sandbox can't be opened right now` |
| a key without Sandbox execution | `403` (key permission) | `revoked` once its grant narrows |
| no browser terminal on this deployment | `503 SHELL_UNAVAILABLE` | — |

The dashboard says why in each case: "This session isn't running, and its sandbox has closed. To start it again, send the session a message, then open the Terminal again." for the closed case it may not start, "The sandbox couldn't start: …" before an admission refusal, and its own words for a canceled or archived session, without minting.

**Surfaces.** REST `POST /v1/agent-sessions/{id}/shell-ticket` (`201`) and GraphQL `createAgentSessionShellSession(id)` (`AgentSessionShellSession {ticket url expiresAt sandbox}`), as a service's Shell ticket is on REST and GraphQL and not MCP: an MCP client has no terminal to open. Scope class `sensitive`; key permission `sandboxes.exec`, and the start inside it needs `agents.manage` too. Capabilities (REST, GraphQL `agentSessionCapabilities`, MCP `get_agent_session_capabilities`) gain `terminalEnabled`, true where bex-api has `BEX_SHELL_TICKET_SECRET` and `BEX_SHELL_WS_URL`; the dashboard offers the Terminal only then. The session view is unchanged: the Terminal applies exactly where `sshAddress` does (neither canceled nor archived).

**Metrics.** `bex_ssh_gateway_web_shell_wakes_total{result}` counts each Terminal that waited, once: `opened`, `no_sandbox` (the start ended without one, or a cancel or archive meanwhile), `timed_out`, `abandoned` (the browser, access, the grant, the cap or the drain ended first). No session, workspace or subject label. Starts themselves are `Resume`'s.

**Least privilege.** bex-api still never holds `pods/exec`: it authorizes, starts through `Resume` (which it already did for D8) and signs. The gateway gains no grant: it reads `agent_sessions` (D7 item 1) and writes the `ssh_sessions` row it already writes for a service's shell. Reviewed in [ADR126](ADR126-security-review-agent-session-terminal.md).

---

## Security analysis

- **Blast radius of the multi-channel carve-out.** Channels multiply streams, not privilege: every channel on a connection runs as the same authenticated identity against the same resolved pod, each through its own bounded exec stream, capped per connection. The single-channel rule existed to stop queued-shell confusion on App instances; for the sandbox target the many-channel shape _is_ the product, and the cap plus the connection limiter bound exhaustion. `bex_ssh_gateway_*` metrics gain a channel count histogram with the same no-identifying-labels rule.
- **Credential exposure is priced in, not accidental.** The sandbox already holds session-scoped, short-lived credentials by design (ADR047 D2: gateway-refreshed Git token confined to `bex-agent/*` branches; BYO model key). D2 gates shell access on `can_view_sensitive` so only roles that may read sensitive material get it, and the credentials a shell can read are the ones ADR047 already assumes a compromised sandbox process could exfiltrate. No platform credential lives in the pod (`automountServiceAccountToken: false`, gVisor, dedicated node pool).
- **The SFTP bridge is not a file-transfer product.** It exists so Zed can install its server; it necessarily also lets an authorized developer copy files in/out of the sandbox working tree — the same material `git push` to the draft PR already externalizes, and strictly less than the shell they already have on this path. It stays off for `srv-…` App instances, where ADR035's ban is unchanged.
- **Target confusion.** Pod and namespace are derived server-side from the session row (`<sandboxID>-0`, `<ws>-sandbox`) — the identical arithmetic as ticket minting — and the partial unique index on live `sandbox_id` guarantees one session per sandbox. Nothing caller-controlled reaches the exec URL.
- **Fail-closed inventory.** `BEX_SSH_HOST` unset ⇒ no address surfaced; `BEX_SSH_MAX_CHANNELS_PER_CONN=0` ⇒ carve-out off; sandbox not live ⇒ resolver refuses at handshake, unless D9 holds the login for a sandbox it starts or waits for, and nothing executes until a channel re-authorizes against the started one; FGA relation absent (model not migrated) ⇒ deny; DB grant absent ⇒ resolver errors, connection refused.

## Amendments to ADR035 (applied on acceptance)

1. §Decision step 6 and §SSH protocol ("one `session` channel per SSH connection", "extra session channels" in the never-reach-Kubernetes list): add the agent-session exception — multiple concurrent session channels, each its own `pods/exec` stream, bounded by `BEX_SSH_MAX_CHANNELS_PER_CONN`; channel-less connections permitted.
2. §Decision step 4 and §Audit: correct the stale `can_operate` to `can_view_sensitive` (codex round-4 #8) and add the `ags-…` → `agent_session:<id>` resolver branch.
3. §SSH protocol: subsystems ban becomes "subsystems including SFTP (except the `sftp` bridge for agent-session sandbox targets)"; SCP protocol stays banned.
4. §Isolation and RBAC: enumerate the sandbox-namespace `pods/exec` binding and the `agent_sessions` SELECT grant.
5. §Context / non-goals: "hosted sandboxes remain excluded" narrows to exclude _anonymous/ephemeral hosted execution_; the agent-session sandbox — a workspace-owned, authorized, audited resource — is admitted the same way the Browser Web Shell was carved in.

## Consequences and limitations

- Zed becomes the second first-class editor surface (after the Browser Web Shell) with zero bex-client software: the button is a URL; auth is the user's existing SSH key (Account Settings → SSH Public Keys); the address also works as plain `ssh ags-…@ssh.bex.co` for a one-shot shell in any terminal.
- Zed's exact-version server matching means first connect uploads ~15–20 MB (compressed) through the gateway per session; the optional baked binary removes that for matching versions. Acceptable for v1; revisit only if gateway egress metering says otherwise.
- Port forwarding inside Zed does not work against bex (forwarding stays banned). Documented, not planned.
- `ags-…` SSH is free-plan-agnostic: eligibility is the live sandbox plus the FGA relation, not a paid compute tier — agent sessions are already metered products (ADR047 D7), unlike the ADR035 paid-plan gate on App SSH.
- The teardown deferral (D6) means a finished session's sandbox can bill up to 4 extra hours if an editor stays open. This is visible in usage metering and bounded; an explicit Cancel always reclaims immediately.
- Contributors (can_operate, not can_view_sensitive) can steer and attach but not open Zed — consistent with the Web Shell boundary, and worth a line in the members docs.

## Non-goals

- ACP-based Zed agent-panel attach (ADR047 D8 phase 2+) — separate feature, unchanged by this ADR.
- Zed (or any editor) remoting into **App service instances** — `srv-…` targets keep the single-channel, no-subsystem contract; this ADR's carve-outs are sandbox-only.
- VS Code Remote-SSH support — the copyable `ssh` command may work for a shell, but VS Code's server bootstrap is untested and not a supported target here.
- An sshd, SFTP daemon, or any listener inside tenant or sandbox images.
- Keeping sandboxes alive indefinitely for editing — the 4-hour connection cap is the ceiling; a "workspace persistence" product is out of scope (`.pm/DO_NOT_DO.md` hosted-execution boundary otherwise stands).
