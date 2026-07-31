# w1 · m58 — bex-api multi-replica correctness — durable shared state for replica-local buckets

**Worker:** worker1 **Goal:** make bex-api's replica-local in-memory state correct under its two-replica HA deployment so outbound webhooks are delivered exactly once and failure notices aren't re-sent. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                            | est  | depends_on          |
| ---- | ---------------------------------------------------------------------------------------------------------------- | ---- | ------------------- |
| t001 | Audit the three replica-local buckets; write the multi-replica contract                                          | 60m  | —                   |
| t002 | Outbound-webhook delivery: Postgres SKIP LOCKED claim so two replicas never double-deliver                       | 90m  | t001                |
| t003 | Outbound-webhook failure-notice suppression: persist the notified-at marker in the control plane                 | 60m  | t001                |
| t004 | Deploy-hook (git-push) rate limiter: pick shared-clamped vs. documented-per-replica and implement                | 60m  | t001                |
| t005 | Live verify on dev-1 with two bex-api replicas: duplicate-delivery + double-email negative tests                 | 60m  | t002, t003, t004    |
| t006 | Simplify (`/simplify` over the diff)                                                                             | 30m  | t005                |
| t007 | Test coverage                                                                                                    | 60m  | t006                |
| t008 | Closeout                                                                                                         | 15m  | t007                |

## Definition of done

With two bex-api replicas serving concurrently, a single triggered event delivers exactly one outbound webhook (not two); a pod restart mid-delivery does not re-deliver or re-notify; the deploy-hook rate limit is the documented value regardless of replica count. Unit/integration tests assert the `SKIP LOCKED` claim and the persisted notified-at marker; a dev-1 two-replica run shows zero duplicates under a forced restart.

## Source + Goal linkage

- **Source:** `.pm/FUTURE-MAYBE.md` "Durable/shared state for bex-api's replica-local in-memory buckets" (w1 brainstorm round 12, 2026-07-15), whose trigger — "bex-api scales past one replica" — **fired** when w1/m52 (2026-07-18) rolled bex-api to two replicas for zero-downtime deploys (reconfirmed 2026-07-30 in the w3/m35 ship's `roll bex onto the freshly-pushed image`).
- **Goal linkage:** platform reliability + the control-plane source-of-truth ([docs/ADR003-control-plane.md](../../docs/ADR003-control-plane.md)). bex-api is HA; its replica-local state must not corrupt tenant-facing event delivery.
- **Expected outcome:** zero duplicate outbound-webhook deliveries and zero duplicate failure-notice emails under two replicas and across restarts; the deploy-hook rate limit behaves identically at one or two replicas.
- **Why now:** the trigger fired ~2 weeks ago and the bug is latent-but-live: `lego/backend/internal/webhooks/worker.go:55` records that two bex-api replicas double-deliver every event, and `worker.go:160`'s `emittedAt` map re-emails on restart / double-emails on the second replica. This is a tenant-facing correctness defect, not a hypothetical.
- **Render parity:** omitted — this is an internal correctness/mechanism change with no REST/GraphQL/MCP/UI surface change. The outbound-webhook contract is unchanged (single-replica already delivered once; this preserves that under HA).
