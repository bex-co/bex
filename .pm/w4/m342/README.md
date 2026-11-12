# w4 · m342 — Turning an environment's network isolation on or off takes effect at once, restarts nothing, and never cuts the environment's own private traffic

**Worker:** worker4 **Goal:** toggling "Block cross-environment connections" changes only network policy: the boundary holds or lifts within one reconcile, no service in the environment gets a new instance, and services inside the environment keep reaching each other throughout. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Every App pod carries its environment's stable id, set at creation and on a move only | 45m | — |
| [t002](t002.md) | Isolation is policy-only: the toggle edits NetworkPolicies, never a pod template | 75m | t001 |
| [t003](t003.md) | Datastores and cron pods follow the same identity, and existing pods migrate once | 40m | t002 |
| [t004](t004.md) | Simplify | 15m | t003 |
| [t005](t005.md) | Test coverage | 30m | t003 |
| [t006](t006.md) | Closeout (live replay on production) | 30m | t004, t005 |

## Definition of done

All on production in `bex-canary`, with a project whose isolated environment `qa-iso` holds a Starter private service P (`traefik/whoami:v1.10.1`) and a Free BusyBox prober A, and whose open environment `qa-open` holds a Free BusyBox prober B. Each prober logs `reach-ok` or `reach-fail` for a `wget -T 3 http://<P>:80/` every 5 s (the round 177 fixture below).

1. **Baseline (passes today):** A logs only `reach-ok` and B only `reach-fail`.
2. **Off:** after `PATCH /v1/environments/<qa-iso> {"networkIsolationEnabled":false}`:
   - B logs `reach-ok` within 10 s;
   - A never logs `reach-fail`;
   - `GET /v1/services/<id>/instances` for A and P lists the same instance ids as before the PATCH.
3. **On:** after `PATCH … {"networkIsolationEnabled":true}`:
   - B logs no `reach-ok` later than 10 s after the PATCH;
   - A never logs `reach-fail`;
   - A and P keep their instance ids.
4. **No deploy:** neither toggle creates a deploy row, as today.
5. **Settings:** the environment settings switch reads the saved state after a reload. That held in w4/m185.
6. Fixtures deleted afterwards.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 177, 2026-11-12 07:53–07:58 UTC (`muse.env`, workspace `bex-canary`; pin `3d66946c9e16`). The finding is below.
- **Governing docs:** [ADR043](../../../docs/ADR043-tenant-namespace-isolation.md) (tenant namespaces, environment isolation) and w4/m185 (done 2026-10-09), which made a label change converge without a manual deploy. Its live replay measured +3 s and +9 s on BusyBox targets and recorded "no deploy, restart", but it did not compare instance ids.
- **Goal linkage:** Render parity for private network isolation, a security boundary: "Block cross-environment connections" must hold the moment it says it does, and turning it on must not take the environment's own services down.
- **Expected outcome:** the toggle is a policy edit. Nothing restarts, and enforcement is immediate in both directions.
- **Why now:** major. Each toggle currently restarts every service in the environment without recording an event, leaves the "isolated" service reachable from outside until its old pods are gone, and cuts private traffic inside the environment while its services roll at different times.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard surface changes. The API and the switch already say the right thing; only the operator's enforcement does not.

## Finding (round 177, 2026-11-12)

### Fixture

- Project `qa-20261112-r177-proj` (`prj-dbqn0n2ibtfs738vkot0`), created by `POST /v1/projects` with `environments: [{name:"qa-iso", networkIsolationEnabled:true}, {name:"qa-open"}]`. That gave `qa-iso` = `evm-dbqn0n2ibtfs738vkotg` (`networkIsolationEnabled: true`) and `qa-open` = `evm-dbqn0n2ibtfs738vkou0`.
- In `qa-iso`:
  - P `qa-20261112-r177-priv` (`srv-dbqn0qaibtfs738vkp00`), a Starter private service running `traefik/whoami:v1.10.1` on port 80;
  - A `qa-20261112-r177-in` (`srv-dbqn0qa6krac73d835v0`), a Free web service on `busybox:1.36`.
- In `qa-open`: B `qa-20261112-r177-out` (`srv-dbqn0qiibtfs738vkp20`), the same BusyBox prober.
- Prober command: `sh -c 'while true; do if wget -q -T 3 -O /dev/null http://qa-20261112-r177-priv:80/; then echo <TAG>-reach-ok; else echo <TAG>-reach-fail; fi; sleep 5; done & … httpd -f -p ${PORT:-10000} …'`.
- Everything was deleted afterwards: the three services (DELETE 204, then GET 404), both environments (204) and the project (204, then GET 404).
- **Fixture caveat:** the probers run `sh` as PID 1, which ignores SIGTERM, so their old pods linger until the grace period kills them. That stretches the old-pod lines below but not the new pods' gaps.

### Probe

Results come from `GET /v1/logs?resource=<id>&text=reach`, split by each line's `instance` label; times are UTC.

| Step | B (outside) → P | A (inside) → P | Instances |
| --- | --- | --- | --- |
| Baseline, 07:53:32–07:55:17 | `reach-fail` every 8 s | `reach-ok` every 5 s | A `…2f5ss`, P one pod, B `…r7mqf` |
| `PATCH {"networkIsolationEnabled":false}` at 07:55:18 (200) | fail until 07:55:33, **ok from 07:55:38** (+20 s) | new pod `…jdvri`: **fail at 07:55:25 and 07:55:33**, then ok; old pod `…2f5ss`: **fail 07:55:25–07:56:45** | **A and P get new pods** with no deploy row and no event |
| `PATCH {"networkIsolationEnabled":true}` at 07:56:04 (200) | **still ok until 07:56:48** (+44 s), fail from 07:56:56 | new pod `…fcf60` (created 07:56:20): **fail 07:56:27–07:56:43**; pod `…jdvri`: fail from 07:56:56 | A `…fcf60` at 07:56:20, **P new pod at 07:56:37** |

- B never changed instance (`…r7mqf` throughout).
- A's and P's Events list only their original `deploy_started` and `deploy_ended`. No toggle recorded anything.
- Each service still has exactly one deploy row.

### Root cause: the boundary is a pod label, and a pod label changes only by replacing the pod

- **The blanket allow excludes labelled pods.** The workspace's `allow-same-namespace` NetworkPolicy selects, and admits traffic from and to, only pods **without** `app.bex.co/network-isolation` (`sameNamespaceNetworkPolicy`, `lego/backend/internal/store/namespaces.go:728-745`, `DoesNotExist`).
- **The per-App policy admits only same-label peers.** An isolated App gets its own policy selecting `app.bex.co/app=<name>`, with ingress and egress only to peers carrying `network-isolation=<env>` or a datastore's `environment-id=<env>` (`reconcileProtectedAppNetworkPolicy`, `lego/operator/internal/controller/environment_network_policy.go:83-123`).
- **The toggle reaches pods only through a rollout.**
  1. It patches the label on each member App CR (`applyACL` → `applyAppEnvironmentLabels`, `lego/backend/internal/environments/service.go:1523-1541`, `:1842-1861`).
  2. The operator copies it onto the Deployment's pod template (`syncPolicyLabels`, `lego/operator/internal/controller/app_controller.go:7578-7583`, from w4/m185 t002's `convergeServingRoute` sync).
  3. A template change is a new ReplicaSet, so every service in the environment rolls.
- **So, turning it on:** P's per-App policy exists from P's reconcile, but P's old pods still lack the label. The blanket allow still selects them, and NetworkPolicy allows are a union, so B keeps reaching P until the last old pod is gone (+44 s here). In general that is a rollout plus up to the service's shutdown delay, which goes up to 300 s.
- **So, turning it off:** P's per-App policy is deleted while P's old pod still carries the label, so the blanket allow does not select it and B and A's new pod are refused until P's new pod is ready.
- **Same-environment gap, both ways:** an old-label pod and a new-label pod of two services in the same environment cannot reach each other. The services roll at different times (A at +16 s, P at +33 s here), so private traffic inside the environment breaks for seconds to a minute.
- **Datastores:** Key Value's client selector is the App pod's isolation label too (`keyvalue_internal_network.go:53-59`), and Postgres peers use `reconcileEnvironmentPeerPolicy` (`environment_network_policy.go:41-80`). An App reaching its own environment's datastores would hit the same gap. **Unverified live** (no datastore in the fixture).
- **Not traced:** why P's reconcile came 17 s after A's.

### Unverified

- **Render's behavior.** Whether Render restarts services when isolation is toggled, and how fast it enforces, was not captured. The target here follows the setting's own promise, "Block cross-environment connections", and DoD 1–3.
- **Datastores in an isolated environment** across a toggle.
- **Moving a service into or out of an isolated environment.** It is the same label path (w4/m185's move cases), and a move legitimately changes the environment id, so t001 allows one rollout there. It was not probed this round.

### Dedupe

- w4/m185 (done) introduced the label sync and measured enforcement, but not restarts or the same-environment gap.
- w6/m19 (done) introduced the protected-environment policies.
- w8/m136 introduced the datastore isolation.
- No open item covers this: `grep -rl -i isolation .pm --include='*.md'` finds no open item about restarts or rollouts.
- Not deploy lag: the last commit to the isolation path is `f89e1617a` (w4/m185), and nothing after `3d66946c9` touches `environment_network_policy.go`, `namespaces.go` or `syncPolicyLabels`.
