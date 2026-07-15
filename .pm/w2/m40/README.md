# w2 · m40 — Blueprint environment-scoped env groups

**Worker:** worker2 **Goal:** Delete the named unsupported error: `projects[].environments[].envVarGroups` — Render's official Blueprint nesting — threads through the existing env-group create/apply path so each group lands with its declared Environment membership, keeping the all-or-nothing preflight. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Thread `environments[].envVarGroups` through Blueprint ingest: group created with Environment membership; all-or-nothing preflight kept | 45m | —          |
| t002 | `/blueprints` validate + sync verbs accept the nesting; the named unsupported error deleted        | 20m | t001       |
| t003 | Fixture e2e: a real `bex.yml` with environment-scoped groups round-trips validate → apply → the environment card shows the group | 30m | t002       |
| t004 | Render parity — nesting vs Render's Blueprint spec; three-surface consistency for the validate/sync verbs | 20m | t003       |
| t005 | Simplify — `/simplify` over the code this milestone changed                                       | 20m | t004       |
| t006 | Test coverage — malformed nesting rejected by name; membership actually recorded; preflight atomicity | 30m | t004       |
| t007 | Closeout — DoD met → move milestone to `done/`                                                    | 10m | t006       |

## Definition of done

The documented unsupported error is gone: a `bex.yml` declaring `projects[].environments[].envVarGroups` passes `validate`, applies atomically, and the created group carries the declared Environment membership (visible via `envGroupIds` on the environment across REST/GraphQL/MCP); malformed nesting still fails preflight with a named error and creates nothing.

## Source + Goal linkage

- **Source:** promotes `w2/010` (filed 2026-07-14/15: "root and `ungrouped.envVarGroups` already work; today an environment-scoped block returns a named unsupported error"). The membership verbs it needs shipped in `w6/m24`.
- **Goal linkage:** Render Blueprint parity (pillar 1, `w1/m24`/`w1/m35` thread) + deploy-from-chat (pillar 4) — agents applying Blueprints shouldn't hit a hole in the official nesting.
- **Expected outcome:** the last named-unsupported branch in Blueprint env-group ingestion closes; Render Blueprints using the standard nesting apply unmodified.
- **Why now:** the blocker (environment membership on env groups) shipped 2026-07-14; the error message itself points here.
- **Render parity closing task: included** (t004) — the Blueprint ingest/validate surface changes.
