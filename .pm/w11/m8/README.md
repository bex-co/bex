# w11 · m8 — Tier-2 mobile quick actions

**Worker:** worker11 **Goal:** add ADR048's narrow fast-follow conveniences—single env-var edit, cron controls, datastore companion views, usage glance, and invite acceptance—without importing desktop bulk/admin/configuration surfaces. **Status:** blocked on w11/m3 and w11/m4

## Gating

Start after `w11/m3/t009` and `w11/m4/t008`; independent of agent milestones.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add one-at-a-time env-var quick view/edit | 60m | w11/m3/t009, w11/m4/t008 |
| t002 | Add cron history, run-now, and cancel controls | 60m | t001 |
| t003 | Add Postgres/Key Value status and read-only insights | 60m | t002 |
| t004 | Add usage/month-to-date glance and invite acceptance | 60m | t003 |
| t005 | Audit Tier-2 scope exclusions and Render parity | 30m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 60m | t005 |
| t008 | Closeout | 10m | t007 |

## Definition of done

One env var can be deliberately viewed/edited, cron history/run-now/cancel works, datastore status/connection health/read-only insights render, usage/month-to-date is glanceable, and invite deep links redeem safely. Bulk import, env groups, secret files, parameter/PITR/failover/allowlist/plan controls, billing administration, and destructive actions remain absent. Every reused verb preserves authorization, audit, error, and confirmation behavior across existing surfaces.

## Source + Goal linkage

- **Source:** ADR048 D3/M3 fast follows.
- **Goal linkage:** extends mobile convenience after the core supervision/safety loop is proven, without diluting ADR048's phone-not-configuration thesis.
- **Expected outcome:** common narrow follow-ups finish from a phone while consequential setup remains desktop-only.
- **Why now:** it is sequenced behind evidence and safe-operation patterns, reducing scope and safety risk; it does not wait on agent phase 2.
- **Render parity:** included for env, cron, datastore, usage, and invite contracts; deliberate mobile omissions are documented rather than treated as API drift.
