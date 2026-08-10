# ADR054 — Open in Zed: SSH remoting into agent-session sandboxes

**Status:** Accepted (2026-08-09), implemented in **w2/m65** — composite `ags-…` target resolver gated on `can_view_sensitive`, sandbox-only multi-channel (`BEX_SSH_MAX_CHANNELS_PER_CONN`) + `sftp-server` subsystem bridge, `GRANT SELECT ON agent_sessions` for the scoped gateway role, editor-aware Completer teardown deferral, `agentSession.sshAddress` across REST/GraphQL/MCP, and the dashboard Open-in-Zed control; backend + dashboard suites green. Amends [ADR035](ADR035-ssh.md) (single-channel invariant, subsystem ban, stale `can_operate` prose — folded in there); complements — does not replace — [ADR047](ADR047-cloud-coding-agent-sessions.md) D8's ACP IDE attach. Production activation still rides ADR035's gates (`scripts/ssh-activate.sh --check` before `BEX_SSH_HOST`); a live end-to-end Zed acceptance against a real cluster remains the milestone's final task.

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
- `ags-…` → a new `agentsessions` SSH resolver that (a) authorizes (D2), (b) reads the session row (`GetAgentSession`), (c) requires a live sandbox — phase ∈ {`creating`, `running`, `resuming`, `redispatching`} and `sandbox_id ≠ ''` — and (d) returns `SSHInstanceTarget{Namespace: <workspaceID>-sandbox, PodName: <sandboxID>-0, Container: "sandbox"}`, the exact derivation `withTicket` already uses for attach tickets. No pod name or namespace is ever parsed from caller input.
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
- Shown only while the phase gate holds; terminal sessions render the unavailable explanation instead of inventing an address, matching the service Shell page convention.

### D6 — Lifetime: an open SSH connection defers Completer teardown

Fire-and-forget semantics create a footgun: the Completer tears the sandbox down within ~15s of the driver reporting a finished turn — which would kill a user's live Zed session mid-edit. Rather than a new keep-alive channel, the Completer reuses the audit table the gateway already writes: before tearing down a finalized session, it checks `ssh_sessions` for an open row (no end time) targeting that `ags-…` id and defers teardown while one exists. Guards:

- Rows older than `BEX_SSH_SESSION_TIMEOUT` are ignored (a crashed gateway replica cannot pin a sandbox forever); deferral is therefore bounded by the 4-hour cap.
- Deferral affects **teardown only** — the session still finalizes (`completed`/`failed`), the transcript is still captured (ADR051), delivery still happens. The sandbox lingers for the editor; `sandbox_id` is cleared when teardown eventually runs.
- Explicit `Cancel` overrides deferral: the user asking to kill the session wins over an open editor.

Compute-seconds metering (ADR047 D7) keeps running while the sandbox lingers — editor time is sandbox time, and it is the caller's own workspace paying for its own open editor.

### D7 — Least-privilege deltas, enumerated

1. `GRANT SELECT ON agent_sessions TO bex_ssh_gateway` in `sshgateway/dbrole/dbrole.sql` (re-applied via `scripts/ssh-gateway-db-role.sh`); the scoped-role negative test (`TestGatewayScopedRoleAllowsOwnSurfaceDeniesTheRest`) updates to match. This is the only new grant — the role still cannot read transcripts' content tables beyond what it has, tenant Secrets, or anything else.
2. No RBAC change: the gateway's namespaced `pods/exec` grant already exists in every `<ws>-sandbox` namespace (bound by the NamespaceReconciler for sandbox-exec, w3/m33). The structural manifest test that forbids cluster-wide gateway grants keeps holding.
3. No network-policy change: SPDY `pods/exec` traverses the Kubernetes API server, not the pod network, and `zed-remote-server proxy` speaks stdio over the exec channel — no listening port, no new Cilium ingress. The gateway's existing 8787 ingress (agentattach) and 8082 (credential broker) are untouched.
4. bex-api is untouched at the exec boundary: it gains a presenter field (D5), nothing else. The gateway remains the sole holder of `pods/exec`.

---

## Security analysis

- **Blast radius of the multi-channel carve-out.** Channels multiply streams, not privilege: every channel on a connection runs as the same authenticated identity against the same resolved pod, each through its own bounded exec stream, capped per connection. The single-channel rule existed to stop queued-shell confusion on App instances; for the sandbox target the many-channel shape _is_ the product, and the cap plus the connection limiter bound exhaustion. `bex_ssh_gateway_*` metrics gain a channel count histogram with the same no-identifying-labels rule.
- **Credential exposure is priced in, not accidental.** The sandbox already holds session-scoped, short-lived credentials by design (ADR047 D2: gateway-refreshed Git token confined to `bex-agent/*` branches; BYO model key). D2 gates shell access on `can_view_sensitive` so only roles that may read sensitive material get it, and the credentials a shell can read are the ones ADR047 already assumes a compromised sandbox process could exfiltrate. No platform credential lives in the pod (`automountServiceAccountToken: false`, gVisor, dedicated node pool).
- **The SFTP bridge is not a file-transfer product.** It exists so Zed can install its server; it necessarily also lets an authorized developer copy files in/out of the sandbox working tree — the same material `git push` to the draft PR already externalizes, and strictly less than the shell they already have on this path. It stays off for `srv-…` App instances, where ADR035's ban is unchanged.
- **Target confusion.** Pod and namespace are derived server-side from the session row (`<sandboxID>-0`, `<ws>-sandbox`) — the identical arithmetic as ticket minting — and the partial unique index on live `sandbox_id` guarantees one session per sandbox. Nothing caller-controlled reaches the exec URL.
- **Fail-closed inventory.** `BEX_SSH_HOST` unset ⇒ no address surfaced; `BEX_SSH_MAX_CHANNELS_PER_CONN=0` ⇒ carve-out off; sandbox not live ⇒ resolver refuses at handshake; FGA relation absent (model not migrated) ⇒ deny; DB grant absent ⇒ resolver errors, connection refused.

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
