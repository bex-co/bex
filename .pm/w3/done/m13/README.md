# w3 · m13 — Fix log-shipper N× duplication

**Worker:** worker3 **Goal:** node-scope the Alloy DaemonSet's pod discovery so each log line ships once, not N times (N = node count) **Status:** done

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on | status |
| ---- | -------------------------------------------------------------------------------------------------------- | --- | ---------- | --- |
| t001 | Inject `NODE_NAME` via downward API into the Alloy DaemonSet config                                      | 30m | —          | — **DONE** |
| t002 | Scope `discovery.kubernetes "pods"` to the local node (`selectors { field = "spec.nodeName=..." }` or a `keep` relabel on `__meta_kubernetes_pod_node_name`) | 1h  | t001       | — **DONE** |
| t003 | Live-verify on a real multi-node cluster that logs are neither lost nor still duplicated — pair with the existing `w3/m5/t005` + `w3/m8/t005` live-run tasks | 1h  | t002       | — **DONE** |
| t004 | Regression signal: a metric/test that would catch a future re-introduction of unscoped discovery         | 45m | t003       | — **DONE** |
| t005 | Simplify                                                                                                 | 30m | t004       | — **DONE** |
| t006 | Test coverage                                                                                             | 1h  | t004       | — **DONE** |
| t007 | Closeout                                                                                                  | 15m | t006       | — **DONE** |

## Definition of done

Each log line is shipped exactly once (not N×) to Loki, verified live on a real multi-node cluster with zero log loss.

**Met 2026-07-15.** Node-scoped `discovery.kubernetes "pods"` via a server-side `spec.nodeName` field selector, reusing the alloy chart's own `K8S_NODE_NAME` downward-API var. Live-verified on prod (Hetzner, 7 nodes): all 7 replicas confirmed via Alloy's component debug API to discover targets from exactly their own node, and a 20-line marked-log test through Loki confirmed zero loss + zero duplication. Found and fixed an unrelated pre-existing incident en route — prod's Argo Application had been stuck in `ComparisonError` since `w3/m8` (Helm's `tpl` choking on the River `stage.template` blocks' own Go-template syntax), silently freezing the live ConfigMap at a pre-`w3/m8` version; fixed with the standard Helm brace-escape, bundled into the same diff. The diff (`deploy/gitops/base/log-shipper.yaml` + `scripts/gitops-validate.sh`) is complete and live-verified but not yet committed — commits happen only via `/ship` per `CLAUDE.md`. See task resolutions for the full verification chain.

## Source + Goal linkage

- **Source:** `.pm/w3/004.md` (filed during `w3/m8`'s `/simplify` pass 2026-07-12, deliberately deferred pending live-cluster verification — the failure mode of a bad node-scope match is silently losing all logs). Promoted via `/pm-brainstorm more` 2026-07-13 (fourth pass).
- **Goal linkage:** infra efficiency/correctness on the durable-logs pipeline `w3/m5` shipped.
- **Expected outcome:** N× shipper CPU, push bandwidth, and apiserver log-follow streams collapse to 1×.
- **Why now:** the risk that blocked it (need a live cluster to verify safely) is the same standing condition as `w3/m5`'s own blocked live-acceptance task — worth landing together once a cluster is available. **Render parity: omitted** — pure infra/operational fix, no REST/GraphQL/MCP/UI surface change.
