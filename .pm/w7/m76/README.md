# w7 · m76 — Sandbox compute metering: `sandbox_compute_seconds` (ADR047 D6)

**Worker:** worker7 **Goal:** sandboxes stop being bex's one unmetered resource — per-second, vCPU/GB-weighted sandbox compute flows through the existing `usage_hourly` → sealed-outbox → Stripe pipeline, gated by the ADR046 PaymentGate, and shows up on the usage surface. **Status:** todo

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | New meter kind + emitter: sandbox lifecycle → `usage_hourly` (weighted, hibernation-excluded) | 90m | —    |
| t002 | `pricing.yaml` + Stripe catalog entry (`scripts/stripe-billing-setup.py`)               | 30m | t001       |
| t003 | PaymentGate coverage for sandbox create (ADR046 paid intent)                            | 45m | t001       |
| t004 | Usage surface: sandbox rows in `GET /v1/usage` + GraphQL/MCP                            | 30m | t001       |
| t005 | Render parity: usage-surface consistency across REST/GraphQL/MCP                        | 30m | t002, t003, t004 |
| t006 | Simplify pass over the metering code                                                    | 20m | t005       |
| t007 | Test coverage: accrual windows, hibernation exclusion, gate, seal/export                | 45m | t005       |
| t008 | Closeout                                                                                | 10m | t007       |

## Definition of done

- A new ADR023 meter kind `sandbox_compute_seconds` accrues per-second sandbox lifecycle compute, weighted by the sandbox's vCPU/GB shape (AgentCore-style, ADR047 D6); hibernated/suspended time does **not** accrue (idle-free, keyed off the accurate phase signal — w3/m42 contract).
- Rows land in `usage_hourly` with per-meter cursors, seal on the `BEX_STRIPE_SEAL_HOURS` horizon, and export through the existing durable `emitted_at` outbox to a Stripe meter event — no new pipeline, one new kind (ADR040 mechanics).
- Sandbox create (and any future paid sandbox tier change) passes through the ADR046 `PaymentGate` under `BEX_REQUIRE_PAYMENT_METHOD=1`: cardless paid intent → 402/`PAYMENT_REQUIRED` across REST/GraphQL/MCP; cardless meter rows stay pending per ADR046's bounded write-off.
- `pricing.yaml` + the Stripe catalog (`scripts/stripe-billing-setup.py`) carry the new meter/price; the runbook (`docs/runbooks/stripe-billing-setup.md`) notes the addition.
- `GET /v1/usage`, the `usage` GraphQL query, and the `get_usage` MCP tool return sandbox usage rows consistent across surfaces.

## Source + Goal linkage

- **Source:** [docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) D6 meter 1 + gap 3 ("sandboxes are the one resource not yet metered"); `/pm-brainstorm` decomposition 2026-08-01. Pipeline: docs/ADR023-usage-metering.md + docs/ADR040-billing-metronome.md + docs/ADR046 (PaymentGate).
- **Goal linkage:** pillar 5 economics + billing correctness (w7's charter: platform integrity). Valuable regardless of ADR047's later waves — the w3/m32 sandbox surface has been billing-invisible since it shipped.
- **Expected outcome:** every sandbox-second is either metered, hibernation-excluded, or pending-by-gate — observable in `usage_hourly`, the usage API, and (Stripe-enabled) meter events.
- **Why now:** ADR047 wave 1 and an existing revenue-leak closure; agent sessions (w3/m41) must not launch compute that nothing meters. Render parity included (t005): the usage surface is tenant-facing across REST/GraphQL/MCP (bex extension — Render has no usage API; consistency per the ADR023 precedent).
