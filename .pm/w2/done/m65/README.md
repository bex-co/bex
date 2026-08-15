# w2 · m65 — Open in Zed: SSH remoting into agent-session sandboxes (ADR054)

**Worker:** worker2 **Goal:** a user on `dashboard.bex.co/agents/ags-…` clicks **Open in Zed** and gets the session's sandbox `/workspace` open as a Zed remote project over real SSH (`zed://ssh/ags-<xid>@ssh.bex.co/workspace`), reusing the ADR035 gateway end to end. **Status:** done (2026-08-11). t010 live acceptance PASSED on the production gateway — a real Zed client connected into an agent-session sandbox (user-confirmed "能联上了"), after diagnosing + fixing two prod-only gaps the automated suites couldn't catch: the out-of-band `GRANT SELECT ON agent_sessions` was never applied to the gateway DB role, and the sftp bridge ran in `/workspace` instead of `$HOME` so Zed's server upload landed nowhere (fixed in `session.go`, shipped `babf63ba`). Evidence: [`evidence/2026-08-11-live-acceptance.md`](evidence/2026-08-11-live-acceptance.md). t012 simplify: reviewed — the m65 code is small and already minimal, and the one complex piece (the Completer deferral) is deliberately left untouched because w2/m67 rewrites that exact seam into the ADR059 idle model, so simplifying it now would be wasted churn.

## Tasks (in order)

| id   | title                                                                                          | est | depends_on       |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Accept ADR054 + amend ADR035 (multi-channel, `ags-*` branch, SFTP carveout, stale relation)    | 30m | —                | — **DONE** |
| t002 | FGA: add `can_view_sensitive` to the `agent_session` type                                      | 30m | —                | — **DONE** |
| t003 | DB: `GRANT SELECT ON agent_sessions` to `bex_ssh_gateway`                                      | 30m | —                | — **DONE** |
| t004 | Composite `TargetResolver`: `ags-<xid>` → live sandbox pod target                              | 45m | t002, t003       | — **DONE** |
| t005 | nativessh: multi-channel connections for sandbox targets (`BEX_SSH_MAX_CHANNELS_PER_CONN`)     | 45m | t004             | — **DONE** |
| t006 | SFTP subsystem bridge for sandbox targets + `sftp-server` in the agent image                   | 30m | t005             | — **DONE** |
| t007 | Completer teardown deferral while an SSH session is open                                       | 30m | t004             | — **DONE** |
| t008 | `sshAddress` on the agent-session View (REST/GraphQL/MCP) + env docs                           | 30m | t004             | — **DONE** |
| t009 | Dashboard: Open in Zed button + copyable `ssh` command on the session header                   | 30m | t008             | — **DONE** |
| t010 | Live acceptance: real Zed client → live sandbox, full flow evidence                            | 45m | t005, t006, t007, t009 | — **DONE** (PASSED 2026-08-11; see evidence) |
| t011 | Render parity sweep (bex extension row + surface consistency)                                  | 30m | t010             | — **DONE** (bex extension, no Render counterpart; `sshAddress` identical across REST/GraphQL/MCP + dashboard) |
| t012 | Simplify pass over the milestone's changes                                                     | 30m | t011             | — **DONE** (reviewed; deferral seam left for w2/m67's ADR059 refactor) |
| t013 | Test coverage for shipped behavior and failure modes                                           | 30m | t011             | — **DONE** |
| t014 | Closeout                                                                                       | 15m | t013             | — **DONE** |

## Definition of done

With `BEX_SSH_HOST` active and a live agent session: the session page shows **Open in Zed**; clicking it opens Zed, which installs its remote server through the gateway (upload path, sandbox egress unchanged) and browses/edits `/workspace`, with working integrated terminals — while a contributor identity (`can_operate` without `can_view_sensitive`) is refused at SSH handshake, an `srv-…` connection still enforces the single-channel contract byte-identically, a finished turn does not kill the open editor (teardown deferred, bounded by the 4h cap, Cancel overrides), and terminal sessions surface no address. All three test suites green; live evidence recorded in the milestone.

## Source + Goal linkage

- **Source:** [docs/ADR054-open-in-zed.md](../../../docs/ADR054-open-in-zed.md) (Proposed 2026-08-09; deep-research handoff — user: "hand off to /pm for w2 for implement such function"). Design decisions D1–D7 are the task list's spine.
- **Goal linkage:** pillar 5 (agent sessions, re-opened 2026-07-27) — the Devin-style "editor workspace tab" ADR047 D9 names as a differentiator; deepens the ADR035 gateway as bex's one editor/shell transport.
- **Expected outcome:** any Zed user can inspect and edit an agent's working tree with zero bex-side client software; the same `ags-…@ssh.bex.co` address works as plain `ssh` for a one-shot shell.
- **Why now:** agent sessions ship fire-and-forget with weak workspace visibility (ADR051 closed transcripts; files remain opaque); all gateway prerequisites (sandbox-namespace `pods/exec` RBAC, exec-with-stdin, resolver seam) already exist, so the marginal cost is low while the differentiator is large.
- **Render parity task included:** the milestone adds fields to REST/GraphQL/MCP and dashboard UI. Render has no agent-session or Zed surface — t011 records this as a bex extension in the ADR018 gap ledger and checks cross-surface consistency (same `sshAddress` semantics on all three surfaces), rather than comparing to a Render behavior.
- **DO_NOT_DO #22 carveout:** the sandbox-only SFTP bridge (D4) is a user-directed narrow reopening recorded in DO_NOT_DO #22 (2026-08-09); `srv-…` App targets keep the full SFTP/SCP/forwarding ban.
