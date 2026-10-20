# w5 · m142 — A node failure is reported as `server_hardware_failure`, not as an app crash

**Worker:** worker5 **Goal:** when a service instance is lost because its machine failed or was removed, the service's events record Render's `server_hardware_failure` instead of `server_failed`, on REST, GraphQL and MCP events, webhooks and the dashboard. An app crash still records `server_failed`. **Status:** todo

## Tasks (in order)

| id   | title                                                                         | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Operator records when an instance was lost to its node, not its container     | 60m | —          |
| t002 | The store records `server_hardware_failure` for node losses                   | 60m | t001       |
| t003 | Vocabulary, webhooks, dashboard catalog and docs; dev-5 node-removal walk     | 45m | t002       |
| t004 | Render parity                                                                 | 20m | t003       |
| t005 | Simplify                                                                      | 20m | t004       |
| t006 | Test coverage                                                                 | 45m | t004, t005 |
| t007 | Closeout                                                                      | 10m | t006       |

Total: **7 tasks, about 4h 20m**.

## Definition of done

- On dev-5, removing the worker node under a running service (`bash scripts/mock-cluster.sh scale N-1`, or deleting the node) records exactly one `server_hardware_failure` for that service, and its recovery records `server_available` as today.
- A container crash (non-zero exit, OOM) on a healthy node still records `server_failed` and never `server_hardware_failure`.
- The event appears on REST, GraphQL and MCP service events, is delivered to a subscribed webhook, and is labelled in the dashboard (en/zh).
- The vocabulary fixture lists `server_hardware_failure` as supported, and ADR018 records the mapping.

## Source + Goal linkage

- **Source:** fourth `/pm-brainstorm for w5` (2026-10-20), item 5, chosen by the user the same day. Render's vocabulary includes `server_hardware_failure`; bex's fixture (`docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-08-17.json`) lists it as "no source yet". Today an instance lost with its node falls through to `server_failed` (`lego/backend/internal/store/event_facts.go:45`, availability edges around `:420-500`), which reads as the tenant's own crash. Kubernetes has the signal — the node goes NotReady and the evicted pod carries a `DisruptionTarget` condition — but the operator never reads nodes or projects it. No board owner.
- **Goal linkage:** GOAL.md #6 (add and remove physical machines); ADR008's machine-readable state; ADR018 Render parity (service events).
- **Expected outcome:** a tenant can tell "the platform lost a machine" from "my app crashed", in the feed and in webhooks.
- **Why now:** the cluster autoscaler adds and removes nodes routinely (`deploy/gitops/base/autoscaler.yaml`), so each scale-down or node fault currently shows up as an app failure. Lower priority than w5's other queued work.
- **Render parity included:** this adds an event type on REST, GraphQL and MCP events, webhooks and the dashboard catalog.
