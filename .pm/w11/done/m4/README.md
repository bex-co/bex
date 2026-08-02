# w11 · m4 — Safe one-tap operations

**Worker:** worker11 **Goal:** add the short, reversible, pre-parameterized operational verbs ADR048 permits on a phone, with confirmations and retry-safe feedback, while enforcing a hard absence of destructive and configuration-heavy actions. **Status:** done

## Gating

Start after `w11/m3/t009`.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add deploy trigger, cancel, and rollback actions — **DONE** | 60m | w11/m3/t009 |
| t002 | Add service restart, suspend, and resume actions — **DONE** | 45m | t001 |
| t003 | Add approved Postgres and Key Value lifecycle actions — **DONE** | 45m | t002 |
| t004 | Enforce confirmations, in-flight dedupe, audit feedback, and route exclusions — **DONE** | 45m | t003 |
| t005 | Render parity — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

Trigger/cancel/rollback, restart, and approved suspend/resume actions converge correctly on real resources, cannot double-submit while in flight, and explain authorization, conflict, timeout, and partial failures. Automated route/action inventory proves delete, PITR, failover, workspace deletion, plan/admin, autoscaling, registry, key, and other ADR048 D4 surfaces are absent. Every mutation is authorized and audited by the existing server core. Generated operations validate against the live backend schema; deploy/app/Postgres/Key Value backend packages and 133 native-client tests pass; iOS/Android production bundles pass; and the native app builds, installs, and launches on an iOS simulator. Authenticated physical-device qualification remains mandatory before distribution and is recorded in `mobile/e2e/m4-safe-actions.md`; this source milestone did not use production tenant credentials.

## Source + Goal linkage

- **Source:** ADR048 D2/M1 safe verbs and D4 destructive-verb policy.
- **Goal linkage:** ADR008's API-first operational loop and Render-compatible behavior.
- **Expected outcome:** an on-call user can safely recover common incidents from a phone without turning mobile into a dangerous configuration console.
- **Why now:** it completes the supervision loop only after m3 proves evidence and network behavior.
- **Render parity:** included across REST/GraphQL/MCP/dashboard/native action semantics and errors; no mobile-only mutation is introduced.
