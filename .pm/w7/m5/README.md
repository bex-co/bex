# w7 · m5 — Managed Key Value network access controls (ipAllowList parity)

**Worker:** worker7 **Goal:** A managed Key Value (Valkey) instance's public endpoint can be scoped to an allowlist of source CIDRs — Render's `ipAllowList` "Networking" control, which managed Postgres already has and Key Value doesn't — so a publicly-exposed Valkey is no longer password-only-open, closing a documented parity + security asymmetry. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Add `spec.ipAllowList` to the `KeyValue` CRD (mirror `Database`); regenerate manifests + deepcopy       | 30m | —          |
| t002 | Operator: project `ipAllowList` onto the Valkey Traefik TCP/SNI route (reuse the Postgres pattern)      | 45m | t001       |
| t003 | Surface `ipAllowList` on Key Value create/update + read-back across REST · GraphQL · MCP                | 45m | t002       |
| t004 | Dashboard: Key Value Networking section (allowlist editor) mirroring the managed-Postgres pattern       | 45m | t003       |
| t005 | Render parity — `ipAllowList` shape/semantics consistent across REST · GraphQL · MCP + dashboard, vs Render's KV Networking | 30m | t004 |
| t006 | Simplify — `/simplify` over the code this milestone changed                                             | 20m | t005       |
| t007 | Test coverage — meaningful tests for allowlist projection + surface round-trip                          | 30m | t005       |
| t008 | Closeout — DoD verified, milestone moved to `done/`                                                     | 15m | t007       |

## Definition of done

A `KeyValue` with a non-empty `spec.ipAllowList` and `Public: true` accepts external TCP connections only from the listed CIDRs (enforced on the Valkey Traefik SNI route, the same mechanism managed Postgres uses); the field round-trips on REST, GraphQL, and MCP create/update + read, and is editable in a dashboard Key Value Networking section; an internal-only KV (no public route) is unaffected; an empty list preserves prior behavior. Parity with Render's KV Networking allowlist is checked across the three adapters.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w7` (2026-07-11). Verified 2026-07-11: `lego/types/v1alpha1/database_types.go:66-71` already has `IPAllowList` (projected via `lego/backend/internal/postgres/access.go` onto a Traefik TCP `ipAllowList` middleware), but `lego/types/v1alpha1/keyvalue_types.go` has **no** such field; `docs/render-parity.md` Key Value row records `ipAllowList` as "omitted, not faked (the CR can't back them yet)" and "no per-instance IP-allowlist UI (Render's Networking section)".
- **Goal linkage:** GOAL.md V0 #7 (security review) + pillar 1 (Render-compatible API — Render's managed Key Value exposes a Networking IP-allowlist).
- **Expected outcome:** a publicly-exposed managed Valkey can be restricted to known source CIDRs instead of relying on the minted password alone; Key Value reaches managed Postgres's access-control parity, and the parity ledger's KV `ipAllowList` gap closes.
- **Why now:** the only network gate on a public KV today is its password (brute-forceable); the allowlist mechanism already exists for Postgres, so this is low-risk pattern-reuse that removes a documented parity/security asymmetry before public signup.
- **Render parity: included** — this adds a create/update field on REST/GraphQL/MCP + a dashboard Networking section, so t005 checks the field shape/semantics across the three adapters and against Render's KV Networking control. `maxmemoryPolicy`/`persistenceMode` (the KV row's other two omitted fields) are deliberately **out of scope** here — config, not security; they belong to a managed-data milestone, not this hardening one.
