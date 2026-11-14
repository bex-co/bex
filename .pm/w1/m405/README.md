# w1 · m405 — An agent session survives a transient sandbox-create failure

**Worker:** worker1 **Goal:** a session whose sandbox creation hits a transient fault (the OpenSandbox server restarting, a node draining in a roll, a dropped connection) is retried within a bound instead of failing the user's turn with `sandbox create failed`. **Status:** todo

## Tasks (in order)

| id   | title                                                          | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | Find and classify the 2026-11-12 09:36 UTC create failure      | 30m | —          |
| t002 | Make a repeated create safe: never two sandboxes for one turn  | 60m | t001       |
| t003 | Retry transient create failures within a bound                 | 60m | t002       |
| t004 | Keep the sandbox control plane serving through node rolls      | 45m | t001       |
| t005 | Live check: restart the sandbox control plane during a start   | 45m | t003, t004 |
| t006 | Simplify                                                       | 15m | t005       |
| t007 | Test coverage                                                  | 30m | t005       |
| t008 | Closeout                                                       | 10m | t007       |

## Definition of done

1. Automated tests: a create that fails with a connection error, a `502`/`503`/`504` or the runtime-unavailable answer is retried (bounded attempts and total time, well inside the dispatch budget) and the session reaches running; a non-transient failure (capacity limit, start failure, other `4xx`) still fails at once with its current reason and code.
2. A retry never leaves two sandboxes for one session turn: it first looks up a sandbox the earlier attempt created (by the session binding labels) and reuses a running one or removes a half-created one.
3. On a dev cluster, restarting the sandbox control plane (or draining its node) while a session starts leaves the session running, with the retry in the bex-api log.
4. If t001 shows the control plane was down through the roll, it keeps serving through a platform roll (replicas and a PodDisruptionBudget), or the README records why that is not possible.

## Source + Goal linkage

- **Source:** the w1/144 kernel roll, [`.pm/w1/blocked/144.md`](../blocked/144.md) step 3: one QA session failed with `sandbox create failed` at 09:36 UTC on 2026-11-12 during the platform roll, and a probe at 09:42 succeeded. Code: `lego/backend/internal/agentsessions/service.go` dispatch calls `CreateAgentSessionSandbox` once and abandons the turn on any error; `lego/backend/internal/sandbox/client.go` repeats only unanswered GETs (`defaultReadRetryBackoff`), and the synchronous create can take up to 330 s. `lego/backend/internal/sandbox/service.go` `CreateAgentSessionSandbox` labels the sandbox with `agentsession.BindingLabels` plus the dispatch turn, so an earlier attempt can be found. The OpenSandbox controller runs one replica (`deploy/gitops/charts/opensandbox-controller/values.yaml` `replicaCount: 1`).
- **Goal linkage:** [ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md) cloud coding-agent sessions; [ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md) sandbox substrate. Devin is the reference: an infrastructure hiccup on its side does not fail the user's turn.
- **Expected outcome:** platform rolls, upgrades and brief control-plane restarts no longer turn into failed agent turns.
- **Why now:** w1/144 still has nodes to roll (control plane, platform, tenant-lg, sandbox), and each roll can fail any session that starts during it.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard shape changes; failure reasons and codes stay as they are.
