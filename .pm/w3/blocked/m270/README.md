# w3 · m270 — Blueprints accept buildSources and services[].buildSource, as on Render

**Worker:** worker3 **Goal:** a render.yaml that declares `buildSources` validates, plans and syncs into one build feeding several services, instead of being refused by name **Status:** blocked — t001–t008 done; t009 Closeout waits on w2/m254's production acceptance (its t016) and a live production check of a `buildSources` Blueprint

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Amend ADR049/ADR097 and ADR018 row 223: Blueprint build-source semantics — **DONE** | 1h | — |
| t002 | Compiler: accept buildSources/buildSource, resolve names, validate — **DONE** | 1.5h | t001 |
| t003 | Apply/sync: create, update and link build sources through the m254 core — **DONE** | 2h | t002 |
| t004 | Blueprint export emits buildSource for linked services — **DONE** | 1h | t003 |
| t005 | Dashboard Blueprint preview shows build-source plan rows — **DONE** | 1h | t003 |
| t006 | Render parity — **DONE** | 30m | t004, t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 40m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- A render.yaml declaring `buildSources` plus two services with `buildSource` validates, plans and syncs: one build feeds both services
- Export → import of that workspace plans nothing
- The compiler refusals and `capabilities.json` unsupported entries are gone; ADR018 row 223 reads ✅
- REST, GraphQL and MCP Blueprint validate/plan/sync and the dashboard preview agree

## Source + Goal linkage

- **Source:** `/pm-brainstorm w3` 2026-11-05 (Render-parity-filtered round, item 1); code-verified at `158cb45d0`; ADR018 row 223
- **Goal linkage:** Render parity first (ADR008 § Render parity first); Blueprint (ADR049) IaC parity
- **Expected outcome:** Render Blueprints that use Build Sources deploy on bex unchanged
- **Why now:** w2/m254 shipped Build Sources on REST/GraphQL/MCP, so the refusal's premise is gone and the IaC entry point is the only gap. Code can proceed now; closing waits on w2/m254's production acceptance (t016)
- **Render parity:** included — feature work touching REST/GraphQL/MCP Blueprint surfaces and the dashboard

## Blocked (2026-11-05, loopx w3)

- **Gate:** the milestone's own why-now: closing waits on `w2/m254` closing (its next step is t016, production acceptance with the QA account). Then, with this milestone deployed, a live check on production: validate and sync a render.yaml that declares one git build source and two linked services through the QA account, see one carrier build feed both services, and re-sync as a no-op.
- **Owner / release condition:** w2's worker closes m254; then any worker runs the live check (`scripts/qa-login.sh`) and t009.
- **DoD note:** ADR018's row reads ◐, not ✅: two edge divergences remain and are filed as `w3/050` (buildSources-only projects; non-literal build-time env vars). Closeout should either land 050 first or record the user's acceptance of ◐.

- **2026-11-05 update:** `w3/m275` closed the two residual gaps; ADR018's row now reads ✅, so the DoD note above is resolved. Only the w2/m254 + live production gate remains.
