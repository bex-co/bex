# w4 · m81 — Stripe live-mode graduation readiness

**Worker:** worker4 **Goal:** every engineering blocker between today's prod test-mode billing and a safe `rk_live_*` cutover is closed, so going live becomes a pure runbook-§6 execution in a staffed window **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | --------------------------------------------------------------------------- | --- | ---------- |
| t001 | Mode-proof the payment-method marker across the test→live cutover — **DONE** | 45m | —          |
| t002 | Lift the test-only dunning fence for live mode — **DONE** | 45m | —          |
| t003 | Provision and require the live Customer Portal configuration — **DONE** | 30m | —          |
| t004 | Idempotent Stripe webhook-endpoint provisioning in the setup script — **DONE** | 45m | —          |
| t005 | Live-safe read-only billing reconciliation report — **DONE** | 45m | —          |
| t006 | Rewrite runbook §6 into the executable go-live checklist — **DONE** | 45m | t001–t005  |
| t007 | Simplify — **DONE** | 30m | t006       |
| t008 | Test coverage — **DONE** | 45m | t006       |
| t009 | Closeout — **DONE** | 15m | t008       |

## Definition of done

All of the following hold with **no live key ever deployed** (the cutover itself is explicitly out of scope):

1. A PG regression test proves a `billing_provider_mappings` upsert that flips `livemode` clears `payment_method_bound_at`, closing the paid-intent gate and the usage-export gate for that workspace.
2. bex-api boots with a live-shaped key **and** `BEX_STRIPE_DUNNING_ENABLED=1` (config-level test); the secret script accepts the pair under `BEX_STRIPE_ALLOW_LIVE=1`.
3. `scripts/stripe-billing-secret.sh` refuses a live key without `BEX_STRIPE_PORTAL_CONFIGURATION_ID` (DRY_RUN-verifiable); test-mode behavior is byte-identical.
4. `scripts/stripe-billing-setup.py --webhook-url …` creates/verifies the pinned-version ten-event webhook endpoint idempotently (second run reports `exists`; drift → non-zero exit; no secret in argv/logs).
5. `scripts/stripe-billing-reconcile.sh report` runs under a live key behind an explicit flag; `repair` remains refused.
6. Runbook §6 is a top-to-bottom executable checklist including the marker reset, portal, webhook, tax activation (operator-confirmed classification), epoch semantics, and the alert watch list; both stale "13 items" occurrences read 14.
7. Backend suite + lint green; prettier clean on all touched markdown.

## Source + Goal linkage

- **Source:** 2026-08-15 live-cutover deep-research session (three-agent sweep: mode-tainted local state, Stripe Tax activation path, runbook §6 gap analysis), following the user's go-live question; plan of record was previously only `docs/runbooks/stripe-billing-setup.md` §6. Prior fences: w7/m51–m53 all scoped live mode out; `.pm/FUTURE-MAYBE.md:15` gated collection on the first paying tenant — which now exists (`tea-d98210cbbpdc73dcrkvg`, first invoice 2026-08-27).
- **Goal linkage:** revenue viability of the Render-alternative core (ADR008) — billing must collect real money before paid tiers mean anything; completes the ADR040/ADR046 billing plane's last mile.
- **Expected outcome:** the operator can execute a live cutover from the runbook alone: no workspace silently keeps paid access without a live card (marker), non-payment is enforceable (dunning), the portal cannot cancel subscriptions (scoped `bpc_*`), the webhook endpoint cannot drift (scripted), and post-cutover drift is observable (live report).
- **Why now:** the first real billing cycle closes 2026-08-27; every test-mode workspace binding is a live-mode security/revenue hole (`payment_method_bound_at` survives the key flip and reopens the ADR046 gate with no live card on file). Graduating without these fixes ships known revenue-integrity and invariant-breaking defects.
- **Render parity:** omitted — no REST/GraphQL/MCP/UI shape changes; the work is store-internal marker semantics, env-gated runtime wiring, ops scripts, and runbook docs. The readiness surface (`workspaceBillingReadiness`) is untouched.
