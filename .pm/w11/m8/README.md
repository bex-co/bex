# w11 · m8 — Tier-2 mobile quick actions

**Worker:** worker11 **Goal:** add ADR048's narrow fast-follow conveniences—single env-var edit, cron controls, datastore companion views, usage glance, and invite acceptance—without importing desktop bulk/admin/configuration surfaces. **Status:** todo (t001–t003 done; t004 actionable)

## Gating

Start after `w11/m3/t009` and `w11/m4/t008`; independent of agent milestones.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add one-at-a-time env-var quick view/edit — **DONE** | 60m | w11/m3/t009, w11/m4/t008 |
| t002 | Add cron history, run-now, and cancel controls — **DONE** | 60m | t001 |
| t003 | Add Postgres/Key Value status and read-only insights — **DONE** | 60m | t002 |
| t004 | Add honest usage and month-to-date glance | 90m | t003 |
| t009 | Add verified invite-link acceptance | 90m | t004 |
| t005 | Audit Tier-2 scope exclusions and Render parity | 30m | t009 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 60m | t005 |
| t008 | Closeout | 10m | t007 |

## Definition of done

One env var can be deliberately viewed/edited with atomic conflict detection, cron history/run-now/cancel works without accepting impossible suspended runs, datastore status/freshness/read-only insights render, usage/month-to-date reports evidence coverage honestly, and OS-verified HTTPS invite links redeem safely with browser fallback. Bulk import, env groups, secret files, parameter/PITR/failover/allowlist/plan controls, billing administration, and destructive actions remain absent. Every reused verb preserves authorization, audit, error, and confirmation behavior across existing surfaces.

## Source + Goal linkage

- **Source:** ADR048 D3/M3 fast follows.
- **Goal linkage:** extends mobile convenience after the core supervision/safety loop is proven, without diluting ADR048's phone-not-configuration thesis.
- **Expected outcome:** common narrow follow-ups finish from a phone while consequential setup remains desktop-only.
- **Why now:** it is sequenced behind evidence and safe-operation patterns, reducing scope and safety risk; it does not wait on agent phase 2.
- **Render parity:** included for env, cron, datastore, usage, and invite contracts; deliberate mobile omissions are documented rather than treated as API drift.
