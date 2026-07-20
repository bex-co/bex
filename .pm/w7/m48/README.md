# w7 · m48 — Billing surface: real invoices and cost from Metronome

**Worker:** worker7 **Goal:** turn the validated m47 export into visible real billing — per-customer contracts, real invoiced/current cost read back from Metronome, surfaced beside the advisory estimate **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est  | depends_on |
| ---- | ------------------------------------------------------------------------------ | ---- | ---------- |
| t001 | Contract provisioning: per-customer Metronome contract (rate card + period)    | 45m  | —          |
| t002 | Metronome read client: current-period cost + finalized invoices               | 45m  | —          |
| t003 | Surface real billing on the usage API (REST + GraphQL + MCP)                    | 1h   | t002       |
| t004 | Dashboard: real invoiced / current spend beside the "estimate only" card       | 45m  | t003       |
| t005 | Render parity                                                                  | 30m  | t004       |
| t006 | Simplify                                                                        | 30m  | t005       |
| t007 | Test coverage                                                                   | 45m  | t006       |
| t008 | Closeout                                                                        | 10m  | t007       |

## Definition of done

A workspace with a Metronome contract sees its real current-period cost and finalized invoices over REST, GraphQL, and MCP — the same fields/semantics on all three — and in the dashboard, visually distinct from the advisory estimate. `estimatedCost` remains for the in-flight (pre-seal, <48h) window. Comped/superadmin tenants are handled via ADR040 §7: Mode A (`billing_excluded` ⇒ no contract) or Mode B (100% credit + non-collectible ⇒ a real invoice showing gross − comp = $0 due).

## Source + Goal linkage

- **Source:** `docs/ADR040-billing-metronome.md` §8 Phase 2.
- **Goal linkage:** same as m47 — `GOAL.md` #5's billing half; makes billing real (not just estimated) to users, the visible half of a hosted offering (`docs/ADR008-vision.md`).
- **Expected outcome:** workspaces see actual invoices and current spend, not only the `pricing.yaml` estimate.
- **Why now:** sequenced strictly after m47 proves Metronome's computed totals match `usage_hourly` (t007 reconciliation) — exposing invoices before that reconciliation would risk billing users off an unvalidated mapping.
- **Render parity — INCLUDED (t005):** the new billing fields touch REST + GraphQL + MCP + dashboard; ensure identical fields/semantics/error shapes across all three surfaces. Render exposes no billing API, so this is bex-ahead **internal** consistency, not Render-matching (noted so the parity task checks cross-surface, not render.com).
