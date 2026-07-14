# w3 · m13 — Fix log-shipper N× duplication

**Worker:** worker3 **Goal:** node-scope the Alloy DaemonSet's pod discovery so each log line ships once, not N times (N = node count) **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Inject `NODE_NAME` via downward API into the Alloy DaemonSet config                                      | 30m | —          |
| t002 | Scope `discovery.kubernetes "pods"` to the local node (`selectors { field = "spec.nodeName=..." }` or a `keep` relabel on `__meta_kubernetes_pod_node_name`) | 1h  | t001       |
| t003 | Live-verify on a real multi-node cluster that logs are neither lost nor still duplicated — pair with the existing `w3/m5/t005` + `w3/m8/t005` live-run tasks | 1h  | t002       |
| t004 | Regression signal: a metric/test that would catch a future re-introduction of unscoped discovery         | 45m | t003       |
| t005 | Simplify                                                                                                 | 30m | t004       |
| t006 | Test coverage                                                                                             | 1h  | t004       |
| t007 | Closeout                                                                                                  | 15m | t006       |

## Definition of done

Each log line is shipped exactly once (not N×) to Loki, verified live on a real multi-node cluster with zero log loss.

## Source + Goal linkage

- **Source:** `.pm/w3/004.md` (filed during `w3/m8`'s `/simplify` pass 2026-07-12, deliberately deferred pending live-cluster verification — the failure mode of a bad node-scope match is silently losing all logs). Promoted via `/pm-brainstorm more` 2026-07-13 (fourth pass).
- **Goal linkage:** infra efficiency/correctness on the durable-logs pipeline `w3/m5` shipped.
- **Expected outcome:** N× shipper CPU, push bandwidth, and apiserver log-follow streams collapse to 1×.
- **Why now:** the risk that blocked it (need a live cluster to verify safely) is the same standing condition as `w3/m5`'s own blocked live-acceptance task — worth landing together once a cluster is available. **Render parity: omitted** — pure infra/operational fix, no REST/GraphQL/MCP/UI surface change.
