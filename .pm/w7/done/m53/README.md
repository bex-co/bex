# w7 · m53 — Production Stripe test-mode billing acceptance + operations handoff

**Worker:** worker7 **Goal:** prove and operationalize the complete bex billing lifecycle against Stripe test mode from the production deployment, with reconciliation, alerts, incident drills, evidence, and no live-mode side effects **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reconcile usage outbox, Stripe meter summaries, and invoice lines — **DONE** | 1h | — |
| t002 | Add secret-safe billing metrics and structured operator diagnostics — **DONE** | 1h | t001 |
| t003 | Alert on backlog, rejects, ambiguity, duplicates, and webhook drift — **DONE** | 1h | t002 |
| t004 | Build dead-letter and old-ambiguity repair workflows — **DONE** | 1h | t001, t002, t003 |
| t005 | Harden test credential rotation, access policy, and custody — **DONE** | 45m | t002 |
| t006 | Provision the paid · excluded · comp production test workspace set — **DONE** | 45m | t001, t005 |
| t007 | Run the full production Stripe test-clock billing lifecycle — **DONE** | 2h | t003, t004, t006 |
| t008 | Drill billing disable, rollback, credential rotation, and recovery — **DONE** | 1h | t005, t007 |
| t009 | Publish the operator acceptance evidence and recurring test runbook — **DONE** | 45m | t007, t008 |
| t010 | Simplify — **DONE** | 30m | t009 |
| t011 | Test coverage — **DONE** | 45m | t010 |
| t012 | Closeout — **DONE** | 10m | t011 |

## Definition of done

The production deployment, using only its dedicated `rk_test_*` and test webhook secret, completes a documented Stripe test-clock lifecycle for three isolated workspaces: paid, structurally excluded, and 100%-comped. Evidence shows usage rollup → sealed outbox → deterministic meter events → rated invoice lines → Checkout payment readiness → tax-aware preview where the collecting test-registration gate is satisfied → invoice finalization/payment → Portal access → failed renewal → grace/enforcement → successful recovery. Reconciliation accounts for every selected `usage_hourly` row and flags permanent rejects or ambiguity older than Stripe's identifier window instead of replaying blindly. Prometheus alerts and operator diagnostics cover backlog, stamp failure, duplicate Customer/Subscription, invoice-read degradation, and webhook drift without exposing secrets. Disable/rollback and credential-rotation drills succeed, disposable Stripe and bex fixtures are cleaned up, and the recurring runbook contains evidence locations but no credential values. No live object, live key, real payment, or production customer is touched.

## Source + Goal linkage

- **Source:** User request on 2026-07-27 to finish the entire billing system in Stripe test mode inside prod and hand it to w7; depends on m51's onboarding and m52's dunning lifecycle.
- **Goal linkage:** Advances ADR008's deterministic and operable hosted-platform goal by converting the billing path from code-complete into a continuously verifiable production capability with machine-readable failure signals.
- **Expected outcome:** Operators can prove quantities, rating, collection simulation, failure handling, recovery, rollback, and secret rotation end to end before any live-mode decision.
- **Why now:** Prod test-mode export and webhook intake are already active; without reconciliation, alerts, lifecycle evidence, and drills, the current green smoke test is not a durable operational handoff.
- **Render parity:** Omitted because this milestone is an operations/acceptance layer and adds no REST, GraphQL, MCP, or dashboard contract; m51 and m52 own parity for their user-facing surfaces.
