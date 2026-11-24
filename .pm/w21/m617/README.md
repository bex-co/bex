# w21 · m617 — A multi-instance service's instances run on different nodes, as on Render

**Worker:** worker21 **Goal:** a service scaled or autoscaled to N instances runs them on N different nodes when the pool has N, so one node's failure never takes every instance down. **Status:** in progress (t001–t003, t005, t006 done; t004 has its ADR018 row and waits for production to carry t002)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Measure placement on the mock cluster and record the spread decision in ADR004 — **DONE** | 45m | — |
| [t002](done/t002.md) | The operator projects a hostname topology spread for multi-instance services, carried by rollouts the service makes anyway — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Mock cluster: a full node makes the second instance add a node, and drain-guard's evacuation still lands — **DONE** | 1h | t002 |
| [t004](t004.md) | ADR018 row "Multiple instances run on different nodes" and a production check with a disposable two-instance service | 45m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | t004 |
| [t007](t007.md) | Closeout | 10m | t005, t006 |

## Definition of done

1. On the mock cluster with at least two tenant workers, a service scaled to N (N at most the node count) runs on N distinct nodes; with more instances than nodes, no node holds two more than another.
2. When the only node with room already holds an instance, the next instance makes the autoscaler add a node instead of landing beside it.
3. No existing single-instance, free or disk-attached service is rolled by the operator upgrade.
4. On production, a disposable two-instance service runs on two nodes.
5. ADR004 and ADR018 record the rule and its evidence.

## Evidence (2026-11-24)

1. Holds on the mock cluster: two, three and five instances over four usable nodes, three runs each, ran one per node and 2-1-1-1 ([t001](done/t001.md), [ADR004 § Multiple instances run on different nodes](../../../docs/ADR004-app-deployment.md#multiple-instances-run-on-different-nodes-w21m617)).
2. Holds on the mock cluster: the second instance waited, the autoscaler added a node within 46 s and the instance ran there at 76 s; before the change it landed beside the first ([t003](done/t003.md)).
3. Holds by construction and test: the spread is adopted only by a roll the service makes anyway; the envtest stores a pre-change Deployment of each kind (two instances, one, Free, disk) and the upgraded projection writes none of them ([t002](done/t002.md), [t006](done/t006.md)).
4. Pending: production must carry t002's commit first ([t004](t004.md)).
5. ADR004 has the rule and the mock evidence; the ADR018 row is in, ◐ until the production reading.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w21` (2026-11-23), filed by `/pmx them all to w21`, item 2. Render's [uptime-best-practices](https://render.com/docs/uptime-best-practices) page was cited nowhere in the repository.
- **Render reference:** uptime-best-practices, read 2026-11-23: scaling to multiple instances means Render "runs those instances on different nodes", and affected services are moved to new instances automatically.
- **bex today:** the production scheduler scores with `NodeResourcesFit` `MostAllocated` (`infra/clusterapi/overlays/hetzner-caph/cluster.yaml:1344-1360`), so it prefers the fullest node. `projectDeployment` (`lego/operator/internal/controller/deployment_projection.go:935`) sets no `topologySpreadConstraints` and no pod anti-affinity; the scheduler's built-in default spread is a soft score (maxSkew 3, `ScheduleAnyway`) that competes with the packing score. drain-guard ([ADR004 § Node rolls](../../../docs/ADR004-app-deployment.md#node-rolls-hand-single-instance-services-over-before-the-drain-w1m186)) covers planned drains of a node holding every Ready instance, not an unplanned node loss.
- **Production reading (2026-11-23, read-only):** 30 tenant Deployments, none with two or more replicas. The gap is latent; no customer depends on it yet.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); manual scaling and autoscaling (Pro and higher) are sold features whose point is availability.
- **Expected outcome:** a customer who scales for availability gets it: one node failure leaves the other instances serving.
- **Why now:** with no multi-instance tenant service on production, the rule lands with no tenant rollout. After the first one exists (signup is open since 2026-11-21), the change would roll a live service.
- **Decision state.** `DoNotSchedule` with `maxSkew: 1` on `kubernetes.io/hostname` and `matchLabelKeys: [pod-template-hash]` is this proposal's recommendation, resting on ADR008 Render parity first (Render states a placement, not a preference). It can make the autoscaler add a node that bin-packing would have avoided; that cost is the price of the guarantee. `ScheduleAnyway` would be a divergence that needs the user's ADR018 sign-off. The constraint is carried only by rollouts a service makes anyway, so the upgrade itself rolls nothing; t001 records how a scale from 1 to 2 of a service whose template lacks it is handled.
- **Render parity:** omitted. The placement is operator-internal: no REST, GraphQL, MCP or dashboard field changes, and Render shows none.
