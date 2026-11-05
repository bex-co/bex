# w3 · m270 — Blueprints accept buildSources and services[].buildSource, as on Render

**Worker:** worker3 **Goal:** a render.yaml that declares `buildSources` validates, plans and syncs into one build feeding several services, instead of being refused by name **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Amend ADR049/ADR097 and ADR018 row 223: Blueprint build-source semantics | 1h | — |
| t002 | Compiler: accept buildSources/buildSource, resolve names, validate | 1.5h | t001 |
| t003 | Apply/sync: create, update and link build sources through the m254 core | 2h | t002 |
| t004 | Blueprint export emits buildSource for linked services | 1h | t003 |
| t005 | Dashboard Blueprint preview shows build-source plan rows | 1h | t003 |
| t006 | Render parity | 30m | t004, t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 40m | t006 |
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
