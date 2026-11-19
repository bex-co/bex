# w10 · m497 — An agent session that finds the sandbox pool full waits for a machine and says why, instead of failing, as on Devin

**Worker:** worker10 **Goal:** a session or automation run whose sandbox cannot be placed because the platform's sandbox pool is full waits for capacity and starts when it frees, showing that it is waiting on every surface. A start that still fails names its real cause: the pool, an image pull, the runtime or the quota. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: name a pod-ready timeout's cause and wait for a full pool; ADR047, ADR042 and ADR018 amended | 45m | — |
| [t002](t002.md) | Cause: bex-api reads why a sandbox pod is not running | 1h30m | t001 |
| [t003](t003.md) | Wait instead of fail: a dispatch whose pod the pool cannot place retries until capacity frees, within a bound | 2h | t002 |
| [t004](t004.md) | Say it: the waiting state and the start-failure cause on REST, GraphQL and MCP | 1h15m | t003 |
| [t005](t005.md) | Dashboard and `bex agent` show "waiting for a sandbox machine" and the named cause | 1h15m | t004 |
| [t006](t006.md) | Automation runs keep their slot: a run's activity shows its session waiting, and a platform start failure reads as one | 45m | t004 |
| [t007](t007.md) | Pool-full signal: a waiting-sessions metric and an alert, so the pool's size is decided on evidence | 45m | t003 |
| [t008](t008.md) | Live check: a full pool makes the next session wait, then start | 1h30m | t005, t006, t007 |
| [t009](t009.md) | Render parity (Devin) | 30m | t008 |
| [t010](t010.md) | Simplify | 20m | t009 |
| [t011](t011.md) | Test coverage | 45m | t009, t010 |
| [t012](t012.md) | Closeout | 10m | t011 |

Total: **12 tasks, about 11h30m**.

## Definition of done

1. **A full pool makes a session wait, not fail.** A dispatch whose sandbox pod is unschedulable for lack of node CPU or memory keeps the session in its starting phase and creates the sandbox again until it is placed. It does so within a bound t001 sets, which is longer than one 300 s server wait. The session starts when capacity frees. A real-Postgres test with the sandbox fake fails on today's code, which settles the turn `SANDBOX_START_FAILED` after the first timeout. Cancel, Delete and Archive end the wait at once. A wait never leaves two sandboxes for one turn. Replica restarts and rollouts neither end a wait nor double it: dispatch recovery (`agent_session_dispatches.next_check_at`) does not abandon a dispatch that is still waiting.
2. **The cause is named.** When the pod-ready wait ends, bex-api tells apart a pod the pool cannot place (`Insufficient cpu` / `Insufficient memory`), an image that does not pull (`ErrImagePull`, `ImagePullBackOff`), a runtime that does not start (`FailedCreatePodSandBox`, a missing `RuntimeClass` handler), the namespace quota, and an unknown cause. A start that still fails records its cause on the session's failure, not "sandbox did not start (platform issue)" alone. `SANDBOX_CAPACITY_LIMIT` keeps its plan-limit meaning.
3. **Every surface says it.** REST, GraphQL and MCP expose the waiting state and the cause, with the same fields and words. The dashboard session page and list, and `bex agent get`, `logs` and `run`, show "waiting for a sandbox machine" while waiting and the named cause on failure, in all eight locales.
4. **Automations keep their slot.** An automation run whose session is waiting shows that session waiting in its activity. A run whose session failed to start for a platform cause reads that way, apart from a failed template.
5. **Operators see a full pool.** A metric counts the sessions waiting for capacity, and an alert fires when sessions keep waiting. The alert is documented next to ADR105's pool-size decision.
6. **Live.**
   - On a dev stack (or the shared cluster's agent-session leg when it is free), the pool is filled and one more session is started. That session waits with the cause shown, and it starts once a running session is canceled.
   - On production, once a deploy carries this milestone, a day of bursts records waits and no `SANDBOX_START_FAILED` whose cause is a full pool. The burst is the routines' hourly starts or the QA user's sessions, with no deliberate fill while customers' sessions are running.

## Source + Goal linkage

- **Source:** inbox note [w10/015](../done/015.md), filed from the agent-automation runs of 2026-11-15..18 and promoted by `/loopx w10` on 2026-11-18.
- **Triage evidence (production, read-only, 2026-11-18):**
  - **The cause is the sandbox pool's memory, not a platform fault.** All six failed sessions in the note hit `KUBERNETES::POD_READY_TIMEOUT`: three at 08:18:52–53 UTC on 2026-11-17 and three at 10:27:39, 10:29:08 and 10:30:38. Their pods were created and then sat `Pending - Sandbox is being created` for 300 s on the only sandbox node, `bex-sandbox-mzp4j-htm8z` (a fixed `cpx42`, 7 CPU and 13.2 GiB allocatable). The OpenSandbox server then deleted them (`Creation failed, cleaning up sandbox …: 504`).
  - **The timeline is in Prometheus.** At 08:15–08:19, kube-state-metrics shows exactly three unschedulable `*-sandbox` pods (`kube_pod_status_unschedulable`), each requesting 500m and 1 GiB. The node's memory requests stood at 14.20 GB of 14.22 GB allocatable. At 10:23–10:30, one to three pods were unschedulable at 96% of memory requested. The single failure at 2026-11-18 20:07:39 (`ags-dbuvn349rejs73fsji7g`) matches one unschedulable pod at 92%.
  - **Nothing grows the pool.** The `bex-sandbox` MachineDeployment has `replicas: 1` and no autoscaler annotations. ADR105 records the user's choice of one cpx42 (§ Until the AX42-1 arrives), and ADR042 § Sandbox groups already says that a create the pool cannot place "waits for the pod and then answers `503 SANDBOX_START_FAILED`".
  - **Quota is not the cause.** The `tea-…-sandbox` ResourceQuota allows 2,058 CPU of requests. bex-api has no read on pods in `<ws>-sandbox` namespaces (`kubectl auth can-i list pods --as=system:serviceaccount:bex-system:bex-api` answers `no`), so it cannot tell these causes apart today. `podReadyTimeoutError` (`lego/backend/internal/sandbox/service.go`) compares only plan limits.
  - **Nothing retries.** `dispatch` records the failure and settles the turn after one create (`lego/backend/internal/agentsessions/service.go`, `bootSandbox`). An automation run is settled `started` as soon as the session is accepted (`decideAutomationRun`, `automation_scheduler.go`), so its slot is lost with the session.
- **Reference product (Devin):**
  - A session Devin cannot place waits for capacity and starts when capacity frees. It is never failed for it.
  - [Devin release notes](https://docs.devin.ai/release-notes/overview), read 2026-11-18:
    - 2026-06-05: "Send Messages While Session Is Queued" to a session "that is waiting for capacity".
    - 2026-08-12: Slack posts a notice when a session is "waiting for machine capacity".
    - 2026-09-11: the Computer tab says the machine is being provisioned and "waits up to roughly five minutes for slow-starting machines".
    - 2026-09-30: "Sessions on an outpost now wait for the outpost's machine to come online instead of failing."
  - Devin automations queue runs past their concurrency instead of dropping them ([Automations](https://docs.devin.ai/product-guides/automations)).
- **Not in this milestone:**
  - **Transient create faults stay in [w1/m405](../../w1/m405/README.md).** These are a dropped connection, a 502/503/504, the runtime not answering, or the OpenSandbox server's `KUBERNETES::API_ERROR` `RemoteDisconnected` 500s (six on 2026-11-16..17, at the routines' :00 and :30 starts). m405's DoD keeps `SANDBOX_START_FAILED` failing at once, and this milestone changes exactly that case. Its t002 ("never two sandboxes for one turn") is the safety this milestone's retry needs too, so t003 reuses it if it has shipped and builds the same guarantee if it has not.
  - **The pool's size stays the user's.** Its autoscaling and node count are ADR105's cost decision, and this milestone's behavior does not depend on them. t007 gives that decision its evidence.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)). Automations are the main unattended use, and a run dropped for a busy node delivers nothing.
- **Expected outcome:**
  - A burst of starts, such as an automation fleet or several users at once, waits for the pool instead of losing a third of its sessions.
  - A start that still fails says why: the pool, an image pull, the runtime or the quota. Today it says "platform issue".
- **Why now:** 7 of the 17 agent-session sandbox create failures on production (2026-11-11..18) are this case, all since 2026-11-17. Seventeen scheduled automations already share one node.
- **Render parity included (reference product: Devin):** the session view gains a waiting state and a cause on REST, GraphQL and MCP, and the dashboard and `bex agent` show them.
