# w7 · m48 — Billing surface: real invoices and cost from Metronome

**Worker:** worker7 **Goal:** turn the validated m47 export into visible real billing — per-customer contracts, real invoiced/current cost read back from Metronome, surfaced beside the advisory estimate **Status:** done

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Contract provisioning: per-customer Metronome contract (rate card + period) — **DONE**     | 45m | —          |
| t002 | Metronome read client: current-period cost + finalized invoices — **DONE**                | 45m | —          |
| t003 | Surface real billing on the usage API (REST + GraphQL + MCP) — **DONE**                    | 1h  | t002       |
| t004 | Dashboard: real invoiced / current spend beside the "estimate only" card — **DONE**        | 45m | t003       |
| t005 | Render parity — **DONE**                                                                   | 30m | t004       |
| t006 | Simplify — **DONE**                                                                        | 30m | t005       |
| t007 | Test coverage — **DONE**                                                                   | 45m | t006       |
| t008 | Closeout — **DONE**                                                                        | 10m | t007       |

## Definition of done

A workspace with a Metronome contract sees its real current-period cost and finalized invoices over REST, GraphQL, and MCP — the same fields/semantics on all three — and in the dashboard, visually distinct from the advisory estimate. `estimatedCost` remains for the in-flight (pre-seal, <48h) window. Comped/superadmin tenants are handled via ADR040 §7: Mode A (`billing_excluded` ⇒ no contract) or Mode B (100% credit + non-collectible ⇒ a real invoice showing gross − comp = $0 due).

## Source + Goal linkage

- **Source:** `docs/ADR040-billing-metronome.md` §8 Phase 2.
- **Goal linkage:** same as m47 — `GOAL.md` #5's billing half; makes billing real (not just estimated) to users, the visible half of a hosted offering (`docs/ADR008-vision.md`).
- **Expected outcome:** workspaces see actual invoices and current spend, not only the `pricing.yaml` estimate.
- **Why now:** sequenced strictly after m47 proves Metronome's computed totals match `usage_hourly` (t007 reconciliation) — exposing invoices before that reconciliation would risk billing users off an unvalidated mapping.
- **Render parity — INCLUDED (t005):** the new billing fields touch REST + GraphQL + MCP + dashboard; ensure identical fields/semantics/error shapes across all three surfaces. Render exposes no billing API, so this is bex-ahead **internal** consistency, not Render-matching (noted so the parity task checks cross-surface, not render.com).

## Closeout note (2026-07-20)

Shipped the Phase-2 billing surface on top of m47's export: `internal/billing/read.go` (Metronome read client — `BillingFor` = current-period `ListCosts` + finalized `Invoices`, normalized to `Amount`/`Invoice`, cents-aware; no-contract ⇒ nil (estimate-only), degraded ⇒ error the caller swallows, never 500; SDK retries disabled so the hot path fails fast), `internal/billing/contracts.go` (`EnsureContract` — idempotent list-then-create bound to `BEX_METRONOME_RATE_CARD_ID`, folded into the emitter's per-customer `ensureBillingSetup`, no-op without a rate card ⇒ m47 byte-identical; `CompCustomer` — Mode B contract + ≥balance credit grant, idempotent via uniqueness key), a `billing` object on `usage.Summary` threaded identically through REST/GraphQL/MCP beside `estimatedCost` (one-core/thin-adapters), a short positive-only `core.TTLCache` in front of the two Metronome reads on the `/v1/usage` hot path, and the dashboard's distinct **Current Spend** card (an _Invoice_ badge, real cost + finalized invoices; estimate-only workspaces render the estimate card alone). Config: `BEX_METRONOME_RATE_CARD_ID` + `BEX_METRONOME_USD_CREDIT_TYPE_ID` (the latter only for comps); contract `starting_at` = the billing epoch.

**Verified here:** `go build ./...` + `go test ./...` green; backend lint 0 issues; dashboard `yarn typecheck` + `yarn lint` + full `yarn test` (1617 tests) green. Cross-surface parity is asserted by a test that runs the REST handler **and** the GraphQL resolver and diffs the `billing` values; degraded/no-contract/nil-reader all fall back to estimate-only without a 500; the TTL cache collapses repeated polls to one Metronome read while never caching an error; contract idempotency + the Mode B comp mechanism are unit-tested with a stub transport. Dashboard GraphQL types were regenerated **offline** via the `SCHEMA_JSON` dump path (`TestDumpGraphQLSchema` → `yarn codegen`), no live API needed. `/simplify` (4 agents) applied: twin provisioning caches collapsed to one `cached`/`mark` pair, the hot-path TTL cache added, the `CompCustomer` comment corrected (the audited admin verb is a follow-up, not shipped), and the MCP `get_usage` description updated to advertise `billing`.

**Residual (live, manual):** the DoD's real contract→invoice read-back and the Mode B `$0-due` invoice require a configured Metronome org + `BEX_METRONOME_TOKEN` + rate card (ADR040 Phase 0 runbook) — not runnable in this env, the same env/credential-gated live step as m47's reconciliation. With the token unset the whole surface is inert (estimate-only, byte-identical). Follow-up filed implicitly: an audited control-plane `billing-comp` verb to trigger `CompCustomer` from ops (mirroring m47's billing-excluded verb); Phase 3 (collection + the ADR040 §9 enforcement ladder) remains deferred.
