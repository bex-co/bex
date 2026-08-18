# w4 · m83 — Webhook delivery fairness and bounded per-workspace backlog

**Worker:** worker4 **Goal:** keep outbound webhook delivery a shared, bounded service: one noisy or broken workspace cannot grow the non-terminal queue without limit or monopolize every worker claim, while admitted deliveries preserve the existing durable retry, dedupe, and history guarantees. **Status:** done

## Tasks (in order)

| id   | title                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------ | --- | ---------- |
| t001 | Bound non-terminal delivery admission per workspace — **DONE**                | 60m | —          |
| t002 | Claim due deliveries fairly across workspaces and replicas — **DONE**         | 75m | t001       |
| t003 | Add bounded overflow evidence, low-cardinality metrics, and alert — **DONE**  | 45m | t001, t002 |
| t004 | Render parity — **DONE**                                                      | 30m | t003       |
| t005 | Simplify — **DONE**                                                           | 20m | t004       |
| t006 | Test coverage — **DONE**                                                      | 60m | t004       |
| t007 | Closeout — **DONE**                                                           | 10m | t006       |

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

## Closeout

- **Shipped behavior:** `EnqueueWebhookDeliveries` now serializes bounded
  admission per workspace and commits the aggregate result with the source
  watermark. The default open-notification ceiling is 10,000;
  `BEX_MAX_WEBHOOK_DELIVERIES_PER_WORKSPACE=0` uses the uncapped fast path. Due
  attempts rank by workspace round before `FOR UPDATE SKIP LOCKED`, preserving
  deterministic per-workspace order and disjoint replica leases. Migration
  `0085_webhook_queue_indexes` supports the open-count and pending-attempt joins.
- **Evidence and operations:** committed admission outcomes feed fixed-label
  admitted/capped/deduplicated metrics, a capped-batch histogram, and one
  aggregate log per dispatch pass. `WebhookDeliveryAdmissionPressure` warns on
  sustained pressure with fire-and-clear promtool coverage; ADR006 and ADR010
  document the capacity response.
- **Render parity:** Render's live official webhook guide, OpenAPI history
  operation, and Recent deliveries walkthrough were rechecked on 2026-08-17.
  They publish sent-attempt history and retry behavior, but no queue-depth or
  scheduler contract. Bounded/fair scheduling is recorded as an internal bex
  extension; capped work creates no false attempt on REST, GraphQL, MCP, or the
  dashboard and never changes the source mutation's response.
- **Simplify:** the required three-way review found no missed reuse. It removed
  the unbounded store-method bypass, replaced stringly worker metric calls with
  one typed aggregate observer, simplified concurrent test collection, added
  the cap-disabled fast path, and added supporting partial indexes. Candidate
  truncation before `SKIP LOCKED` was rejected because it would let one replica
  temporarily hide deeper due work from another.
- **Verification:** `cd lego/backend && go test ./...`; full
  `BEX_TEST_DB_URI` store suite against throwaway Postgres 17; targeted
  real-Postgres `go test -race` for bounded concurrent enqueue and two fair
  claimers; `make lint-backend` (`0 issues`); Prometheus 3.8 `promtool check
  rules` (35 rules) and `promtool test rules` (`SUCCESS`); `git diff --check`.
