# w2 · m261 — Blueprint create with `existingResources: create_new`

**Worker:** worker1 **Goal:** a Blueprint created with Render's `create_new` mode manages its own suffixed copy of every resource and leaves same-named existing resources untouched. **Status:** todo

## Tasks (in order)

| id   | title                                                                              | est | depends_on |
| ---- | ---------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Accept `create_new` at create: mint and store a Render-style suffix                | 30m | —          |
| t002 | Suffix transform: rename every managed resource and every reference to it          | 1h  | t001       |
| t003 | Apply the transform on every plan, approve, sync and auto-sync of the Blueprint    | 45m | t002       |
| t004 | ADR049 § Blueprint onboarding: drop the `create_new` divergence                    | 15m | t003       |
| t005 | Render parity: REST / GraphQL / MCP create agree; compare with Render CLI v2.35    | 20m | t004       |
| t006 | Simplify                                                                           | 20m | t005       |
| t007 | Test coverage                                                                      | 30m | t005       |
| t008 | Closeout                                                                           | 10m | t007       |

## Definition of done

Creating a `create_new` Blueprint in a workspace that already has `api` from another Blueprint plans `api-<suffix>` as a create and leaves `api` untouched; approving creates it; a later sync of the same Blueprint updates the suffixed set (never the unsuffixed one); references (`fromService`, `fromDatabase`, `fromGroup`) resolve to the suffixed resources; `requiredManualEnvVars` name the suffixed services; `BLUEPRINT_EXISTING_RESOURCES_UNSUPPORTED` is gone.

## Source + Goal linkage

- **Source:** w2/069 (promoted 2026-11-03 during `/loopx w2`: the transform spans the compiler, references and every sync path — more than an hour across several tasks).
- **Goal linkage:** Render parity first (ADR008) — Render CLI v2.35's `ea blueprints create` sends `existingResources`; w2/m256 shipped only `adopt`.
- **Expected outcome:** Render clients can onboard a second copy of a Blueprint beside an existing stack.
- **Why now:** m256's onboarding is live; `create_new` is its one refused option, and the columns (migration 0185) already exist.
- Render parity included: the change is on the REST/GraphQL/MCP Blueprint create surface. No dashboard surface sends `existingResources`.
