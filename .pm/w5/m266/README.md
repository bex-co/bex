# w5 · m266 — Key Value Pro, Pro Plus and size-named plans create, as on Render

**Worker:** worker5 **Goal:** a customer can create and upgrade to Key Value stores larger than 1 GB using Render's plan names on every surface. **Status:** todo (t001–t004 and t008 done: tiers, placement, admission, prices, the one Key Value cost meter (D5) and the dashboard; Stripe meter live 2026-11-06; next — t005 live walk after deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's Key Value plan specs and add the tiers — **DONE** | 45m | — |
| t002 | Operator schedules large Key Value stores onto pools that fit them — **DONE** | 1h30m | t001 |
| t003 | Backend admission and price sheet accept the new plans on REST, GraphQL, MCP and Blueprints — **DONE** | 1h | t001 |
| t004 | Dashboard plan picker and ADR updates — **DONE** | 1h | t003 |
| t005 | Live: create a pro store in production and upgrade standard → pro | 1h | t002, t004 |
| t006 | Dated widen task: 20g and 40g go live once w2/m250's large pools exist | 30m | t005 |
| t007 | Render parity | 30m | t006 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage | 45m | t007, t008 |
| t010 | Closeout | 10m | t009 |

Total: **10 tasks, about 7h30m**.

## Definition of done

- Render's `keyValuePlan` values `pro`, `pro_plus`, `5g`, `10g`, `20g`, `40g` are accepted on REST, GraphQL, MCP, Blueprints and the dashboard, with prices.
- A pro store runs and connects in production; a standard → pro upgrade grows its PVC.
- 20g and 40g are verified live on w2/m250's large pools, or their remaining gate is recorded with an owner (and any interim refusal carries the user's ADR018 sign-off).

## Source + Goal linkage

- **Source:** `/pm-brainstorm w5` 2026-11-05 item 2; `lego/types/tiers/tiers.yaml:155`; Render CLI v2.35.0 `KeyValuePlanValues`; ADR018 row 222; Render changelog 2026-10-01 (Key Value plans repriced, 3 GB/50 GB added).
- **Goal linkage:** ADR021 managed Key Value; ADR008 Render parity first.
- **Expected outcome:** Key Value stores above 1 GB exist on bex.
- **Why now:** one of Render's most common upgrade paths is impossible on bex today, and Render just reworked its Key Value ladder.
- **Render parity:** included.
- **Open decision (not yet signed):** whether 20g/40g get a temporary named refusal while w2/m250's pools are blocked; t006 records the outcome.

## Progress (2026-11-05)

- **t001 findings — Render's current Key Value ladder** ([render.com/docs/compute-plans](https://render.com/docs/compute-plans) and [render.com/pricing](https://render.com/pricing#key-value), read 2026-11-05). Plan ids are memory sizes; "legacy plan names remain valid across all Render tooling":

  | Plan id | Legacy name | RAM / storage | Connections | Render $/mo (size plan) | Render legacy $/mo | bex at ×0.70 |
  | --- | --- | --- | --- | --- | --- | --- |
  | `256mb` | Starter | 256 MB / 256 MB | 200 | 10 | 10 | 7.00 (bex `starter` today) |
  | `1g` | Standard | 1 GB / 2.5 GB | 250 | 20 | 32 | **14.00** (bex `standard` is 21.00 today, above Render) |
  | `3g` | — | 3 GB | 3,000 | 60 | — | 42.00 |
  | `5g` | Pro | 5 GB | 5,000 | 100 | 135 | 70.00 |
  | `10g` | Pro Plus | 10 GB | 10,000 | 200 | 250 | 140.00 |
  | `20g` | Pro Max | 20 GB (high CPU) | 20,000 | 350 | 550 | 245.00 |
  | `40g` | Pro Ultra | 40 GB (high CPU) | 40,000 | 600 | 1,100 | 420.00 |
  | `50g` | — | 50 GB (high CPU) | 50,000 | 750 | — | 525.00 |

  The pinned CLI v2.35.0 `KeyValuePlanValues` lists `pro`, `pro_plus`, `5g`, `10g`, `20g`, `40g` but not yet `3g`, `50g`, `pro_max`, `pro_ultra`; Render's docs accept all of them.

- **t002 placement plumbing (shipped, inert until a tier names a pool):** `ValkeyTier.servingPool` / `paused` (a pool must be one the compute ladder names; the default tier may not be paused); the operator places the Valkey StatefulSet with `execution.PlaceOnServingPool`; workload admission admits the exact pool toleration on a KeyValue-controlled StatefulSet (`variables.keyValueStatefulSet`), proven by the admission envtest (every other platform workload is still refused); `gitops-validate.sh` pins the clause; ADR060 addendum.

- **Why the rest cannot ship yet:** a tier needs a `pricing.yaml` price (tests pin it), and every price is a billable meter: `billing/contracts.go` refuses all contract work ("active price with lookup key … is missing") until `scripts/stripe-billing-setup.py --live` has created it. So the live Stripe catalog must change **before** the deploy that adds the tiers.

## BLOCKED — decisions needed from the user (2026-11-05)

1. **Authorize the live Stripe catalog change** (runbook `docs/runbooks/stripe-billing-setup.md`, as w2/m171 did): create the new Key Value Prices at Render size-plan × 0.70 — `3g` $42, `pro`/`5g` $70, `pro_plus`/`10g` $140, `pro_max`/`20g` $245, `pro_ultra`/`40g` $420, `50g` $525 per month. Recommendation: yes; bex's tier ids follow its existing legacy-name ladder (`pro`, `pro_plus`, …) with the size ids as aliases, like `256mb`→`starter`.
2. **Reprice `standard`** from $21.00 to $14.00/mo (Render cut `1g` to $20 on 2026-10-01, so bex is now above Render, against ADR030). Existing subscriptions would move at their next period, the w2/058 pattern. Recommendation: yes, same rollout as w2/058.
3. **Confirm how the `xxl`-pool tiers behave** while the Hetzner dedicated-core limit blocks w2/m250: (a) extend the signed Pro Ultra pause to Key Value `pro_ultra`/`40g` and `50g` (refused by name, shown unavailable) — recommended, same pool and release condition; or (b) accept them and let them sit Pending. `pro_plus`/`pro_max` follow your w2/m250 decision for the `xl` pool (accepted, Pending until cx53 restocks).

**Next steps once decided:** add the tiers + aliases to `tiers.yaml` / `aliases.go` / the KeyValue CRD enum (t001), size their pods and PVCs (t002 envtest), prices + admission + Blueprint (t003), dashboard picker (t004), run the Stripe catalog script live, then deploy and run the t005 live checks; t006 then tracks the `xxl` release.

## Unblock decisions — 2026-11-05 (user, `/unblock w5`)

Affects w5/m266 t001–t006. Source: **explicit user decisions** (D1=A, D2=A, D3=A), checked on main `9c487a399`.

- **D1 — live Stripe prices: authorized.** Create the new Key Value Prices at Render size-plan × 0.70 (ADR030): `3g` $42, `pro`/`5g` $70, `pro_plus`/`10g` $140, `pro_max`/`20g` $245, `pro_ultra`/`40g` $420, `50g` $525 per month. Additive only. Run `scripts/stripe-billing-setup.py` in test mode, then `--live`, **before** the deploy that adds the tiers, because billing refuses all contract work while a catalog Price is missing. Reference: render.com/pricing#key-value, read 2026-11-05 (256mb $10, 1g $20, 3g $60, 5g $100, 10g $200, 20g $350, 40g $600, 50g $750).
- **D2 — reprice `standard` from $21.00 to $14.00/mo (Render 1g is $20 since 2026-10-01).** Existing subscriptions move at their next billing period, following the w2/058 pattern (`docs/runbooks/stripe-billing-setup.md` § Repricing a dimension): create the new Price, retire the old one, then swap the live subscription items with `proration_behavior=none` after each subscription's current period ends. If that boundary is still in the future when the code ships, the swap is a dated follow-up inbox note with that date as its only gate, like w2/058.
- **D3 — `pro_ultra`/`40g` and `50g` share the signed Pro Ultra pause** (w2/m250, user sign-off 2026-11-02; same `xxl` pool, same release condition: the Hetzner dedicated-vCPU limit raised to ≥32). Mark them `paused: true` in the valkey family, and add the backend refusal across REST, GraphQL, MCP, Blueprints and the dashboard picker, mirroring compute's `INSTANCE_TYPE_UNAVAILABLE`. Record the divergence and this sign-off in ADR018. Release them together with Pro Ultra. `pro_plus`/`10g` and `pro_max`/`20g` follow the w2/m250 `xl` decision: accepted, Pending until cx53 stock returns.

**Worker next steps:**
1. Add the tiers and aliases to `tiers.yaml`, `aliases.go` and the KeyValue CRD enum, re-adding `ValkeyTier.paused` with its enforcement.
2. Size the pods and PVCs, with an envtest for an `xl` tier.
3. Add the prices to `pricing.yaml`, including the `standard` repricing.
4. Wire admission, Blueprint and the dashboard picker.
5. Run the Stripe catalog in test mode, then live.
6. Ship, then run the t005 live checks: a `pro` store Ready and connectable, and a standard → pro upgrade that grows the PVC.
7. Track the `xl` and `xxl` release in t006.

**Acceptance:** the milestone's DoD is unchanged.

## Decision D5 — 2026-11-05 (user, raised during `/loopx w5`)

**Finding:** every billable dimension is one item on the workspace's single Stripe Subscription, and Stripe caps a Subscription at 20 items. The catalog had 19, and six new Key Value tiers would make 25 (`billing/contracts_sync_test.go` `TestCatalogFitsOneSubscription`).

**Decision (user, recommended option):** Key Value becomes **one cost-denominated dimension**. Every Key Value tier's instance seconds are rated at that tier's `pricing.yaml` rate and metered in micro-USD on a single Key Value Price, the way `agent_model_cost_microusd` already works (ADR091). The catalog goes from 19 to 18 dimensions. The invoice shows one "Key Value" line instead of a line per tier; per-instance detail stays on bex's Billing page. The retired `instance_seconds.key_value.starter` / `.standard` subscription items move at their next period, together with D2's `standard` repricing (w2/058 pattern). ADR040 records the change.

### D5 Stripe provisioning — 2026-11-06 (`/loopx w5`)

- Created with the setup script's own `ensure_meter`/`ensure_price` for this one dimension only (the `agent_model_cost_microusd` Price stays with w10/m14 t007), each run twice to confirm idempotence (`exists` on the second run):
  - test: meter `mtr_test_61Vi6z6Qg4SgyFK8341EqsEqs2tLVV8K`, Price `price_1UYj1yEqsEqs2tLVGmbPJI9m` (0.0001¢ per unit).
  - live: meter `mtr_61Vi702MFL0MyL33741EqsEqs2tLVV3o`, Price `price_1UYj2nEqsEqs2tLVQhG81soo` (0.0001¢ per unit).
- Read-only catalog diff: live matches `pricing.yaml` on every other dimension. Test mode carries older rounding on the six service Prices and egress/build/storage, so a full test-mode run of the script refuses at `instance_seconds.service.starter`; that drift predates m266 and is unrelated.
- Migration: 11 active live Subscriptions carry `instance_seconds.key_value.starter` and `.standard` items (15–18 items each; all periods end 2026-11-16). After the deploy, `syncContractItems` adds the cost item (≤ 19 items) and the old items receive no further usage, so the D2 price cut takes effect from the deploy rather than the next period — a cut only, so nobody is overcharged. The old items are removed after the 2026-11-16 invoices finalize: [w5/180](../blocked/180.md).
