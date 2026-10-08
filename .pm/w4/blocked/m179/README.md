# w4 · m179 — Revoke deleted Postgres logins even when objects reference the role

**Worker:** worker4 **Goal:** deleting an additional Postgres credential prevents new authentication while preserving tenant objects, even when a grant or ownership dependency would block DROP ROLE. **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Retire deleted Postgres logins without dropping tenant objects — **DONE** | 45m | — |
| t002 | Audit all retirement aliases and preserve passing role controls — **DONE** | 35m | w4/m179/t001 |
| t003 | Render parity — **DONE** | 20m | w4/m179/t001, w4/m179/t002 |
| t004 | Simplify — **DONE** | 10m | w4/m179/t003 |
| t005 | Test coverage for granted logins and real SQL retirement — **DONE** | 45m | w4/m179/t003, w4/m179/t004 |
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

## Progress (2026-10-08)

t001–t005 done. The operator projects a `spec.deletedUsers` tombstone as `ensure: present, login: false, disablePassword: true` (`managedRoles`, `database_controller.go`), so CNPG issues `ALTER ROLE … NOLOGIN PASSWORD NULL` instead of the `DROP ROLE` that PostgreSQL refuses while a grant references the role. Tombstones are durable retirement intent; same-name recreation still removes them. The dashboard toast reads "Revocation requested for {name}" (en/zh). GraphQL `deleteDatabaseUser` and MCP `delete_postgres_user` now describe accepted asynchronous retirement. The REST aliases keep their 204/200 status and body. ADR009 and ADR018 are updated.

Real-CNPG evidence: `scripts/pg-role-retirement-check.sh` on the local CAPD cluster (CNPG **1.27.0**; production runs 1.30.0, whose role code path the finding traced). With `LEGACY=1`, the old `ensure: absent` shape reproduces `cannotReconcile … 1 object in database` while a fresh login still reads the granted table. The new shape passes four checks. A fresh login is refused at once. The role is NOLOGIN with a NULL password, and the grant and owner data survive. Same-name reissue accepts the new password and refuses the old one. A retired role that owns a table keeps the table. A tombstone for a role that never existed creates it as NOLOGIN. Operator `make test`, backend `internal/postgres`, dashboard databases tests (174), `make lint` (4 modules) and mobile codegen drift all pass.

Not verified: pooler/replica and private-only endpoints, and already-established sessions. NOLOGIN blocks only new authentication. Production CNPG 1.30 is unverified until the live replay.
