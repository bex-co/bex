# w5 · m99 — No sandbox outlives its owner: bound lifetimes and reconcile the live inventory

**Worker:** worker5 **Goal:** every sandbox on the platform is reachable by something that will eventually stop it, so a failed teardown costs minutes of compute instead of running forever. **Status:** todo

## Live evidence (production, 2026-09-15)

Workspace `bex` / `tea-d98210cbbpdc73dcrkvg` holds one sandbox: `271ec9ce-a32b-4128-bb43-02a9e57b01b6`, `status: running`, `plan: starter`, `timeoutSeconds: 0`, image `ghcr.io/bex-co/bex-agent-sandbox@sha256:e7adb8cb…`, created `2026-08-30T02:20:56Z` — **403 hours ago**.

Its agent session `ags-da9p720k98cs738k20c0` (`repo bex-co/web-beancount`, branch `bex-agent/m83-response-budget-e2e` — the w5/m83 E2E session) was created in the **same second** and was **canceled at `2026-08-30T02:27:32Z`**, seven minutes later. The session detail returns no `sandboxId` at all.

The cost is real and ongoing. `GET /v1/usage?ownerId=tea-…&period=2026-09` attributes to that one sandbox:

```json
{
  "serviceId": "271ec9ce-a32b-4128-bb43-02a9e57b01b6",
  "resourceKind": "sandbox",
  "costUsd": "77.55",
  "charges": [
    {
      "kind": "sandbox_compute_seconds",
      "tier": "starter",
      "unit": "vCPU-hr",
      "rateUsd": "0.0895",
      "quantity": "866.53",
      "costUsd": "77.55"
    }
  ]
}
```

That is **$77.55 of the $77.85** of all sandbox spend this period, and ~31% of the workspace's `billing.currentCost.amountUsd` of `$247.72` (currently absorbed by a $1000 credit grant, so no cash has left yet — the grant is masking it). The meter itself is correct: 866.53 vCPU-hr over ~357 hours of September is what a continuously-running `starter` sandbox accrues.

## Root cause — three independent gaps, each sufficient on its own

1. **Cancel is a no-op when the row forgot the sandbox.** `Service.Cancel` (`lego/backend/internal/agentsessions/service.go:1169-1173`) terminates only `if record.SandboxID != ""`. A steer/redispatch blanks that column (`internal/store/agentsessions.go:212` — `SET sandbox_id='', phase=$2, …`) and hands the old id to a **best-effort, log-only** teardown (`service.go:1476-1479`, "teardown of previous sandbox failed … %v" then continue). Once that log line is written the row no longer references the sandbox, and a later cancel silently reclaims nothing.
2. **The deferred-teardown reaper's scan window is anchored to `now`, not to the row.** `Completer.reapIdleSandboxes` (`internal/agentsessions/completion.go:319-328`) calls `ListTerminalAgentSessionsWithSandbox(ctx, now - (2*sshGraceTTL + idleTTL))` — with the shipped defaults `defaultSSHGraceTTL = 4h` (`completion.go:217`) and `BEX_AGENT_SANDBOX_IDLE_TTL` default `30m` (`cmd/api/config.go`), that is an **8h30m** window. A teardown that keeps failing does not advance `updated_at`, so the row ages out of the scan and is never retried again. The store comment (`internal/store/agentsessions.go:474-480`) states the assumption this violates: the recency bound "skips pre-feature history (whose sandboxes are long gone)".
3. **Nothing bounds a sandbox's lifetime, so there is no backstop.** `validateCreateMetadata` (`internal/sandbox/service.go:160-173`) documents `timeoutSeconds: 0` as **"0 (no expiry)"**, and `bex.co/timeout-seconds` is written as OpenSandbox metadata (`service.go:197`) and only read back for display (`service.go:321`) — no bex component enforces it and there is no time-based sweep. This is also a **pinned-client contract divergence**: upstream's flag help reads "Maximum sandbox lifetime in seconds. 0 uses the default and maximum of 86400 (24 hours)" (`cmd/sandboxcreate.go:104`) and the client omits the field entirely when it is not positive (`pkg/sandbox/service.go:83-84`). So `bex ea sandboxes create` with default flags asks for Render's 24-hour cap and gets an immortal sandbox.

`sandbox.WorkspacePurger` (`internal/sandbox/purger.go`) is the only other sweep and it runs solely on **workspace delete**, so it cannot help a live workspace.

## Tasks (in order)

| id   | title                                                                             | est | depends_on             |
| ---- | --------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Enforce a bounded sandbox lifetime and reconcile `timeoutSeconds: 0` with the pin | 60m | —                      |
| t002 | Reap by age from the row, not from a window anchored to `now`                     | 45m | —                      |
| t003 | Reconcile the live sandbox inventory against session rows                         | 75m | t002                   |
| t004 | Make the steer/redispatch previous-sandbox teardown durable                       | 45m | t003                   |
| t005 | Surface orphans: an over-age-sandbox metric and an alert                          | 45m | t003                   |
| t006 | Reclaim the production orphan and record its cost (needs explicit user approval)  | 20m | t003                   |
| t007 | Render parity across REST/GraphQL/MCP and the dashboard                           | 30m | t001, t004, t005, t006 |
| t008 | Simplify                                                                          | 20m | t007                   |
| t009 | Test coverage                                                                     | 45m | t007                   |
| t010 | Closeout                                                                          | 10m | t009                   |

## Definition of done

- A sandbox created with no explicit timeout is stopped automatically by a bounded lifetime; the effective bound is returned on REST/GraphQL/MCP and the `timeoutSeconds: 0` semantics either match the pinned client's documented 24-hour default/maximum or are recorded as a deliberate divergence in `docs/ADR018-render-parity.md` and `docs/cli-compatibility-checklist.md`.
- A running sandbox with no live session that owns it is reclaimed within one reconcile interval, regardless of how old it is, whether its session row still carries `sandbox_id`, and how many prior teardown attempts failed.
- A teardown that fails is retried until it is confirmed, and its failure is visible as a metric with a loaded alert rather than only as a log line.
- Production holds no sandbox whose owning session is terminal; the `271ec9ce-…` orphan is stopped (after explicit user approval) and its total cost is recorded in this milestone.
- Regression tests fail on `main` today and pass after the fix: a canceled session whose `sandbox_id` was blanked, a terminal session aged past `2*sshGraceTTL + idleTTL`, and a sandbox with no session row at all are all reclaimed.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` hunt against `https://api.bex.co/v1/`, sweep 2, 2026-09-15 UTC (human device login as the QA identity, isolated CLI config, workspace `bex`). Found by inventorying `bex ea sandboxes list` and correlating the one running sandbox against `GET /v1/agent-sessions` and `GET /v1/usage`.
- **Goal linkage:** pillar 5 (hosted sandboxes / cloud coding-agent sessions, [ADR014](../../docs/ADR014-sandboxes.md), [ADR042](../../docs/ADR042-sandbox-cluster-substrate.md), [ADR047](../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR054](../../docs/ADR054-open-in-zed.md) D6, [ADR059](../../docs/ADR059-agent-sandbox-hibernation.md)) and billing integrity ([ADR023](../../docs/ADR023-usage-metering.md), [ADR040](../../docs/ADR040-billing-metronome.md)). bex's economics rest on dense bin-packing and reclaiming idle compute; a resource class that can run forever with no owner breaks both.
- **Expected outcome:** the live-sandbox inventory equals the set of sandboxes some session or user still wants, continuously and observably. Orphan cost drops from "unbounded until someone notices" to at most one reconcile interval.
- **Why now:** it is live on production and accruing — one orphan is already ~31% of the workspace's monthly bill, and it is invisible precisely because a $1000 credit grant is absorbing it. The same gaps apply to every tenant once signup opens, and the sandbox is a live agent-image workload holding whatever state its canceled session had, so this is a security-surface question as well as a cost one.
- **Render parity task included:** the fix changes the sandbox create contract (`timeoutSeconds`) and the agent-session lifecycle, both of which are exposed on REST, GraphQL, MCP, and the dashboard, so the standing parity check applies.

## Dedupe

`rg` over open and `done/` `.pm` for orphan + sandbox: `w1/m61` (workspace-delete teardown — only on delete, cannot reach a live workspace), `w5/m85` (crash-safe dispatch — recovers rows with `sandbox_id=''` in phases `creating`/`redispatching` via `agent_session_dispatches`, not terminal sessions), `w2/m64` t001 (cancel racing an in-flight dispatch), `w5/m80` t003 (no orphaned pod from the fast-fail path). None covers a terminal session whose `sandbox_id` was blanked by a steer, the reaper's `now`-anchored window, or an unbounded `timeoutSeconds`. `docs/cli-compatibility-checklist.md:270` grades `ea sandbox create` without touching timeout semantics. No `DO_NOT_DO.md` item applies — pillar 5 was re-opened 2026-07-27.
