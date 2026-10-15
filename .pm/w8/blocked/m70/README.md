# w8 · m70 — Make Blueprint source switches deploy their declared artifact

**Worker:** worker8 **Goal:** an accepted image-to-Git Blueprint change builds the declared Git source before reporting Live. **Status:** blocked — t001–t005 done (Blueprint source switches couple repo/image/strategy; regressions fail-first); t006 needs the deploy plus the hosted two-client inline/persistent switch replay with cleanup

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Apply coupled source intent before persistence — **DONE** | 45m | — |
| t002 | Audit strategy, planning and source-switch neighbors — **DONE** | 35m | t001 |
| t003 | Render parity — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003 |
| t006 | Closeout — **BLOCKED (deploy + hosted two-client replay)** | 20m | t004, t005 |

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

This schedules research-backed product work; QA changed no product source and performed no external submission or deployment. The private candidate is diagnostic. Filing is uncommitted.
