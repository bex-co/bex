# w7 · m47 — Shadow export: emit sealed usage to Metronome

**Worker:** worker7 **Goal:** ship real, reconciled usage to Metronome for every workspace via an env-gated, byte-identical-when-off backend sidecar — the billing MVP with zero charging risk **Status:** todo

## Tasks (in order)

| id   | title                                                                                  | est    | depends_on       |
| ---- | -------------------------------------------------------------------------------------- | ------ | ---------------- |
| t001 | Metronome tenant-setup runbook: billable metrics + products + rate cards from pricing.yaml | 1h     | —                |
| t002 | Metronome Go SDK client wrapper (auth, batch ≤100, retry/backoff, env-gated)            | 45m    | —                |
| t003 | Migration: add nullable `emitted_at` to `usage_hourly` (outbox column)                 | 20m    | —                |
| t004 | Customer provisioning + `tenants.billing_excluded` (admin-only, audited)               | 1h     | t002             |
| t005 | Seal-then-emit loop + `BEX_METRONOME_EPOCH` floor guard                                 | 1h15m  | t002, t003, t004 |
| t006 | `.env.example`/`.env.template` + CLAUDE.md env-table sync                               | 20m    | t005             |
| t007 | Reconciliation harness: Metronome totals vs `usage_hourly`                              | 45m    | t001, t005       |
| t008 | Simplify                                                                                | 30m    | t006, t007       |
| t009 | Test coverage                                                                           | 45m    | t008             |
| t010 | Closeout                                                                                | 10m    | t009             |

## Definition of done

With `BEX_METRONOME_TOKEN` set, bex-api emits each **sealed** `usage_hourly` row exactly once to Metronome `/v1/ingest` with a deterministic `transaction_id`; every non-excluded tenant has a Metronome customer keyed by ingest alias `tea-…`; the reconciliation harness shows Metronome's computed billable-metric totals match `usage_hourly` for a chosen period within rounding; `429`/`5xx` are retried and non-429 `4xx` land in a DLQ log without blocking the loop; the `BEX_METRONOME_EPOCH` floor prevents pre-billing backfill and the first-enable `4xx` flood; `billing_excluded` tenants stay out of Metronome entirely yet still see `estimatedCost`. **With `BEX_METRONOME_TOKEN` unset, behavior is byte-identical to today.**

## Source + Goal linkage

- **Source:** `docs/ADR040-billing-metronome.md` (§Decision 1–7); fires `.pm/FUTURE-MAYBE.md` "Subscription / invoices / payments" trigger (a hosted bex offering becomes roadmap-worthy).
- **Goal linkage:** `GOAL.md` #5 (usage metering) → its deferred **billing half**; enables the hosted-offering economics in `docs/ADR008-vision.md`. Builds directly on w8/m7's price sheet (`internal/pricing`) and the m1–m15 metering pipeline (`usage_hourly`).
- **Expected outcome:** real, reconciled usage lands in Metronome per workspace; the rating/invoicing engine is exercised end-to-end with no charging risk.
- **Why now:** the FUTURE-MAYBE trigger fired (user request 2026-07-19); this is the lowest-risk first step (env-gated, no tenant-facing change) and the hard prerequisite for m48's billing surface — the event→metric→rate mapping must be validated before a cent is exposed.
- **Render parity — OMITTED:** no REST/GraphQL/MCP/UI surface change. This is a pure internal export sidecar, and Render exposes no billing-export API to be consistent with. The user-facing billing surface is m48, where the parity task applies.
