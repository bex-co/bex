# rt-fix-logic-bugs ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

- 2026-11-13 ADR012 §7 platform-client class exemption — the ADR exempts registry-listed platform clients from the class check, `Identity.CapabilityExempt` does not, and `TestScopeClassExemptions` pins the 403; fail-closed today, needs an ADR decision rather than a bug fix

## Skipped

- 2026-11-18 billing/estimate-gate agreement incl. kept plans — audited, holds: the gate trips exactly when the estimate shows a priced line or variable entry on every valid stack, and both read stackAtKeptPlans (TestStackPaidPlanAgreesWithEstimate, kept-plan tests, pricing property tests all pass)
- 2026-11-18 billing/nil-fields kept-plan asymmetry — services with no field record use the create default while datastores keep the live plan, mirroring the apply (re-apply whole vs apply declared fields only); consistent, not a bug
- 2026-11-18 billing/KV estimate storage floor vs Valkey.EffectiveStorageGB — equivalent on valid stacks (a parsed spec carries no allocated high-water, unknown plans are rejected at parse); hand-rolled max() is duplication only
- 2026-11-18 billing/duplicate-display-name estimate — a kept-plan lookup miss means create (correct); a duplicate display name is refused 409 at apply, fail-closed on a degenerate state, not a logic fix
- 2026-11-17 deploys.GetDeployHook dispatch/verb conjunction — GET deploy-hook is OpClassMint at dispatch (needs bex.write) but AuthorizeApp(RelCanViewSensitive) at the verb (needs bex.sensitive), so a human token needs both plus the mint class; the verb comment justifies the sensitive half (same relation as database connection strings) and the conjunction only ever narrows, never widens; deliberate, not a logic fix
- 2026-11-17 ApplyBlueprintServiceSpec service type change — the plan reports an update naming `type` while `updateStackService` refuses it, but `TestBlueprintStillDetectsARealTypeChange` pins the plan's detection and the apply's refusal message tells the user to delete and recreate; deliberate split, not a logic fix
- 2026-11-17 ApplyBlueprintServiceSpec explicit-null service fields — a manifest `repo:`/`branch:` null cannot reach the appliers; the compiler schema rejects null scalars with BLUEPRINT_SCHEMA_INVALID
- 2026-11-17 x-bex.builder non-auto values — unreachable via valid manifests; the schema requires `runtime` and `specFromCreate` refuses a runtime+builder pair, so only `auto` (a build-strategy noop) passes
- 2026-11-16 stripeBillingGate future BEX_STRIPE_EPOCH — accepted and pushes the emitter floor forward, permanently skipping pre-epoch windows; rejecting it is validation hardening needing an ADR decision, not a logic fix
- 2026-11-16 MonthlyEstimate negative StorageGB/seconds inputs — callers clamp to non-negative (plan floors, high-water marks, admitted sizes); unvalidated direct use is out of contract
