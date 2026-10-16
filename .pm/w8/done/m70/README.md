# w8 · m70 — Make Blueprint source switches deploy their declared artifact

**Worker:** worker8 **Goal:** an accepted image-to-Git Blueprint change builds the declared Git source before reporting Live. **Status:** done — source-coupling fix eebd8ce27, inline sweep322 and same-source persistent sweep323 first-artifact/no-op acceptance; different-repo context remains m78

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Apply coupled source intent before persistence — **DONE** | 45m | — |
| t002 | Audit strategy, planning and source-switch neighbors — **DONE** | 35m | t001 |
| t003 | Render parity — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003 |
| t006 | Closeout — **DONE** | 20m | t004, t005 |

## Definition of done

- Both actual native clients create fresh own Free image-born controls. API-only inline and owned persistent provided-manifest Blueprint changes to Git/native source then deploy the correct commit, actual built package/content and exact runtime literal before Live acceptance. No manual recovery deploy is needed. Repeat after projector convergence and prove identical apply opens no deployment.
- Actual shared compile/applier/persistence/consumer regressions fail before the repair and pass after for both source-switch directions, native/Dockerfile targets, declared runtime, omitted/unchanged fields, branch defaults and canonical row failures. Preserve valid rollback overrides and their restart/deploy roles; do not globally ban Repo+Image.
- Preview/action-plan and REST/GraphQL/MCP/UI readback describe the same effective source and errors. Current pinned CLI/native contracts and normal source-save behavior stay coherent. Required affected checks pass.
- Clean only exact-owned services, enumerated deployment children and Blueprint; retain baseline inventories and final quota. Complete every task before moving to done.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs-cli` w8/sweep294,2026-10-15; discovered while qualifying m66's persistent live gate. [Finding](finding.md), [complete evidence](evidence/cli-sweep294/README.md).
- **Goal linkage:** ADR008 reliable hosting, ADR049 authored Blueprint intent and ADR018 truthful CLI/deploy status.
- **Expected outcome:** Live at a declared Git commit means that Git artifact is actually serving, instead of the former image.
- **Why now:** both inline and persistent production applies report Live/new commit but serve the old image; native manual deployment recovers both. Current source reproduces three transitions and separates this merge defect from m66's already repaired canonical writer.
- **Sizing:**175m across six tasks; coupled presence/strategy semantics, rollback/default/planner neighbors and runtime acceptance exceed a sub-hour note.
- **Render parity included:** this is a shared Blueprint/config and deployment-status surface. Compare primary field meanings and the exact pin honestly; no Render-production migration is claimed.

The original finding and private candidate are historical diagnosis. The maintained implementation is eebd8ce27; the hosted acceptance below closes its source-coupling scope. QA changed no product source, performed no external submission and inferred no serving SHA.

## Historical hosted replay 2026-10-15 — sweep322, partial acceptance

[Complete native/API/source/cleanup evidence](../../m78/evidence/cli-sweep322/README.md). The first inline accepted Git deployment builds and serves the exact Express commit/package/literal with no native recovery. Published Bex, checkout Bex and unchanged Render inventory/deploy reads agree; delayed identical apply adds no deployment and runtime stays correct. Six maintained source tests (11 named results) pass.

The fresh persistent Blueprint has a different containing repo, and provided-manifest sync replaces its explicit service repo/branch with that containing source, then build-fails. This is [m78](../../m78/README.md), a compile-context issue distinct from the repaired old-image fold. The stored manifest retains the authored source. The persistent gate is not declared passed, no old source task is reopened and t006 stays blocked. Two services/five deployment children plus the owned Blueprint are absent; original services/Blueprint and quota are preserved. No serving SHA is inferred from the desired image pin.

## Closed 2026-10-15 — sweep323

[Complete persistent acceptance](evidence/cli-sweep323/README.md): the first API-accepted Git deployment serves the exact package/hash/commit/literal, all three native clients agree, REST/GraphQL/MCP source and command readback match, and delayed identical sync creates no deployment. The later runtime/history remains correct without any recovery deploy. All own resources and three deployment children are gone; original services/Blueprint remain and quota termination is0. Together with sweep322's inline pass and the completed maintained source work, this closes all six m70 tasks. [m78](../../m78/README.md) remains open for a different containing/service repo or branch; broader Blueprint source contexts are not certified by this closeout.
