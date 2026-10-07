# w4 · m179 — Revoke deleted Postgres logins even when objects reference the role

**Worker:** worker4 **Goal:** deleting an additional Postgres credential prevents new authentication while preserving tenant objects, even when a grant or ownership dependency would block DROP ROLE. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Retire deleted Postgres logins without dropping tenant objects | 45m | — |
| t002 | Audit all retirement aliases and preserve passing role controls | 35m | w4/m179/t001 |
| t003 | Render parity | 20m | w4/m179/t001, w4/m179/t002 |
| t004 | Simplify | 10m | w4/m179/t003 |
| t005 | Test coverage for granted logins and real SQL retirement | 45m | w4/m179/t003, w4/m179/t004 |
| t006 | Closeout | 15m | w4/m179/t005 |

## Definition of done

Repeat the owned Free/public PostgreSQL18 repro in [finding.md](finding.md), using fresh qa-prefixed fixtures and retaining generated passwords only locally:

- Create an additional login, grant SELECT/INSERT on the owner's marker table, and verify a fresh connection reads it. Delete through the dashboard; within60s on a healthy fixture, the saved credential cannot open a new verified-TLS connection, while the owner still reads the unchanged marker.
- Reload the active-user page: the retired login is omitted. Any retained SQL role is NOLOGIN with a NULL password; a role may remain to preserve dependencies. Deletion reports accepted revocation intent before settle rather than a completed SQL drop.
- Remove the test grant, recreate the same name, and verify the newly returned password connects while the prior password fails. Delete the dependency-free issuance: fresh authentication fails and owner data remains; physical role absence is not required by the chosen retirement contract.
- Existing owner/active roles and reserved-role refusal remain correct. Shared alias, owned-object, slow/error and issuance controls pass through t002/t005 without claiming those unexercised cases were live observations of this hunt.
- Required checks and the live replay are recorded. Delete the owned database, verify REST404 and no owned cluster resources, and revoke this QA session before marking done.

Already-connected sessions, private-only endpoints, pooler/replica access and owned-object cases were not live-tested by this finding; their limits/controls are explicit in [finding.md](finding.md). No production paid configuration is authorized.

## Source + Goal linkage

- **Source:** continuous qa-find-bugs, 2026-10-07 sweep15, muse.env, workspace bex; [researched finding with complete API/SQL evidence](finding.md). Research main f7c8d338e; two granted-role deletes reproduced continued fresh authentication, including a read at +172.714s. Fixture and session already cleaned up.
- **Goal linkage:** ADR008 reliable hosting and ADR009 managed Postgres access; ADR085's issuance/revocation invariant must also hold when SQL dependencies prevent DROP ROLE.
- **Expected outcome:** retired credentials cannot authenticate; the user can revoke access without destroying tables or first discovering/removing hidden PostgreSQL dependencies.
- **Why now:** successful deletion hides a still-valid data-access credential from every active-user list. This affects ordinary granted roles, not only reserved identifiers or stranded Secrets.
- **Sizing:** ~2h50 across retirement/acceptance, the four-alias/shared-role family audit, real SQL regression controls, parity, simplification and live closeout.
- **Render parity:** included (t003), because all delete surfaces describe tenant-facing credential semantics. Render documents deactivation to preserve original-user objects; bex's additional-role/default-owner model is retained and its accepted asynchronous intent is documented explicitly.
- **Scope:** globally retire tombstoned additional Postgres logins through the existing CNPG projection. No DROP OWNED/CASCADE, new control-plane DB dependency, Key Value/App credential redesign or default-user rotation feature.
- **Dedupe:** no open/done item covers dependency-blocked deletion. ADR085's generation-specific issuance control passed; this is its unhandled dependency case, not the old Secret-adoption regression. w4/m170 reserved-role wedging remains separate.
