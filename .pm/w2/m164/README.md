# w2 · m164 — The disposition contract, enforced: every tenant- and subject-scoped table declares how it dies

**Worker:** worker2 **Goal:** a new table carrying a workspace id or an identity cannot ship without declaring what deletes it — the guard catches it in CI instead of a reviewer remembering ADR086 **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Widen the identity census: catch `owner_subject`, `billing_email` and their kin     | 45m | —          |
| t002 | Add the tenant-scope census: every workspace-keyed table declares cascade, purger, or retention | 1h  | —          |
| t003 | Close the declared gaps the two censuses surface                                    | 1h  | t001, t002 |
| t004 | Give `workspace_creation_attempts` a real disposition (subject + billing email)      | 45m | t001       |
| t005 | ADR086 amendment: the disposition table becomes the census's source of truth         | 40m | t003, t004 |
| t006 | Simplify the code this milestone changed                                            | 30m | t005       |
| t007 | Test coverage: both censuses fail on a seeded violation                              | 40m | t005       |
| t008 | Closeout                                                                            | 15m | t006, t007 |

## Definition of done

- The identity/provenance census (`lego/backend/internal/store/accounts_pg_test.go`) matches columns by **shape**, not by an exact-name list: a new `*_subject`, `*_email`, `*_identity_id`, `created_by`/`actor*`/`caller` column fails the test until it is declared. Proven by seeding such a column in a test migration and watching the test fail.
- A second census enumerates every table with a `tenant_id`/`workspace_id`/`owner_id` column and asserts each one is either (a) FK-cascaded to `tenants`, (b) covered by a registered `WorkspacePurger`, or (c) listed with an explicit retention/no-cleanup rationale. Proven the same way.
- Every gap the two censuses surface on today's schema is either closed or listed with a written reason — no silent entries.
- `workspace_creation_attempts.owner_subject` and `.billing_email` have a disposition: a deleted account's subject and billing address do not survive in terminal attempt rows.
- ADR086's disposition table and the censuses agree, and the ADR says which is authoritative.

## Source + Goal linkage

- **Source:** the 2026-09-16 offboarding research. Findings, all verified against the schema and the cleanup code:
  - The existing census (`accounts_pg_test.go:280-330`) matches an **exact column-name list** (`subject`, `owner_identity_id`, `email`, `invited_by`, `caller`, `created_by`, `actor_id`). `workspace_creation_attempts` (migration 0103, one migration after ADR086's own 0102) carries `owner_subject` and `billing_email` — both evade the census purely by naming, and neither appears in ADR086's disposition table.
  - `store.DeleteTenant` (`internal/store/workspaces.go:133`) is a single `DELETE FROM tenants`, so workspace teardown relies entirely on FK cascades plus the eight registered purgers (apps, postgres, keyvalue, sandbox, secrets, envgroups, registrycreds, billing). Tables with a workspace id and **neither** — `jobs` (0027), `agent_session_dispatches` (0105), `datastore_event_facts`, `datastore_observed_checkpoints`, `service_event_index`, `workspace_creation_attempts` — keep their rows forever. Some of those are deliberate (audit-class: `audit_events`, `ssh_sessions`, `cli_telemetry_events` have retention sweeps); nothing distinguishes deliberate from forgotten.
  - The near-miss that proves the guard is needed: `git_connections` had no cascade for 85 migrations until 0094 added one, and `sandbox_tenant_keys` likewise — both caught late, by hand.
- **Goal linkage:** ADR086 account deletion (its closing rule is literally "Future subject/provenance columns must extend this table and cleanup before shipping" — today nothing enforces it), ADR043 tenant isolation, ADR003 control plane.
- **Expected outcome:** the class of bug that produced `w5/m101` and `w2/m163` — a durable row outliving the thing it belongs to — becomes a CI failure at the migration that introduces it.
- **Why now:** the repo already has the two halves of the answer (an ADR disposition table and one narrow census test); they just do not meet. Doing this while the offboarding work is fresh is far cheaper than rediscovering each gap one incident at a time, and `w2/m162` is live evidence that new subject-bearing tables are still being written (see inbox note `037`).
- **Render parity task omitted:** this milestone changes no REST/GraphQL/MCP/UI surface — it is schema-governance and test infrastructure. The user-visible dispositions it triggers (t004) are internal data handling, not an API shape.
