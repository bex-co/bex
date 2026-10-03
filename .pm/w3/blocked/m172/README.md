# w3 · m172 — Stop leaking cardless Stripe Customers

**Worker:** worker3 **Goal:** a bex Stripe Customer exists only for a workspace that bound a payment method, is comped, or is mid-way through a still-live setup flow — every abandoned flow is either never minted or reclaimed **Status:** blocked (t001–t005, t007–t009 done; t006 live audit run needs the operator's Stripe key; t010 closeout follows it)

## Tasks (in order)

| id   | title                                                                              | est   | depends_on       |
| ---- | ---------------------------------------------------------------------------------- | ----- | ---------------- |
| t001 | Classify the existing empty `tea-*` Customers by creation path (read-only audit) — **DONE** | 45m   | —                |
| t002 | Decide and record the Customer-lifecycle rule (lazy mint vs reclaim) in ADR040 — **DONE** | 30m   | t001             |
| t003 | Stop minting Customer + Subscription before Checkout completes — **DONE** | 1h30m | t002             |
| t004 | Reclaim abandoned-checkout Customers with a bounded cleaner — **DONE** | 1h    | t002             |
| t005 | Close the workspace-create leak when the Customer id is never persisted — **DONE** | 45m   | —                |
| t006 | Remediate the existing orphaned Customers (dry-run-first script) — **BLOCKED (operator live audit run)** | 45m   | t001, t004       |
| t007 | Render parity: billing readiness/checkout across REST, GraphQL, MCP, and dashboard — **DONE** | 30m   | t003, t004, t005 |
| t008 | Simplify — **DONE** | 30m   | t007             |
| t009 | Test coverage — **DONE** | 1h    | t007             |
| t010 | Closeout                                                                           | 15m   | t006, t009       |

## Definition of done

- Clicking "Add payment method" and abandoning Checkout leaves **no** live Stripe Customer or Subscription for that workspace once the reclaim horizon passes (or none is minted at all, per t002's decision) — proven by a test against the Stripe fake and by a test-mode run of the real flow.
- A workspace-create attempt whose SetupIntent creation or `SetWorkspaceCreationSetup` write fails, then is abandoned, has its Customer deleted by `WorkspaceCreationCleaner` (located by the `workspaceCreationAttemptMetadataKey` metadata, not only by the persisted id).
- Completing Checkout still binds the card, creates exactly one Subscription, stamps `payment_method_bound_at`, and lifts the payment gate — the m162 invariants (`pendingSetupMetadataKey`, `checkout_started_at`) still hold or are deliberately retired in ADR040.
- Comped workspaces, admin `provision`/`comp`, the gate-off emitter path, and account-deletion's retained (anonymized) Customers are untouched and documented as intended Customer owners.
- After t006, the t001 audit shows only intended Customer owners, with before/after counts recorded in t006 (or the live run explicitly recorded as owed pending authorization).

## Source + Goal linkage

- **Source:** 2026-10-02 Q&A investigation "why does Stripe have so many `tea-*` Customers with no payment" — the user's model was "we only create a Customer when the user adds a payment method"; code reading proved otherwise. `CreateCheckoutSession` (`lego/backend/internal/billing/sessions.go:117`) calls `EnsureContract` → `EnsureCustomer` (`stripe.go:204`, Name = workspace id, no email) and `Subscriptions.New` (`contracts.go:37`) **before** the hosted page loads, from four dashboard entry points (`payment-method-card.tsx:212`, `payment-required.tsx:206`, `billing-onboarding.tsx:236`, `payment-setup-page/index.tsx:136`). Nothing ever reclaims them. The workspace-create path (`workspace_creation.go:50`) is reclaimed after 24h by `WorkspaceCreationCleaner` (`workspace_creation.go:232`, wired at `cmd/api/main.go:856`) except when the Customer id never reached the DB.
- **Goal linkage:** ADR040 billing correctness ([docs/ADR040-billing-metronome.md](../../../docs/ADR040-billing-metronome.md)) and the m162 lineage ("a live subscription must not read as a bound card"). Billing state in Stripe must mean what it says before open signup.
- **Expected outcome:** the Stripe Customer list stops reading as a payment-funnel failure; Customer/Subscription counts become a trustworthy signal; no cardless `charge_automatically` Subscription can accumulate usage and fall into failed collection if the gate is ever turned off.
- **Why now:** the leak is unbounded and grows with every Checkout click (including `scripts/stripe-billing-verify.sh:89` runs); it is already misleading the operator about funnel health, and remediation gets more expensive with every orphan. Open signup multiplies it.
- **Render parity included** because t003/t004 change when `customerReady`/`subscriptionReady` are true on `workspaceBillingReadiness` (REST/GraphQL/MCP) and what the dashboard onboarding shows before Checkout.
