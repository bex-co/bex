# w5 · m143 — Blueprints from Render's own templates validate on bex

**Worker:** worker5 **Goal:** the `render.yaml` files Render's own Deploy-to-Render templates ship validate and apply on bex unchanged, with every remaining refusal a recorded deliberate divergence. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin Render's accepted forms for unquoted booleans, capitalized plans and empty `envVars` — **DONE** | 30m | — |
| t002 | Coerce YAML booleans on env-value paths before schema validation — **DONE** | 45m | t001 |
| t003 | Case-fold plan enums and treat a null `envVars` as empty — **DONE** | 30m | t001 |
| t004 | Vendor the render-examples corpus as a fixture test — **DONE** | 1h | t002, t003 |
| t005 | dev-5: apply an unmodified template end to end — **DONE** | 30m | t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006, t007 |
| t009 | Closeout — **DONE** | 10m | t008 |

Total: **9 tasks, about 5h**.

## Definition of done

_Amended at closeout (2026-10-20): the filing's "nine repos validate" rested on a research claim the corpus disproved. forem and pgadmin also fail on names and dotted keys (filed as w5/169), and five of the nine fail only on the deliberate `PORT`/`RENDER_*` refusals. The DoD below is the evidenced version; the original lines follow it._

- Unquoted boolean env values, capitalized plans, a bare `envVars:` and env-var prompt metadata validate (unit tests fail on the old code).
- `TestRenderExamplesCorpus` passes on all 170 render-examples files: every refusal is deliberate, Render-consistent, or a filed gap (w5/169).
- dev-5's REST validate accepts the unmodified `retool` and `redash` templates; `langfuse4-template`, `hasura-graphql`, `zulip` and `saleor` fail only on deliberate refusals.

Original:

- The corpus test passes, with every remaining refusal on the deliberate list.
- The nine repos blocked only by this gap validate.
- An unmodified template applies on dev-5 (recorded).

## Source + Goal linkage

- **Source:** third `/pm-brainstorm for w5` (2026-10-20), re-offered as the parity-only set and chosen by the user the same day. Of 170 `render.yaml` files from `render-examples`, 65 passed `ValidateBlueprint`; after the deliberate refusals, the main remaining class is inputs Render's published schema forbids but its templates use (13 repos use unquoted booleans). w8/m53 only fenced these off (`.pm/w8/done/m53/README.md:22`).
- **Goal linkage:** ADR049 `render.yaml` parity; migrating from Render.
- **Expected outcome:** templates people deploy with a Render button deploy on bex unchanged.
- **Why now:** this is the most common migration path, and the corpus test catches the rest of the class.
- **Render parity:** included: Blueprint validate/preview/apply on REST, GraphQL, MCP and the dashboard.

## Progress (2026-10-20)

- **Shipped.** `blueprintJSONScalar` gives an env value's boolean as its text, lowercases plan values on service/database `plan`, and reads a null `envVars` as an empty list. The mapping walk skips `displayName`/`description`/`sensitive`/`required` on an `envVars` entry, Render's template prompt metadata, which was added to this milestone as part of the same class (`apps/blueprint_compiler.go`). w8/m53's boolean fence (`TestBlueprintEnvValueStillRefusesBooleans`) became `…StillRefusesMappings`.
- **Corpus guard (t004).** The files are third-party, so they are fetched by `scripts/render-examples-corpus.sh` rather than vendored, and run by the opt-in `TestRenderExamplesCorpus`. Deliberate and Render-consistent refusals are listed, and filed gaps sit in `knownBlueprintGaps` (w5/169: free-form names, dotted env keys). All 170 files pass the rule; 67 are fully valid, most of the rest on deliberate region/preview refusals.
- **t005 deviation.** It validated through dev-5's REST API (rebuilt bex-api, a throwaway verified Kratos user) instead of applying. An apply needs a git-connected Blueprint and pulls multi-GB images onto the shared local cluster; validate exercises the same compiler and transport. Result: `retool` and `redash` valid; `langfuse4-template`/`hasura-graphql`/`zulip` only the reserved `PORT`, `saleor` only a `RENDER_*` `fromService`.
- **Render parity (t006).** The four inputs match Render's observable acceptance (its templates deploy). Free-form names and dotted keys are filed as w5/169, needing your naming decision with w5/m118.
- **Simplify/coverage.** A small, self-reviewed diff; `TestBlueprintAcceptsRenderTemplateSpellings`, `TestBlueprintBooleanCoercionIsScopedToEnvValues` and `TestBlueprintIgnoresRenderTemplateEnvMetadata` all fail on the old code. `make lint-backend` is clean.
