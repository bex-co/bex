# w5 · m150 — Render templates with free-form resource names or dotted env keys validate

**Worker:** worker5 **Goal:** Render templates whose resource names are free-form or whose env keys are dotted validate and deploy on bex. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A Blueprint name is a display name mapped onto a generated slug id — **DONE** | 1h30m | — |
| t002 | Accept dotted env keys end to end — **DONE** | 1h15m | — |
| t003 | Corpus and template tests hold the line — **DONE** | 45m | t001, t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

Total: **7 tasks, about 5h15m**.

## Definition of done

- pgadmin, gitpod-openvscode-server, vibe-code-demo-apps, postgresql-missing-trigger-index and elasticsearch validate in the corpus run.
- `knownBlueprintGaps` is empty.

## Source + Goal linkage

- **Source:** fifth `/pm-brainstorm for w5` (2026-10-20), all five items chosen by the user the same day after `2163b27c7` made Render parity the top principle (a divergence stands only with ADR018-recorded user sign-off; never park parity on an optional decision); promoted from w5/169 (w5/m143's corpus run), which was parked on a naming decision that the new principle forbids parking on.
- **Goal linkage:** ADR049 render.yaml parity; ADR008.
- **Expected outcome:** Render's own templates deploy unchanged.
- **Why now:** the principle change makes the parked note workable now, with the stricter naming choice left to w5/m118.
- **Render parity:** included: Blueprint validation, REST and dashboard names.

## Closeout (2026-10-21)

- **Names.** A Blueprint resource name that isn't already a resource name maps onto one in the IR (`blueprintResourceSlug`, `lego/backend/internal/apps/blueprint_names.go`). The mapping is stable, and a name over 30 characters gets a short hash.
  - Plan, preview, the resource list, error locations and the duplicate check all see the same name. References by the original name resolve, and two names that share a slug are refused as `BLUEPRINT_DUPLICATE_RESOURCE`.
  - A new service gets the original as its display name, once, so a later dashboard rename sticks. An unusable label leaves the resource name showing rather than failing the sync.
- **Env keys.** They follow the Kubernetes Secret-key rule (letters, digits, `-`, `_`, `.`) on every write path. The shared `config-key-vectors.json`, the dashboard validator and the copy all match. A native build's shell skips names it can't export, and Dockerfile builds include dot-prefixed Secret keys.
- **Corpus.** It went from 83 to 88 of 170 valid; pgadmin, gitpod-openvscode-server, openclaw, vibe-code-demo-apps and postgresql-missing-trigger-index now validate. Both m150 entries are gone from `knownBlueprintGaps`.
- **Tests.** `blueprint_names_test.go` (slug rules, deploying a free-form name with a dotted key and a by-name reference without duplicates on re-sync, collision). Updated vectors and refusal fixtures. The full backend and dashboard suites pass, and lint is clean.
- **Follow-up.** w5/173: free-form names on REST, GraphQL and MCP create, and datastore display names. Render's create takes a free-form name too.
