# w2 · m261 — Blueprint create with `existingResources: create_new`

**Worker:** worker1 **Goal:** a Blueprint created with Render's `create_new` mode manages its own suffixed copy of every resource and leaves same-named existing resources untouched. **Status:** done

## Tasks (in order)

| id   | title                                                                              | est | depends_on |
| ---- | ---------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Accept `create_new` at create: mint and store a Render-style suffix — **DONE**                | 30m | —          |
| t002 | Suffix transform: rename every managed resource and every reference to it — **DONE**          | 1h  | t001       |
| t003 | Apply the transform on every plan, approve, sync and auto-sync of the Blueprint — **DONE**    | 45m | t002       |
| t004 | ADR049 § Blueprint onboarding: drop the `create_new` divergence — **DONE**                    | 15m | t003       |
| t005 | Render parity: REST / GraphQL / MCP create agree; compare with Render CLI v2.35 — **DONE**    | 20m | t004       |
| t006 | Simplify — **DONE**                                                                           | 20m | t005       |
| t007 | Test coverage — **DONE**                                                                      | 30m | t005       |
| t008 | Closeout — **DONE**                                                                           | 10m | t007       |

## Definition of done

Creating a `create_new` Blueprint in a workspace that already has `api` from another Blueprint plans `api-<suffix>` as a create and leaves `api` untouched; approving creates it; a later sync of the same Blueprint updates the suffixed set (never the unsuffixed one); references (`fromService`, `fromDatabase`, `fromGroup`) resolve to the suffixed resources; `requiredManualEnvVars` name the suffixed services; `BLUEPRINT_EXISTING_RESOURCES_UNSUPPORTED` is gone.

## Source + Goal linkage

- **Source:** w2/069 (promoted 2026-11-03 during `/loopx w2`: the transform spans the compiler, references and every sync path — more than an hour across several tasks).
- **Goal linkage:** Render parity first (ADR008) — Render CLI v2.35's `ea blueprints create` sends `existingResources`; w2/m256 shipped only `adopt`.
- **Expected outcome:** Render clients can onboard a second copy of a Blueprint beside an existing stack.
- **Why now:** m256's onboarding is live; `create_new` is its one refused option, and the columns (migration 0185) already exist.
- Render parity included: the change is on the REST/GraphQL/MCP Blueprint create surface. No dashboard surface sends `existingResources`.

## Closeout (2026-11-03)

Done. `create_new` mints a 4-character base36 suffix in `store.CreatePlannedBlueprint`. `apps/blueprint_suffix.go` renames every declared service, Key Value, database and env group (root, `ungrouped`, project environments) and every reference to a declared one, on the decoded source before normalization, through `compileStack` and `blueprintValidationFor`. Every Blueprint store read now carries `existing_resources`/`resource_suffix`, and the legacy connect path resets to adopt. Evidence: `TestBlueprintOnboardingCreateNewManagesASuffixedCopy` (a create_new Blueprint beside an adopt one plans, approves and re-plans the suffixed copy, leaving `api` untouched), `TestSuffixRenamesEveryResourceAndReference`, `TestCreateNewBlueprintSuffixPG`. Render parity: REST/GraphQL/MCP all route to `CreatePlannedBlueprint`; the MCP parameter description now names both modes. Simplify review fixed a missed `ungrouped` container and stopped suffixing references to undeclared (existing) resources. The unmodified-CLI walk stays on ADR018's onboarding row.
