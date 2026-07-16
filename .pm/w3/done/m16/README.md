# w3 · m16 — Events feed fidelity: required from/to details + auto_deploy discrimination

**Worker:** worker3 **Goal:** Close the two closable ◐ divergences the ADR018 Service-events row records: `plan_changed`/`instance_count_changed`/`autoscaling_config_changed` gain the `from`/`to` details Render marks **required**, and `auto_deploy_changed` splits into Render's `auto_deploy_enabled`/`auto_deploy_disabled` — via the typed-nullable-field precedent maintenance mode established, never a generic verb-arguments object. **Status:** done — **DONE**

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Audit seam: typed, per-verb non-secret detail fields (plan from/to · replica from/to · autoDeploy bool), the maintenance-boolean pattern | 60m | —          | — **DONE** |
| t002 | Projection: the three event types emit Render's required `from`/`to`; auto-deploy events discriminate by the recorded boolean | 30m | t001       | — **DONE** |
| t003 | Refresh the ledger row (divergences 1+2 closed; 3+4 stay with their recorded rationale) + `scripts/events-verify.sh` coverage | 30m | t002       | — **DONE** |
| t004 | Render parity — conformance suite validates the events route's new required fields; three-surface consistency  | 20m | t003       | — **DONE** |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                   | 20m | t004       | — **DONE** |
| t006 | Test coverage — event-projection tests incl. legacy rows without recorded details (omit, don't fake)          | 30m | t004       | — **DONE** |
| t007 | Closeout — DoD met → move milestone to `done/`                                                                | 10m | t006       | — **DONE** |

## Definition of done

A plan change, a manual scale, and an autoscaling-config change each produce an event whose `details` carry the real `from`/`to` values, validated by the w7/m30 conformance suite without an allowlist entry; toggling auto-deploy produces `auto_deploy_enabled`/`auto_deploy_disabled` (Render's names); pre-existing audit rows without recorded details render their events with the fields omitted (never fabricated); the ADR018 row's divergence list shrinks by exactly entries (1) and (2).

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 10 (2026-07-15) — the ADR018 Service-events row (◐, line 35) records both divergences and itself names the safe mechanism: maintenance mode's "typed nullable boolean supports exact … output without admitting arbitrary values" exception to the no-verb-arguments rule (w4/m10's secret-safety hole stays closed — a plan name, a replica count, and a boolean are not secrets).
- **Goal linkage:** Render parity (pillar 1); w3's events feature (`w3/m7`) is the surface being polished.
- **Expected outcome:** event consumers (dashboard Events tab, `list_service_events`, w3/m11 webhooks) see real before/after values on the three types Render requires them for.
- **Why now:** the fields are *required* in Render's schema — the conformance suite can't honestly validate the events route until they exist; the typed-field mechanism already shipped with maintenance mode, so the marginal cost is low.
- **Render parity closing task: included** (t004) — REST/GraphQL/MCP event shapes change.
