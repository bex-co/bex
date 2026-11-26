# w3 · m405 — An agent session survives a transient sandbox-create failure

**Worker:** worker3 **Goal:** a session whose sandbox creation hits a transient fault (the OpenSandbox server restarting, a node draining in a roll, a dropped connection) is retried within a bound instead of failing the user's turn with `sandbox create failed`. **Status:** blocked (t001–t004, t006, t007 done; t005 needs the dev cluster's agent-session leg)

## Tasks (in order)

| id   | title                                                          | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | Find and classify the 2026-11-12 09:36 UTC create failure — **DONE** | 30m | —          |
| t002 | Make a repeated create safe: never two sandboxes for one turn — **DONE** | 60m | t001       |
| t003 | Retry transient create failures within a bound — **DONE**      | 60m | t002       |
| t004 | Keep the sandbox control plane serving through node rolls — **DONE** | 45m | t001       |
| t005 | Live check: restart the sandbox control plane during a start   | 45m | t003, t004 |
| t006 | Simplify — **DONE**                                            | 15m | t005       |
| t007 | Test coverage — **DONE**                                       | 30m | t005       |
| t008 | Closeout                                                       | 10m | t007       |

## Definition of done

1. Automated tests: a create that fails with a connection error, a `502`/`503`/`504` or the runtime-unavailable answer is retried (bounded attempts and total time, well inside the dispatch budget) and the session reaches running; a non-transient failure (capacity limit, start failure, other `4xx`) still fails at once with its current reason and code.
2. A retry never leaves two sandboxes for one session turn: it first looks up a sandbox the earlier attempt created (by the session binding labels) and reuses a running one or removes a half-created one.
3. On a dev cluster, restarting the sandbox control plane (or draining its node) while a session starts leaves the session running, with the retry in the bex-api log.
4. If t001 shows the control plane was down through the roll, it keeps serving through a platform roll (replicas and a PodDisruptionBudget), or the README records why that is not possible.

## Source + Goal linkage

- **Source:** the w1/144 kernel roll, [`.pm/w1/blocked/144.md`](../../../w1/blocked/144.md) step 3: one QA session failed with `sandbox create failed` at 09:36 UTC on 2026-11-12 during the platform roll, and a probe at 09:42 succeeded. Code: `lego/backend/internal/agentsessions/service.go` dispatch calls `CreateAgentSessionSandbox` once and abandons the turn on any error; `lego/backend/internal/sandbox/client.go` repeats only unanswered GETs (`defaultReadRetryBackoff`), and the synchronous create can take up to 330 s. `lego/backend/internal/sandbox/service.go` `CreateAgentSessionSandbox` labels the sandbox with `agentsession.BindingLabels` plus the dispatch turn, so an earlier attempt can be found. The OpenSandbox controller runs one replica (`deploy/gitops/charts/opensandbox-controller/values.yaml` `replicaCount: 1`).
- **Goal linkage:** [ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md) cloud coding-agent sessions; [ADR042](../../../../docs/ADR042-sandbox-cluster-substrate.md) sandbox substrate. Devin is the reference: an infrastructure hiccup on its side does not fail the user's turn.
- **Expected outcome:** platform rolls, upgrades and brief control-plane restarts no longer turn into failed agent turns.
- **Why now:** w1/144 still has nodes to roll (control plane, platform, tenant-lg, sandbox), and each roll can fail any session that starts during it.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard shape changes; failure reasons and codes stay as they are.

## Record

### t001 — the cause (2026-11-26, production Loki)

- **Class: the control plane restarting.** bex-api logged `sandbox runtime unreachable (POST /sandboxes): … dial tcp 10.108.221.93:8077: connect: connection refused` at 09:36:26 and 09:36:46 UTC, each followed by `agent-session dispatch: sandbox create failed (…): the sandbox runtime is not answering right now; retry in a few seconds`, which is `SANDBOX_RUNTIME_UNAVAILABLE`. Every bex-api read of the server was refused from 09:35:11 to about 09:37 (39 refused requests in the minute to 09:36, 56 in the minute to 09:37, none after), while the drained platform node's `opensandbox-server` pod moved and its volume reattached. A second gap of about 20 s followed at 10:09 (a new server pod started 10:09:17). Reproduce: `{namespace="bex-system", service="bex-api"} |= "sandbox runtime unreachable"` over 2026-11-12 09:00–11:00 UTC; the server's own logs reach Loki only from 10:09.
- **Replicas and disruption budget.** `opensandbox-server` (`deploy/opensandbox/server-in-cluster.yaml`): 1 replica, `strategy: Recreate`, no PodDisruptionBudget. `opensandbox-controller-manager` (chart `deploy/gitops/charts/opensandbox-controller`): 1 replica with leader election on (`--leader-elect`, lease `2fa1c467.opensandbox.io`), no PodDisruptionBudget. Both ran on the same platform node of three.

### t002–t003 — what shipped

- `sandbox.IsTransientCreateFailure`: `SANDBOX_RUNTIME_UNAVAILABLE`, any other 503, the server's `KUBERNETES::API_ERROR` 500 (the RemoteDisconnected creates of 2026-11-16..17 in ADR105 and w32/m285 t009), a 502 or 504 without the server's code, and a connection that ended before the answer. Every other failure fails the turn at once with its current reason and code.
- `agentsessions.createWaitingForRoom` repeats such a create after 2, 5, 10, 20, 30, 30, 30 and 30 s (157 s), never starting one more than 3 minutes after the first failure, and only while the session still waits for the turn. Each repeat is logged (`repeat N of 8`) and counted (`bex_agent_session_sandbox_create_repeats_total`); the dispatch stays one provisioning sample. A retained-disk dispatch does not fall back to a fresh sandbox for it.
- Each repeat sets `AgentSessionSandbox.Repeat`: it lists the turn's sandboxes by `bex.co/agent-session` and `bex.co/agent-dispatch-turn`, takes a running one as its answer and ends any other before creating ([ADR047 § w3/m405](../../../../docs/ADR047-cloud-coding-agent-sessions.md#w3m405--a-create-the-sandbox-runtime-did-not-serve-is-repeated-2026-11-26)).

### t004 — the control plane through a roll

- **The lifecycle server cannot keep serving through a drain.** OpenSandbox 0.2.2's only store is SQLite (`StoreConfig.type: Literal["sqlite"]`, read in the production image), on a ReadWriteOnce volume, so a second replica or a surge pod cannot share it, and a PodDisruptionBudget on one replica would only stall the drain. Its gap is what t003's repeats ride out ([ADR042 D1](../../../../docs/ADR042-sandbox-cluster-substrate.md)).
- **The controller now does.** Two leader-elected replicas, preferred on different platform nodes, with a `maxUnavailable: 1` PodDisruptionBudget (new chart template, enabled in `deploy/gitops/base/values/opensandbox-controller.values.yaml`; `scripts/dev-env.sh`'s single-replica, no-leader-election install keeps the chart defaults). `scripts/gitops-validate.sh` fails if production loses either.

### t006–t007

- `/simplify`: one wording fix and a gitops-validate ordering bug; the rest was clean.
- Tests: `lego/backend/internal/sandbox/agent_repeat_test.go` (which answers are transient; a lost answer's repeat keeps the turn's one sandbox; a half-made or failed one is ended first; a first create never looks), `lego/backend/internal/agentsessions/create_repeat_test.go` (transient then running, a redispatch, non-transient failures at once with their reasons, the bound with today's reason, a cancel during the pause, the bounds), and a retained-disk case in `retention_pg_test.go`. Each failed with its fix reverted.

### t005 — gated (2026-11-26)

The drill needs the dev cluster's agent-session leg (OpenSandbox server and controller, ssh-gateway), which is cluster-wide and exclusive. It is held by dev-9: `opensandbox-system` is labelled `app.bex.co/dev-env=dev-9` and dev-9's bex-api has run since 2026-11-24. Restarting its server would break another workstream's stack. Once w9 or the user releases it (`bash scripts/dev-env.sh 9 agent-down`): `bash scripts/dev-env.sh 3 agent-up`, start a session, `kubectl -n opensandbox-system rollout restart deploy/opensandbox-server` while its create runs, then check the session reaches running with one sandbox for the turn and `repeat 1 of 8` in dev-3's bex-api log; then t008 closes the milestone.
