# w9 · m349 — Agent sessions survive a sandbox-node hiccup: transient exec failures retry, a lost container's reason is kept, and sandbox logs are shipped

**Worker:** worker9 **Goal:** an agent session is failed only when its sandbox is really gone, and when it is, the failure says why (OOM, exit code). A brief node or network stall no longer turns into a terminal failure plus sandbox deletion, and the sandbox node stops overcommitting itself into stalls. **Status:** blocked (every task but t006 and t011 done and live; t006, the sandbox node capacity, needs the user's go-ahead)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Gateway: an exec into a pod whose node is not Ready answers a distinct "target unavailable" code, not the generic start failure — **DONE** | 45m | — |
| [t002](done/t002.md) | Completer: transient status-read failures retry for a configurable window (10 min), and only a really-gone sandbox fails the session — **DONE** | 90m | t001 |
| [t003](done/t003.md) | A lost sandbox's failure records the container's terminated reason and exit code before the sandbox is deleted — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Ship ssh-gateway, opensandbox and agent-sandbox pod logs to Loki — **DONE** | 30m | — |
| [t005](done/t005.md) | The agent turn bound is inactivity-based, under a hard ceiling no longer than the sandbox lifetime — **DONE** | 60m | — |
| [t006](t006.md) | Sandbox node capacity: kubelet reservations, a longer not-ready toleration for sandbox pods, and enough nodes for the T0/T1 concurrency | 45m | — |
| [t007](done/t007.md) | Live check on production: a long session survives a simulated status-read outage, and an OOM-killed sandbox fails with its reason — **DONE** | 45m | t002, t003, t004 |
| [t008](done/t008.md) | Render parity (Devin): a session's failure states and reasons read the same on REST, GraphQL, MCP and the dashboard — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008, t009 |
| [t011](t011.md) | Closeout | 10m | t010 |

## Definition of done

On production:

- A status-read failure lasting a few minutes (node NotReady, gateway restart) leaves the session working, and it carries on afterwards. Only a sandbox that is really gone fails the session.
- A session whose container died (OOM or a crash) fails with a reason naming it, for example "the sandbox ran out of memory (exit 137)", not "sandbox terminated before completion".
- ssh-gateway, opensandbox and agent-sandbox pod logs are queryable in Loki.
- The sandbox node pool no longer stalls under the workspace's allowed concurrency (t006, after the user's decision).

## Source + Goal linkage

- **Source:** w9/089, promoted 2026-11-12 by `/loopx w9` after a read-only production diagnosis.
  - All five sandboxes deleted on 2026-11-12 were deleted by bex-api itself (opensandbox-server access log).
  - 12:21–12:26 UTC: the single sandbox node `bex-sandbox-cgqq6-p8wcg` (cx33, 4 vCPU, 7.5 GiB, no kubelet reservations) stalled. Cilium stopped answering at 12:22:30, the node was NotReady 12:25:13–12:26:10, and containerd's kill-pod deadlines were exceeded.
  - Agent sandboxes request 500m/1Gi but may use 2 CPU/4Gi (`sandbox/client.go:307`), so two of them overcommit the node.
  - The completer's 2-minute retry window (`agentsessions/completion.go:126`) treats every exec error alike. Through `sandbox/exec.go:511`, a node stall reads as `ErrSandboxesUnavailable` ("sandbox runtime not configured"), so two 1–2 h sessions failed and were deleted.
  - 13:06: `7224297b`'s container had already exited (no `Killing` event; `ReadSessionStatus` maps Terminated to NotFound, `sandbox/service.go:1202-1205`). `failLostSandbox` deleted it at once (`completion.go:553-566`), so the exit reason was lost. The gateway and sandbox logs are not shipped (`deploy/gitops/base/log-shipper.yaml`).
  - The turn cap is a wall-clock `setTimeout` (`driver/src/session.ts:220-224`). Production raised it to 4 h on 2026-11-12 (`9a6904a27`); the 2026-11-11 "ACP turn exceeded 1800000ms" failures came before that.
- **Goal linkage:** pillar 5 (cloud coding agents). Devin sessions run for hours and survive platform hiccups; scheduled automations (ADR101) depend on it.
- **Expected outcome:** the nightly automations finish their work instead of failing minutes in, and a failure that does happen names its cause.
- **Why now:** every agent session on production after 10:00 UTC on 2026-11-12 failed. The workspace's eight nightly automations are the first heavy, long-running load.
- **Gate:** t006 changes production node capacity: a second sandbox node or a larger instance type costs money, and kubelet reservations roll the node. It needs the user's go-ahead; the code tasks do not.
- **Parity:** included (t008), because failure reasons are read on REST, GraphQL, MCP and the dashboard.
