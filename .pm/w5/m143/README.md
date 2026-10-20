# w5 · m143 — Blueprints from Render's own templates validate on bex

**Worker:** worker5 **Goal:** the `render.yaml` files Render's own Deploy-to-Render templates ship validate and apply on bex unchanged, with every remaining refusal a recorded deliberate divergence. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin Render's accepted forms for unquoted booleans, capitalized plans and empty `envVars` | 30m | — |
| t002 | Coerce YAML booleans on env-value paths before schema validation | 45m | t001 |
| t003 | Case-fold plan enums and treat a null `envVars` as empty | 30m | t001 |
| t004 | Vendor the render-examples corpus as a fixture test | 1h | t002, t003 |
| t005 | dev-5: apply an unmodified template end to end | 30m | t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 45m | t006, t007 |
| t009 | Closeout | 10m | t008 |

Total: **9 tasks, about 5h**.

## Definition of done

- The corpus test passes, with every remaining refusal on the deliberate list.
- The nine repos blocked only by this gap validate.
- An unmodified template applies on dev-5 (recorded).

## Source + Goal linkage

- **Source:** third `/pm-brainstorm for w5` (2026-10-20), re-offered as the parity-only set and chosen by the user the same day. Of 170 `render.yaml` files from `render-examples`, 65 passed `ValidateBlueprint`; after the deliberate refusals, the main remaining class is inputs Render's published schema forbids but its templates use (13 repos use unquoted booleans). w8/m53 only fenced these off (`.pm/w8/done/m53/README.md:22`).
- **Goal linkage:** ADR049 `render.yaml` parity; migrating from Render.
- **Expected outcome:** templates people deploy with a Render button deploy on bex unchanged.
- **Why now:** this is the most common migration path, and the corpus test catches the rest of the class.
- **Render parity:** included: Blueprint validate/preview/apply on REST, GraphQL, MCP and the dashboard.
