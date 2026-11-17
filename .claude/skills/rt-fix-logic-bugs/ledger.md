# rt-fix-logic-bugs ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

- 2026-11-13 ADR012 §7 platform-client class exemption — the ADR exempts registry-listed platform clients from the class check, `Identity.CapabilityExempt` does not, and `TestScopeClassExemptions` pins the 403; fail-closed today, needs an ADR decision rather than a bug fix

## Skipped

- 2026-11-17 ApplyBlueprintServiceSpec service type change — the plan reports an update naming `type` while `updateStackService` refuses it, but `TestBlueprintStillDetectsARealTypeChange` pins the plan's detection and the apply's refusal message tells the user to delete and recreate; deliberate split, not a logic fix
- 2026-11-17 ApplyBlueprintServiceSpec explicit-null service fields — a manifest `repo:`/`branch:` null cannot reach the appliers; the compiler schema rejects null scalars with BLUEPRINT_SCHEMA_INVALID
- 2026-11-17 x-bex.builder non-auto values — unreachable via valid manifests; the schema requires `runtime` and `specFromCreate` refuses a runtime+builder pair, so only `auto` (a build-strategy noop) passes
- 2026-11-16 stripeBillingGate future BEX_STRIPE_EPOCH — accepted and pushes the emitter floor forward, permanently skipping pre-epoch windows; rejecting it is validation hardening needing an ADR decision, not a logic fix
- 2026-11-16 MonthlyEstimate negative StorageGB/seconds inputs — callers clamp to non-negative (plan floors, high-water marks, admitted sizes); unvalidated direct use is out of contract
