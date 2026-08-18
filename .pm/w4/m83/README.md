# w4 · m83 — Webhook delivery fairness and bounded per-workspace backlog

**Worker:** worker4 **Goal:** keep outbound webhook delivery a shared, bounded service: one noisy or broken workspace cannot grow the non-terminal queue without limit or monopolize every worker claim, while admitted deliveries preserve the existing durable retry, dedupe, and history guarantees. **Status:** todo

## Tasks (in order)

| id   | title                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------ | --- | ---------- |
| t001 | Bound non-terminal delivery admission per workspace                | 60m | —          |
| t002 | Claim due deliveries fairly across workspaces and replicas         | 75m | t001       |
| t003 | Add bounded overflow evidence, low-cardinality metrics, and alert  | 45m | t001, t002 |
| t004 | Render parity                                                      | 30m | t003       |
| t005 | Simplify                                                           | 20m | t004       |
| t006 | Test coverage                                                      | 60m | t004       |
| t007 | Closeout                                                           | 10m | t006       |

## Definition of done

- A configured per-workspace ceiling bounds pending/retrying webhook deliveries at the persistence boundary under concurrent enqueue workers; `0` explicitly disables the ceiling.
- Crossing the ceiling never rolls back the service/deploy/resource mutation that produced the event. The event watermark advances honestly, and overflow leaves bounded, deduplicated operator evidence instead of another unbounded row stream.
- A saturated workspace cannot keep a quiet workspace's due delivery out of the next eligible claim batch. Multiple bex-api replicas still claim disjoint rows with `FOR UPDATE SKIP LOCKED` semantics.
- Admitted rows keep the existing stable `webhook-id`, lease, backoff, endpoint-disable, retention, and exactly-eight-attempt contract.
- Real-Postgres tests cover the cap boundary, concurrent enqueuers, a noisy-plus-quiet fairness case, and two concurrent claimers without duplicate delivery.
- Metrics and alerts diagnose sustained overflow/fairness pressure without workspace ids, endpoint ids, hostnames, or other unbounded labels.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w4` on 2026-08-17, materializing `.pm/w1/048.md` finding #3's deferred remainder. Builds on the delivery-history and retry work in `w1/m58`, `w1/m67`, and `w2/m70`.
- **Goal linkage:** `.pm/GOAL.md` goals 2, 5, and 7 — safe multi-tenant operation, honest observable behavior, and a dependable Render-compatible control plane; ADR008's multi-tenant PaaS product boundary.
- **Expected outcome:** webhook bursts become bounded per workspace and scheduler capacity remains available to unrelated tenants, without turning webhook pressure into failed deploys or duplicated external side effects.
- **Why now:** endpoint count, terminal retention, no-op suppression, lease sizing, and pending-row age-out already shipped. The remaining amplification and starvation mechanisms are now a compact, self-contained persistence/worker milestone.
- **Render parity:** included because webhook delivery and delivery history are user-facing Render-compatible behavior.
