# w8 · m80 — Resolve omitted Git branches in Blueprint services

**Worker:** worker8 **Goal:** independently sourced Blueprint services build their repository's actual default when branch is omitted. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Resolve own-repository defaults before Blueprint planning | 50m | — |
| [t002](t002.md) | Audit shared callers and source omission semantics | 30m | t001 |
| [t003](t003.md) | Render parity | 20m | t002 |
| [t004](t004.md) | Simplify | 10m | t003 |
| [t005](t005.md) | Test coverage | 35m | t003 |
| [t006](t006.md) | Closeout | 20m | t004, t005 |

Six tasks,165m total.

## Definition of done

- Fresh permitted Free fixtures created through both native clients accept the exact manifest through native validation. The first accepted omitted-branch Git deployment selects its independently verified repo default and reaches Live/HTTP200 with exact commit/package/hash and harmless authored literals, without a branch correction or install workaround. API-only sync remains labeled separately from native validation.
- Real prepare/plan/apply/canonical-write tests cover new/existing image-origin × root/nested × omitted/explicit branch and meaningful neighboring source/default/no-op/rollback/error rules. Preview and apply agree; direct manual source edits and reviewed-file provenance are preserved.
- REST/GraphQL/MCP/dashboard behavior is coherent and required checks pass. Measured projector convergence and identical reapply preserve source and child deployments. Unexercised variants and any Render divergence are explicit.
- Owned services, child deployments and metadata are absent, original baselines remain and quota termination is0. Closeout moves all completed tasks and the milestone through /pm; the current diagnostic recoveries do not satisfy this gate.

## Source + Goal linkage

- **Source:** [functional CLI QA sweep344](evidence/cli-sweep344/README.md), [producer/defaulting diagnosis](finding.md); follow-up to m76 direct-create and m78 explicit-source acceptance.
- **Goal linkage:** ADR008 reliable hosting, ADR018 Render-compatible source selection and ADR049 authored Blueprint intent.
- **Expected outcome:** an independently declared Git repo builds its real default without requiring a redundant branch field; canonical state, first commit and build input agree.
- **Why now:** both native-born hosted fixtures fail their first Git clone on main; changing only branch to master makes both healthy. Four actual source-path omission cases also fail while four explicit controls pass. Existing direct-create and explicit-source fixes bypass this distinct defaulting path.
- **Render parity included:** the source decision is user-facing and shared by REST, GraphQL, MCP and dashboard consumers. [m76](../done/m76/README.md) and [m78](../done/m78/README.md) remain completed within their accepted scopes.
