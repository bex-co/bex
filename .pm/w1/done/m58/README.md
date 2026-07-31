# w1 · m58 — bex-api multi-replica correctness — durable shared state for replica-local buckets

**Worker:** worker1 **Goal:** make bex-api's replica-local in-memory state correct under its two-replica HA deployment so outbound webhooks are delivered exactly once and failure notices aren't re-sent. **Status:** done (2026-07-30) — SKIP LOCKED delivery claim + `(endpoint_id,event_id)` unique index + persisted `notified_at`; deploy-hook limiter documented per-replica; proven by integration + two-worker e2e tests (see Closeout)

## Tasks (in order)

| id   | title                                                                                                            | est  | depends_on          | status     |
| ---- | ---------------------------------------------------------------------------------------------------------------- | ---- | ------------------- | ---------- |
| t001 | Audit the three replica-local buckets; write the multi-replica contract                                          | 60m  | —                   | — **DONE** |
| t002 | Outbound-webhook delivery: Postgres SKIP LOCKED claim so two replicas never double-deliver                       | 90m  | t001                | — **DONE** |
| t003 | Outbound-webhook failure-notice suppression: persist the notified-at marker in the control plane                 | 60m  | t001                | — **DONE** |
| t004 | Deploy-hook (git-push) rate limiter: pick shared-clamped vs. documented-per-replica and implement                | 60m  | t001                | — **DONE** |
| t005 | Live verify on dev-1 with two bex-api replicas: duplicate-delivery + double-email negative tests                 | 60m  | t002, t003, t004    | — **DONE** |
| t006 | Simplify (`/simplify` over the diff)                                                                             | 30m  | t005                | — **DONE** |
| t007 | Test coverage                                                                                                    | 60m  | t006                | — **DONE** |
| t008 | Closeout                                                                                                         | 15m  | t007                | — **DONE** |

## Definition of done

With two bex-api replicas serving concurrently, a single triggered event delivers exactly one outbound webhook (not two); a pod restart mid-delivery does not re-deliver or re-notify; the deploy-hook rate limit is the documented value regardless of replica count. Unit/integration tests assert the `SKIP LOCKED` claim and the persisted notified-at marker; a dev-1 two-replica run shows zero duplicates under a forced restart.

## Closeout (2026-07-30)

The two replica-local **correctness** buckets became control-plane-durable; the one **rate** bucket is documented per-replica.

- ✅ **Exactly-once delivery** under two replicas: `EnqueueWebhookDeliveries` dedupes dispatch on a new `(endpoint_id, event_id)` unique index (`ON CONFLICT DO NOTHING`), and `ClaimDueWebhookDeliveries` leases each due row with `FOR UPDATE … SKIP LOCKED` (bumping `next_attempt_at` to `now + 4×requestTimeout`) so two send passes take disjoint batches. Migration `0055` collapses any pre-existing duplicates first.
- ✅ **No re-notify on restart / second replica**: the in-memory `emailedAt` map is replaced by a persisted `notified_at` marker + an atomic `ClaimWebhookFailureNotice` CAS, cleared on re-enable.
- ✅ **Deploy-hook limit**: documented **per-replica** (credential-gated + newest-wins idempotent ⇒ a ≤2× ceiling is a bounded damper, consistent with the other two replica-local limiters); stale "one replica" comment fixed.
- ✅ **Tests (real Postgres, CI-run)**: SKIP LOCKED disjoint claim (4 concurrent claimers, each row once), lease re-visibility, dispatch dedup, `notified_at` CAS, a **two-worker end-to-end** run (30 events, each delivered exactly once), and `TestDeployHookRateLimiterIsPerReplica`. Full backend suite green with the DB; `golangci-lint` 0 issues.

No REST/GraphQL/MCP/UI surface change (the outbound-webhook contract is unchanged — single-replica already delivered once; this preserves that under HA).

## Source + Goal linkage

- **Source:** `.pm/FUTURE-MAYBE.md` "Durable/shared state for bex-api's replica-local in-memory buckets" (w1 brainstorm round 12, 2026-07-15), whose trigger — "bex-api scales past one replica" — **fired** when w1/m52 (2026-07-18) rolled bex-api to two replicas for zero-downtime deploys (reconfirmed 2026-07-30 in the w3/m35 ship's `roll bex onto the freshly-pushed image`).
- **Goal linkage:** platform reliability + the control-plane source-of-truth ([docs/ADR003-control-plane.md](../../docs/ADR003-control-plane.md)). bex-api is HA; its replica-local state must not corrupt tenant-facing event delivery.
- **Expected outcome:** zero duplicate outbound-webhook deliveries and zero duplicate failure-notice emails under two replicas and across restarts; the deploy-hook rate limit behaves identically at one or two replicas.
- **Why now:** the trigger fired ~2 weeks ago and the bug is latent-but-live: `lego/backend/internal/webhooks/worker.go:55` records that two bex-api replicas double-deliver every event, and `worker.go:160`'s `emittedAt` map re-emails on restart / double-emails on the second replica. This is a tenant-facing correctness defect, not a hypothetical.
- **Render parity:** omitted — this is an internal correctness/mechanism change with no REST/GraphQL/MCP/UI surface change. The outbound-webhook contract is unchanged (single-replica already delivered once; this preserves that under HA).
