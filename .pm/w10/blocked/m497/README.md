# w10 · m497 — An agent session that finds the sandbox pool full waits for a machine and says why, instead of failing, as on Devin

**Worker:** worker10 **Goal:** a session or automation run whose sandbox cannot be placed because the platform's sandbox pool is full waits for capacity and starts when it frees, showing that it is waiting on every surface. A start that still fails names its real cause: the pool, an image pull, the runtime or the quota. **Status:** blocked (t001–t007, t009–t011 done 2026-11-18; t008, the live check, waits for the shared cluster's agent-session leg, held by `dev-9`, then for a production deploy carrying this commit; see "Gate: t008")

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: name a pod-ready timeout's cause and wait for a full pool; ADR047, ADR042 and ADR018 amended — **DONE** | 45m | — |
| [t002](done/t002.md) | Cause: bex-api reads why a sandbox pod is not running — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Wait instead of fail: a dispatch whose pod the pool cannot place retries until capacity frees, within a bound — **DONE** | 2h | t002 |
| [t004](done/t004.md) | Say it: the waiting state and the start-failure cause on REST, GraphQL and MCP — **DONE** | 1h15m | t003 |
| [t005](done/t005.md) | Dashboard and `bex agent` show "waiting for a sandbox machine" and the named cause — **DONE** | 1h15m | t004 |
| [t006](done/t006.md) | Automation runs keep their slot: a run's activity shows its session waiting, and a platform start failure reads as one — **DONE** | 45m | t004 |
| [t007](done/t007.md) | Pool-full signal: a waiting-sessions metric and an alert, so the pool's size is decided on evidence — **DONE** | 45m | t003 |
| [t008](t008.md) | Live check: a full pool makes the next session wait, then start | 1h30m | t005, t006, t007 |
| [t009](done/t009.md) | Render parity (Devin) — **DONE** | 30m | t008 |
| [t010](done/t010.md) | Simplify — **DONE** | 20m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009, t010 |
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

- **Source:** inbox note [w10/015](../../done/015.md), filed from the agent-automation runs of 2026-11-15..18 and promoted by `/loopx w10` on 2026-11-18.
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
  - **Transient create faults stay in [w3/m405](../../../w3/m405/README.md).** These are a dropped connection, a 502/503/504, the runtime not answering, or the OpenSandbox server's `KUBERNETES::API_ERROR` `RemoteDisconnected` 500s (six on 2026-11-16..17, at the routines' :00 and :30 starts). m405's DoD keeps `SANDBOX_START_FAILED` failing at once, and this milestone changes exactly that case. Its t002 ("never two sandboxes for one turn") is the safety this milestone's retry needs too, so t003 reuses it if it has shipped and builds the same guarantee if it has not.
  - **The pool's size stays the user's.** Its autoscaling and node count are ADR105's cost decision, and this milestone's behavior does not depend on them. t007 gives that decision its evidence.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)). Automations are the main unattended use, and a run dropped for a busy node delivers nothing.
- **Expected outcome:**
  - A burst of starts, such as an automation fleet or several users at once, waits for the pool instead of losing a third of its sessions.
  - A start that still fails says why: the pool, an image pull, the runtime or the quota. Today it says "platform issue".
- **Why now:** 7 of the 17 agent-session sandbox create failures on production (2026-11-11..18) are this case, all since 2026-11-17. Seventeen scheduled automations already share one node.
- **Render parity included (reference product: Devin):** the session view gains a waiting state and a cause on REST, GraphQL and MCP, and the dashboard and `bex agent` show them.

## Progress 2026-11-18 (/loopx w10)

t001–t007 and t009–t011 shipped together with this board move (one commit). What is true now, against the definition of done:

- **Item 1 (wait, not fail), as code and tests.**
  - `createWaitingForRoom` (`agentsessions/sandbox_wait.go`) creates again while the start fails with cause `pool_full`. It waits 15 s between creates (each create is itself the server's 300 s wait) for up to an hour, never past the turn ceiling. Before each new create it ends the turn's leftover sandboxes.
  - `MarkAgentDispatchWaiting` stamps `agent_sessions.sandbox_wait_since` (migration 0301) and moves `next_check_at` 15 minutes ahead before every create.
  - Real-Postgres tests with the sandbox fake:
    - `TestAFullPoolMakesTheSessionWaitThenStart` fails on today's code ("read waiting 0 times").
    - `TestACancelDuringTheWaitEndsItWithNoSandbox`, `TestACancelEndsTheCreateInFlight` and `TestAnArchiveDuringTheWaitEndsIt` cover Cancel and Archive.
    - `TestARecoveryTickDuringTheWaitDoesNotAbandonIt` fails with the keep-alive reverted.
    - `TestTheBoundPassingFailsWithThePoolFullCause`, `TestAnImagePullFailureFailsAtOnce` and `TestARedispatchWaitsToo` cover the bound, the other causes and a redispatch.
- **Item 2 (cause).**
  - A new read-only ClusterRole, `bex-tenant-sandbox-api` (pods get/list), is bound to bex-api per `<ws>-sandbox`, with its admission and `gitops-validate.sh` guards.
  - `startWatch` (`sandbox/start_cause.go`) reads the attempt's pod every 10 s while the create waits. It maps the pod to `pool_full`, `image_pull`, `runtime`, `quota` or `unknown`, and a refused read is `unknown`.
  - The cause rides `SANDBOX_START_FAILED` as `params.cause`. `SANDBOX_CAPACITY_LIMIT` is unchanged.
  - Tests: `TestEachPodShapeNamesItsCause`, `TestTheLastPodSeenNamesTheCauseAfterTheServerDeletesIt`, `TestAPodSeenOnAMachineNeverBlamesThePlan`, `TestNoPodAndAFullQuotaIsTheQuota`, `TestARefusedReadIsUnknown` and `TestARenderSandboxTimeoutAnswersTheCause`.
- **Item 3 (surfaces).**
  - `waitingFor`, `waitingSince` and `failureCause` are on REST, GraphQL and MCP from one `startState` (`TestEverySurfaceServesTheWaitAndTheCause`).
  - The dashboard's session page header, chat column, recents, table and sidebar say "Waiting for a sandbox machine · 3m 12s", and a failed start shows its cause line, in all eight locales.
  - `bex agent get`, `logs`, `run --wait/--follow` and `wait` print the waiting line once, and a failure prints `SANDBOX_START_FAILED (pool_full): …`.
- **Item 4 (automations).**
  - An automation run's activity carries `session {state, waitingFor, waitingSince, failureReasonCode, failureCause}` on all three APIs (`TestARunShowsItsSessionWaitingAndWhyItDidNotStart`).
  - The run history reads "Waiting for a sandbox machine", or "Didn't start" with the cause, apart from `failed_create`.
  - A platform start failure never pauses the automation (`TestAPlatformStartFailureNeverPausesTheAutomation`).
- **Item 5 (signal).**
  - The metrics are `bex_agent_session_sandbox_waiting{cause}`, `bex_agent_session_sandbox_wait_seconds{outcome}` and a new `waiting` outcome on `bex_agent_session_provision_seconds`.
  - `AgentSandboxPoolFull` fires after 15 minutes of waiting, with promtool tests. `AgentSessionProvisionFailing` leaves waits out of its ratio.
  - The panel "Agent sessions waiting for a sandbox machine" is on the bex-api dashboard. ADR105 § Until the AX42-1 arrives names the alert as the pool-size decision's evidence.
- **Item 6 (live): not yet,** see the gate below.
- **Deviations, stated.**
  - A wait lives in the replica that accepted the turn. Recovery and other replicas never end or double it, but the waiting replica's own exit ends it as "interrupted" within 15 minutes, as for any dispatch in flight. A wait that survives its own replica's restart needs the dispatch's inputs stored, and is filed as [w10/022](../../done/022.md).
  - Events are not read (pods get/list is the least privilege that names every cause in the DoD).
  - The visual check of the waiting state ran on the offline stub (`dashboard/scripts/local-bex.mjs` gained a waiting and a pool-full fixture) for the list rows and sidebar at 1440 px and 390 px. They show no layout shift.
  - The stub cannot render a session page for any fixture (its turns query is missing, which predates this work). The header and failure callout rest on their component tests, and on t008's dashboard step.
- **Parity (t009).**
  - The same three fields and words are on the session view, the session list and the run activity in REST, GraphQL and MCP, the dashboard and `bex agent`.
  - Devin is matched: a session waiting for capacity starts by itself; a message sent to it queues (`liveSandboxPhase` covers `creating`), as in Devin's "Send Messages While Session Is Queued"; and automation runs queue behind a waiting session through the overlap rule.
  - No drift was found beyond w10/022.
- **Suites run:**
  - `lego/backend` `go test -p 1 ./...` against real Postgres, OpenFGA, OpenBao and Loki.
  - `make lint` (four modules) and `cd lego/cli && go test ./...`.
  - Dashboard `yarn lint` and `yarn test`, and promtool rule tests.
  - `scripts/gitops-validate.sh`, whose one local FAIL comes from an untracked `lego/operator/bin/manager` binary matching a retired-string grep, not from this change.

## Gate: t008 (the live check) — BLOCKED 2026-11-18

**Needs:**

- The shared kind/CAPD cluster's agent-session leg, which is cluster-wide and exclusive, released by its holder: `bash scripts/dev-env.sh 9 agent-down` from w9's checkout, or the user's say-so that w10 may take it over. On 2026-11-18 `opensandbox-system` carries `app.bex.co/dev-env=dev-9` (w10/m491 waits on the same leg). A worker is read-only on another workstream's stack.
- Then step 5's production read, which needs a deploy carrying this commit: check `deploy/gitops` pins an image built from it.

**Next worker, in order:**

1. Confirm the leg is free: `kubectl get ns opensandbox-system -o jsonpath='{.metadata.labels.app\.bex\.co/dev-env}'` is empty or `dev-10`.
2. Bring it up: `bash scripts/dev-env.sh 10 up`, then `agent-up` and `agent-stub`.
3. Run t008 steps 1–4: fill the sandbox node with `stub-hold` sessions, start one more, watch it wait on every surface, cancel a holder, and watch it start. Also give one session an unpullable image.
4. Run step 5 once production runs this commit.
5. `bash scripts/dev-env.sh 10 down`, then t012.
