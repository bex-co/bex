# w9 · m6 — Key Value stable `red-` id + rename

**Worker:** worker9 **Goal:** Key Value stores get a stable Render-shaped `red-…` id and a mutable display name, mirroring what w9/m3 shipped for Postgres — closing the last documented name-as-id datastore deviation. **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Capture Render's Key Value id/rename contract (OpenAPI + live docs)            | 30m | —          |
| t002 | Mint `id.Kind` keyvalue (`red-` prefix) + CR identity + backfill migration     | 45m | t001       |
| t003 | Store + projector: stable id column, mutable name                              | 30m | t002       |
| t004 | REST/GraphQL/MCP: route by `red-` id + rename verb                             | 45m | t003       |
| t005 | Dashboard: rename control + id display                                         | 30m | t004       |
| t006 | Roll dev-N + prod; official-CLI `red-` routing verify leg                      | 45m | t005       |
| t007 | Render parity                                                                  | 30m | t006       |
| t008 | Simplify                                                                       | 30m | t007       |
| t009 | Test coverage                                                                  | 45m | t007       |
| t010 | Closeout                                                                       | 15m | t009       |

## Definition of done

A Key Value store carries a stable `red-…` id that survives rename; the unmodified official Render CLI routes to it by id; `docs/cli-compatibility-checklist.md` RC5 flips to ✅ and `docs/ADR018-render-parity.md:218`'s `red-` routing divergence is recorded closed with evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — closeout-residual sweep (`w9/done/m3/README.md:41` left KeyValue explicitly out of scope; checklist RC5 calls it "the one documented name-as-id datastore deviation").
- **Goal linkage:** Render parity across all five surfaces, incl. the official CLI (fifth surface, w9/m2 charter).
- **Expected outcome:** the last name-as-id datastore deviation is gone; CLI `red-` client-side routing works against bex.
- **Why now:** the identical Postgres migration (w9/m3) is freshly shipped — pattern, scripts, and rollout playbook are warm; w9's other open milestones are small. Render parity closing task included — this touches REST/GraphQL/MCP/UI.
