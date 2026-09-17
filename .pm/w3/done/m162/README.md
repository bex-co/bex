# w3 · m162 — Billing webhook, account-deletion, and analytics-pipeline correctness

**Worker:** worker3 **Goal:** Close every correctness defect found in the 2026-09-16 "why is web services stuck at 7" investigation, so a bound payment method is never silently lost, a deleted account leaves nothing behind in Stripe, and the analytics pipeline never labels a failed collection as authoritative. **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on            |
| ---- | --------------------------------------------------------------------------- | --- | --------------------- |
| t001 | Stop the live 503 retry loop: ignore billing events for deleted workspaces — **DONE** | 45m | —                     |
| t002 | Finish account deletion in Stripe: detach payment methods, anonymize Customer — **DONE** | 1h  | —                     |
| t003 | Make the three silent webhook 400s loud; stop hard-rejecting on API version — **DONE** | 45m | —                     |
| t004 | Alert on webhook liveness using the existing last-success metric — **DONE**             | 30m | w3/m162/t003          |
| t005 | Guard against Stripe endpoint ↔ code drift (enabled_events + api_version) — **DONE**    | 45m | w3/m162/t003          |
| t006 | Disambiguate "opened checkout" from "bound a card" in the data model — **DONE**         | 45m | —                     |
| t007 | Fix verify-tenant-isolation.sh resource leak and clear the leaked App CRs — **DONE** | 30m | —                     |
| t008 | Populate the product-analytics audience dimension and guard it — **DONE**               | 30m | —                     |
| t009 | Never mark a failed services inventory collection as complete — **DONE**                | 30m | —                     |
| t010 | `/simplify` over the milestone's changes — **DONE** | 30m | t001–t009             |
| t011 | Test coverage for the shipped behavior — **DONE** | 1h  | t010                  |
| t012 | Closeout — **DONE** | 15m | w3/m162/t011          |

## Definition of done

- Deleting a workspace no longer produces a webhook that retries forever: delivering `customer.subscription.deleted` for an already-deleted workspace returns 204, and `bex_api_http_requests_total{route="POST /v1/webhooks/stripe",status="503"}` stops increasing. The currently stuck event `evt_1UGSEjEqsEqs2tLVTHfQYNzv` converges.
- After a workspace is deleted, its Stripe Customer has zero attached payment methods, no `default_payment_method`, no `email`/`name`, and carries `bex_deleted_at`; the Customer object itself still exists. The two known orphans (`cus_VCV5qtrtlYh0N0`, `cus_VGzT7XpXc4z57t`) are remediated.
- Each of the three webhook rejection branches writes a log line naming the event id and the offending value; an API-version mismatch no longer rejects the delivery.
- An alert fires when no Stripe webhook has succeeded for longer than the configured window.
- A check fails when the live Stripe endpoint's `enabled_events`/`api_version` disagree with the code's whitelist.
- `billing_provider_mappings` distinguishes a started checkout from a bound card; no query can mistake `subscription_id` for proof of binding.
- `scripts/verify-tenant-isolation.sh` cleans up on every exit path, and `verify-iso-a`/`verify-iso-b`/`verify-iso-eg` plus their two empty namespaces are gone from production.
- Every workspace in `product_analytics_audiences` carries a non-`unclassified` audience, and a test fails if a known QA workspace is missing one.
- A failed Kubernetes list for `source='services'` records `complete=false`, so an all-`unknown` snapshot is distinguishable from a real outage.

## Source + Goal linkage

- **Source:** the 2026-09-16 investigation of "web services has been stuck at 7" (user question → Stripe/control-plane reconciliation). The headline question resolved to a non-defect (the count is genuinely 8 and the intake gate is working as designed), but the investigation surfaced nine real correctness defects, one of them actively failing in production.
- **Goal linkage:** [ADR040 billing](../../../docs/ADR040-billing-metronome.md) and [ADR075 onboarding](../../../docs/ADR075-user-onboarding.md) — a bound payment method is the single fact that decides whether a workspace may create anything, so the machinery that records it must be loud when it fails and must never lose a binding. Also [ADR043](../../../docs/ADR043-tenant-namespace-isolation.md) for the isolation-script leak.
- **Expected outcome:** no webhook failure can be silent or permanent; deleting an account leaves no payment instrument or PII in Stripe; analytics snapshots are honestly labeled.
- **Why now:** t001 is failing in production right now (`billing: Stripe lifecycle handler event=evt_1UGSEjEqsEqs2tLVTHfQYNzv type=customer.subscription.deleted: … violates foreign key constraint … (SQLSTATE 23503)`), and it repeats on every account deletion. Stripe disables endpoints that fail persistently; a disabled endpoint would drop `checkout.session.completed` and silently lock real customers out of the product — the exact catastrophic failure this investigation started out suspecting.
- **Render parity closing task omitted:** this milestone changes no REST/GraphQL/MCP field, error shape, or dashboard surface. The only external contract it touches is bex's Stripe webhook receiver, which has no Render counterpart. The user-facing companion change lives in `w3/m163`.
