# w6 · m515 — An SNI-proxy roll lets external Key Value and Postgres clients finish, and the edge keeps admitting new ones

**Worker:** worker6 **Goal:** when a deploy changes `/kv-sni-proxy` or `/pg-sni-proxy`, each node's new proxy pod is Ready before the old one stops, so new external clients keep connecting on every node throughout the roll. The old pod then stops accepting and keeps relaying its open streams until each ends or a bounded grace period passes, so an external command in flight at the roll gets its reply. **Status:** done (2026-11-20: verified on production. Across pin `a47204c7e`, the first later deploy that moved both SNI pins, held clients on both proxies got their replies after `SIGTERM` and closed at the 30 s bound, and no new connection failed. t008 added the manager-exit tests.)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [`sniproxy` drains its open streams](done/t001.md) | 1h | — | — **DONE**
| t002 | [Both SNI proxies drain on `SIGTERM`](done/t002.md) | 45m | t001 | — **DONE**
| t003 | [The SNI DaemonSets surge instead of holding the node's port](done/t003.md) | 1h | t002 | — **DONE**
| t004 | [Prove it on the local cluster](done/t004.md) | 1h30m | t003 | — **DONE**
| t005 | [Record it in ADR021, ADR009 and the operator guide](done/t005.md) | 30m | t003 | — **DONE**
| t006 | [Ship and verify on production](done/t006.md) | 1h30m | t004, t005, w6/m470 t006 | — **DONE**
| t007 | [Simplify](done/t007.md) | 15m | t006 | — **DONE**
| t008 | [Test coverage](done/t008.md) | 30m | t006 | — **DONE**
| t009 | [Closeout](done/t009.md) | 10m | t007, t008 | — **DONE**

## Definition of done

- A test starts `sniproxy`'s drain while a client is mid-command, and the client still reads the backend's reply. A dial after the drain began is not accepted by the old listener. An idle client stays open until the deadline and is closed then. Every limiter slot is released. Cancelling without a drain still closes everything at once.
- `kv-sni-proxy` and `pg-sni-proxy` drain on their first `SIGTERM` for their drain variable and exit as soon as no stream is open. With the variable unset they close at once, as today. A second signal closes everything at once.
- Neither DaemonSet asks for a `hostPort`. Both roll with `maxSurge: 1` and `maxUnavailable: 0`, and `terminationGracePeriodSeconds` covers the bound.
- On the local cluster, a roll of each SNI DaemonSet caused by a binary change lets an external TLS client mid-command get its reply: a Key Value `BLPOP` and a Postgres `pg_sleep`. New external clients keep connecting throughout the roll, and a held idle client is closed no earlier than the chosen bound.
- On production, the shipping deploy rolls each DaemonSet once by surge, and no new external connection fails during it. Across the first later deploy that moves an SNI pin, a held external client mid-command gets its reply through each proxy.
- ADR021 § 10 and ADR009 § SNI proxy state the rule, the bound and its Render basis, the rejected directions and the gap that remains.

## Source + Goal linkage

- **Source:** promoted from inbox note [w6/144](../144.md) on 2026-11-19 by `/loopx w6`. w6/m512 t006 filed it.
- **Goal linkage:** ADR008 § Render parity first.
  - Render breaks a datastore's connections for its own infrastructure only in service-affecting maintenance, which "usually occur[s] no more than once every three months". It announces it by email, and a paid service can usually reschedule it ([platform maintenance](https://render.com/docs/platform-maintenance), read 2026-11-19).
  - bex closes every external Key Value and Postgres connection, without notice, on each deploy that changes an SNI binary. Any `lego/types/v1alpha1` edit does: at most 22 of the 56 production pins from 2026-11-12 to 2026-11-18 (ADR021 § 10). No ADR018 row signs this off.
  - While the old pod holds the node's port, the edge cannot admit new clients on that node either.
- **Expected outcome:** an external command in flight at an SNI-proxy roll gets its reply instead of a reset, and new external clients connect on every node throughout the roll. Held clients (pools, pub/sub, long blocking commands, idle `psql` sessions) still reconnect once, at the end of the bound at the latest.
- **Why now:** w6/m470 holds the SNI images on deploys that leave their code alone. The deploys that do change it still drop every external stream at once, and they are frequent because the binaries link the whole CRD scheme. w6/m512 just shipped the same drain for `kv-internal-proxy`. Activator (w6/m485) and ssh-gateway (w6/m482) drains are the recent precedents.
- **Render parity task omitted:** the change is a platform-internal mechanism. No REST, GraphQL, MCP or dashboard field or message changes. t004 and t006 check the tenant-visible effect.

## Findings at promotion (2026-11-19, code at `82b453c8f`)

- **The premise holds.**
  - On `SIGTERM`, `ctrl.SetupSignalHandler` cancels the context, and both mains close the listener (`cmd/kv-sni-proxy/main.go` around line 255, `cmd/pg-sni-proxy/main.go` around line 403).
  - `sniproxy.Serve` (`internal/sniproxy/limiter.go`) then returns, `main` returns, and the process exit closes every relayed stream (`sniproxy.CopyBidirectional`, `internal/sniproxy/meter.go`).
  - Both DaemonSets (`config/kv-sni-proxy/daemonset.yaml`, `config/pg-sni-proxy/daemonset.yaml`) set `hostPort` (6379, 5432), no `updateStrategy` and no `terminationGracePeriodSeconds`. On the local cluster that reads as `{"maxSurge":0,"maxUnavailable":1}` with the default 30 s grace.
- **Production does not use the `hostPort`.**
  - The Hetzner edge LB sends `5432 → 31056` and `6379 → 31892`, each with a TCP health check every 15 s, 3 retries (`infra/terraform/main.tf`).
  - Those NodePorts are `bex-pg-sni-proxy-public` and `bex-kv-sni-proxy-public`, both `externalTrafficPolicy: Local` (`config/prod/*-sni-proxy-lb.yaml`).
  - The `hostPort` serves only ADR009's dev "hostNetwork edge". It is also what stops a surge: the scheduler will not place a second pod asking for the same host port on a node, so a new pod would wait for the old one to be gone.
- **The local cluster cannot run either proxy today.** On 2026-11-19, `bex-kv-sni-proxy` and `bex-pg-sni-proxy` showed desired 1, current 0, with `FailedCreate … violates PodSecurity "baseline:latest": hostPort`. w6/m457 hit the same refusal. Dropping the `hostPort` is therefore also what makes t004's local proof possible.
- **Topology.**
  - Production has 3 `bex-platform` nodes (cx33, autoscaler min = max = 3) and runs Cilium.
  - The local cluster runs Calico with kube-proxy, and each DaemonSet targets one node there (desired 1), the single-node case w6/144 names.
  - How an established NodePort flow to a terminating pod behaves must be checked on both. Locally that is t004, and on production it is t006's held client.
- **The deploy wait fits a surge.** `scripts/operator-pins.sh wait` waits for each DaemonSet's pinned image, then `rollout status` with 300 s. A rollout completes when each node's new pod is available and does not wait for terminating pods.
- **Host ports are already a privilege.** `deploy/gitops/base/operator-workload-admission.yaml` refuses a `hostPort` in every operator-authored workload, because it "publishes a container on its node's address, past every Service and NetworkPolicy". The SNI DaemonSets come from Argo, so that rule never covered them.

## Design

- **Surge, not port handover.**
  - Drop both `hostPort`s and roll with `maxSurge: 1`, `maxUnavailable: 0`. Each node's new pod becomes a Ready local endpoint of the `Local` NodePort before the old pod gets `SIGTERM`. The edge LB's health check never sees the node without an endpoint, and the old pod's established flows keep their conntrack entries.
  - **Rejected: `SO_REUSEPORT` handover.** Two pods share a port only in the host's network namespace, which needs `hostNetwork`, a stronger privilege than `hostPort`. Both pods would also have to leave the port undeclared to pass the scheduler's host-port check.
  - **Rejected: draining only while another node serves the edge.** The LB takes up to 45 s (15 s × 3) to drop a node, and a single-node pool has no other node.
- **The drain is opt-in by env.** With the drain variable unset, the proxy closes at once, as today, so `make run` and the existing tests are unchanged.
- **One roll.** The binary change and the template change (no `hostPort`, the drain env, the grace period) ship in one commit. The roll that ships them is already a surge, since the new pods ask for no host port. Each DaemonSet rolls once, and that roll still closes the old pods' streams at once, because the old pods run the old code.
- **The bound.** By default 30 s with `terminationGracePeriodSeconds: 35`, as w6/m512, which is Render's default shutdown delay for its own instances ([deploys](https://render.com/docs/deploys)).
  - With a surge the drain no longer holds the edge, so a longer bound costs less here than on a Deployment: up to the proxies' 1 h idle timeout, as the ssh-gateway's 3,660 s grace for open sessions (w6/m482).
  - t003 weighs that against how long a platform node drain waits for the pod, and two pods of up to 512 Mi each per node.
  - t005 records the choice and its Render basis, and files the other as a follow-up.
- **Routing during the drain is unchanged.** The manager stops with the signal, so a draining pod's routing table is frozen. Open streams never consult it again, and dropping a route has never closed an open stream (ADR021 § 10).
- **Limits during the surge.** Each pod has its own limiter, so for one node's drain the route caps (twice the plan's limit) and the 4,096 global cap count per pod. The datastore's own limit still decides who connects (w6/m457), as it does across nodes today.

## Sequencing constraint

- **Do not ship before w6/m470 t006 has its held-client deploy.** This milestone changes both SNI binaries, so its shipping deploy moves both pins and rolls both DaemonSets. m470's DoD bullet 2 needs a deploy that keeps both pins while `m470-t006-kv-client` and `m470-t006-pg-client` hold their connections.
- Implement and prove it locally meanwhile. Ship once m470 t006 has recorded that deploy, or m470 is in `done/`. If m470's probes are re-established for a later window, coordinate the ship with its record.

## Park record (2026-11-19)

- **Verdict: blocked.** The premise held at `ffd500edd`, and nothing in the work needed a decision. The milestone's own sequencing gate is the blocker. w6/m470 was re-parked in `ffd500edd`: its next pin moves both SNI pins, and its held-client record needs a later pin that keeps both. Every task here changes the SNI binaries or the DaemonSet templates, and either change rolls both DaemonSets. So no part of it can ship early without costing m470 its kept-pin deploy.
- **Ready in stash `loopx/w6/m515` (`a1c172dc0`).** It holds t001–t005: `internal/sniproxy/serve.go` and its tests, both SNI mains and their tests, both DaemonSets and the Service comment, the `gitops-validate.sh` guard, ADR021 § 10, ADR009 and `lego/operator/AGENTS.md`. Checked on that tree:
  - `go test -race` on the three packages, and `make test`;
  - `make lint`;
  - `bash scripts/gitops-validate.sh`;
  - `bash scripts/docs-links-validate.sh`.
- **Proven on the local cluster (t004).** In a binary-change roll of both DaemonSets, external `BLPOP` and `pg_sleep` clients read their results through the old pods after `SIGTERM`. Held idle clients closed at the 30 s bound, and new-connection loops saw no failure.
- **Two findings change the design, recorded in t003.**
  - Each old pod pauses 5 s in `preStop` before `SIGTERM`. Without the pause, a new client routed to the old pod in the moment before its endpoint left kube-proxy's rules was refused. The grace period is therefore 40 s.
  - § Findings' "a rollout … does not wait for terminating pods" does not hold for `kubectl rollout status`, which waits for them: 91.5 s for a 90 s drain, measured. That keeps the bound at 30 s. The 1 h variant is [w6/146](../146.md).
- **Gate:** [w6/m470 t006](../m470/done/t006.md) records a deploy whose write-back logs `kept` for both SNI pins while its probes hold, or m470 is in `done/`. That clears on its own after at least two more production pins; check `git log origin/main --grep "pin platform images"` and m470's board. Then apply the stash and follow t006 § Gate.
- **Seen on the way, not changed here.** `ByteMeter.Add` counts a stream's egress only when it closes. A stream that closes less than one 15 s scrape before its process exits is never billed: today that is every open stream at a roll. The drain bills more of them, not all (w6/146 § Weigh).

## Unblock record (2026-11-19)

- **Affected:** w6/m515 t006 (and, through it, t007–t009). **Source:** verified evidence, the gate's own release condition.
- **The gate holds.** w6/m470 is in `done/` (`6dff53a8f`). Its t006 recorded the held-client deploy: `99bbc4239` pinned `c624a4f217ff`, its write-back logged `kept` for both SNI pins, and an external Key Value client and a Postgres client each stayed on one connection across it. So shipping this milestone no longer spends m470's kept window.
- **The stash applied on `def3eb488`.** No upstream commit touched `internal/sniproxy`, either SNI main or either DaemonSet since the stash's base `ffd500edd`. Drift was in the docs and in `scripts/gitops-validate.sh` (w6/m517, w6/m518 t001–t002, w4/m511), merged by hand:
  - ADR021 § 10 keeps w6/m517's per-store template hold, then this milestone's SNI paragraph, then one "gap that remains" for both. The link to w6/146 points at [w6/m518](../m518/README.md), its promotion, and the bound bullet says w6/m518 t001–t002 already made `operator-pins.sh wait` and `bex-operator`'s Argo health stop waiting on draining SNI pods.
  - t004's reference to w6/m512 t005 now names `.pm/w6/done/m512/done/t005.md`.
- **Rechecked on the merged tree:** `go test -race` on `internal/sniproxy` and both SNI mains, `make test`, `make lint`, `bash scripts/gitops-validate.sh` and `bash scripts/docs-links-validate.sh`.
- **Next (t006):** once production's pin carries the ship commit, record that each DaemonSet rolled once by surge with no `hostPort`, the drain env and the 40 s grace, with a new-connection loop logging no failure. Then hold clients across the first later deploy that moves an SNI pin. w6/m518 t003 onward waits for that record.

## Park record (2026-11-19, after the ship)

- **Verdict: blocked** on production running the ship commit. `495d04b86` carries t001–t005. The newest pin, `d38508f70` (producer `1c63b6fe0a34`), predates it, and t006 reads the deploy that rolls it.
- **Done in this run:** the stash applied and shipped (§ Unblock record), the stash dropped, and t007's `/simplify` ran on the shipped diff with nothing to apply. t008's open point, a `main`-level test for the second signal and the manager-exit path, stays for after t006.
- **Armed for t006 step 1:** `m515-t006-kv-loop` in `bex-canary` opens a new external Key Value connection about every second and logs each failure. No Postgres loop: the workspace's free database was a QA run's. Production's DaemonSet state before the deploy, and the exact next steps, are in [t006](done/t006.md#progress-2026-11-19-2110z-shipped-as-495d04b86-waiting-for-a-pin-that-carries-it).
- **Gate:** pin must carry `495d04b86`. Then t006 steps 1–3. w6/m518 t003 onward waits for t006's record.

## Unblock record (2026-11-19, the pin carries the ship)

- **Affected:** w6/m515 t006, and through it t008 and t009. **Source:** verified evidence, the gate's own release condition.
- **The gate holds.** Production pin `c8c774658` (21:57:46Z) pins `98e053468f52`, which contains `495d04b86` (`git merge-base --is-ancestor`). Its deploy run 44620029262 logged `moved` for both SNI pins (`697fe9c0…` → `1ec4a7c2…`). At 22:16Z both DaemonSets run `1ec4a7c2…` with `maxSurge: 1`, `maxUnavailable: 0`, no `hostPort`, the drain variable at `"30"`, a 5 s `preStop` and a 40 s grace.
- **Next (t006):** step 1's record from that deploy and `m515-t006-kv-loop`'s log. Then step 2's held clients on both proxies across the next deploy that moves an SNI pin, and step 3.

## Park record (2026-11-19, step 2 armed)

- **Verdict: blocked** on the next production deploy that moves an SNI pin. No commit after `98e053468f52` touches a package the SNI binaries link, so the next pin is expected to keep both (t006 § Next).
- **Done in this run:** t006 step 1 ([t006 § Step 1](done/t006.md#step-1-the-shipping-deploy-2026-11-19-pin-c8c774658)). Pin `c8c774658` moved both SNI pins, and each DaemonSet rolled once by surge, each node's new pod Ready before its old pod was deleted. `m515-t006-kv-loop` logged no failed connection. Acceptance criterion 1 is met.
- **Deviation from § Design "One roll":** Argo applied `495d04b86`'s templates at push, at 21:09Z, on the old image, so each DaemonSet rolled twice: by template, then by binary at 22:00Z. Both rolls were surges and neither failed a new connection, but neither drained, since the old pods ran the old binary. An SNI template change rolls at push.
- **Armed for step 2:** held `BLPOP` and `pg_sleep` clients, a `SUBSCRIBE` and an idle `psql` session, and a new-connection loop on each proxy, all in `bex-canary`. They reconnect by themselves, so they need no re-arming before the deploy. The free Postgres probe `dpg-dbvnees3j89c738afv00` is new. [t006 § Step 2](done/t006.md#step-2-armed-2026-11-19-2228z) lists them, and [t006 § Next](done/t006.md#next-the-worker-after-the-next-pin-that-moves-an-sni-pin) gives the check and the cleanup.
- **Gate:** pin must carry `c8c774658`, which clears at the next pin, then the first deploy from there that moves an SNI pin. If a pin keeps both, park again on it. w6/m518 t003 onward waits for t006's record.

## Unblock record (2026-11-20, the next pin moved both SNI pins)

- **Affected:** w6/m515 t006, and through it t008 and t009. **Source:** verified evidence, the gate's own release condition.
- **The gate holds.** Pin `a47204c7e` (04:24:00Z) pins `3e53abbe3bc3`, the first pin after `c8c774658`, which it contains (`git merge-base --is-ancestor`). Its deploy run 44671243497 logged `moved` for both SNI pins at its write-back (04:23:58–59Z): `/kv-sni-proxy` `efb1ca1e…` → `1fc3398e…` and `/pg-sni-proxy` `3c65b030…` → `76153ec2…`, each to `3cfea3ac…`. The SNI input it changed is `lego/types/v1alpha1/keyvalue_types.go` (`58c64788f`, w9/m521). No SNI template changed after `98e053468f52`, so this was each DaemonSet's first roll since step 2 was armed (kv generation 818 → 819, pg 901 → 902).
- **The probes were still running.** At 04:44Z all six `bex.co/probe=m515` pods were `Running` on `hbkdk`, and the roll had happened at 04:25:22–04:26:07Z, so events covered it.
- **Next:** t006 step 2's record from that roll, then step 3's cleanup (both done in this run, [t006 § Step 2](done/t006.md#step-2-the-sni-moving-deploy-2026-11-20-pin-a47204c7e)), then t008 and t009.
