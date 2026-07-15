# w4 · m24 — ipAllowList descriptions: persist what we accept

**Worker:** worker4 **Goal:** Delete the "accepted but dropped" subset RC12/RC14 recorded: `description` on ipAllowList entries persists and returns end to end for managed Postgres, Key Value, and Environments — across REST/GraphQL/MCP, the dashboard editors, and the CLI round-trip. **Status:** done (2026-07-15)

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | CRD: `Database`/`KeyValue` ipAllowList becomes `[{cidr, description}]` with backward-compatible conversion from the string list; operator enforcement reads cidr only — **DONE** | 60m | —          |
| t002 | Backend: persist + return descriptions on every verb for both datastores + Environments' store entries; coordinate with `w4/m23/t004`'s wire work — **DONE** | 45m | t001       |
| t003 | Dashboard editors show/edit descriptions; `cli-compat` postgres/keyvalue legs round-trip a non-empty description — **DONE** | 30m | t002       |
| t004 | Render parity — three-surface consistency; delete the "accepted but dropped" annotations from checklist/ledger with evidence — **DONE** | 20m | t003       |
| t005 | Simplify — `/simplify` over the code this milestone changed — **DONE** | 20m | t004       |
| t006 | Test coverage — round-trip + legacy-string-list conversion + description never influences enforcement — **DONE** | 30m | t004       |
| t007 | Closeout — DoD met → move milestone to `done/` — **DONE** | 10m | t006       |

## Resolution (2026-07-15) — done

Shipped end to end. CRD: `Database`/`KeyValue` `spec.ipAllowList` is `[{cidr, description}]` (`v1alpha1.IPAllowEntry`, union-decodes legacy bare strings; field Schemaless so both serializations validate); the operator renders middleware from CIDRs only (envtest pins legacy-shape identity + description-neutrality). Backend: shared `core.IPAllowListEntry` + conversions; descriptions persist on create/get/update/list across REST (Render `{cidrBlock, description}` wire), GraphQL (`ipAllowListEntries` fields + `entries` args extending the product-neutral string lists), and MCP (entries args/results) for Postgres, Key Value, and Environments (store column `text[]`→jsonb, migration 0034; legacy rows read back with empty descriptions; ACL fan-out carries descriptions). Blueprint `{source, description}` entries persist too. Dashboard editors gained the description column/input. Verified live via the unmodified official CLI against dev-4 (create-seeded and update-replaced descriptions came back on get/list); `scripts/cli-compat.sh verify` fully green with new legs asserting the description RETURNS (plus a typed-id fix to the pre-existing raw-metadata legs). RC12's "accepted but dropped" annotation and every "accepted and discarded" echo deleted with evidence (checklist, ADR009/ADR018/ADR021/ADR032, code comments). Follow-up filed: `w4/019` (one-shot CR normalization to retire the Schemaless shim).

## Definition of done

A `{cidrBlock: "10.0.0.0/8", description: "office"}` entry survives create → get → update → list unchanged on Postgres, Key Value, and Environments across all three API surfaces and displays in the dashboard; a pre-existing CR with the legacy string-list shape reads back with empty descriptions and keeps enforcing; the RC12/RC14 "accepted but dropped" annotations in `docs/cli-compatibility-checklist.md` are deleted with a verified CLI round-trip as evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 10 (2026-07-15) — RC12 (Postgres) and RC14 (Key Value) in `docs/cli-compatibility-checklist.md` both record "bex still only persists the CIDR — an incoming `description` is accepted but dropped, a documented subset"; Environments' entries are bare strings (`w4/m23/t004` aligns the wire shape only). Continues w4's payload-parity thread (m20 → m23).
- **Goal linkage:** Render parity (pillar 1) — Render's `{cidrBlock, description}` objects are round-trip fields, not write-only.
- **Expected outcome:** allow-list entries keep the operator-facing label a human gave them; three documented subsets close at once.
- **Why now:** three fresh closeouts recorded the same subset; the CLI verify script re-asserts it on every run, so closing it flips a standing ◐ into checked-forever ✅.
- **Render parity closing task: included** (t004) — REST/GraphQL/MCP + dashboard shapes change.
