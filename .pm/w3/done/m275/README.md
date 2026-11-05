# w3 · m275 — Blueprint build sources: buildSources-only projects and every build-time env var form, as on Render

**Worker:** worker3 **Goal:** every Blueprint build source Render's schema admits applies on bex: a project holding only buildSources, and build-time envVars in any envVar form **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A project that declares only buildSources applies — **DONE** | 1h | — |
| t002 | Build-time env vars take every envVar form — **DONE** | 2h | t001 |
| t003 | ADR097 § Blueprints and ADR018 row to the shipped state — **DONE** | 30m | t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- A buildSources-only project validates, plans and syncs, scoping its build sources to that project
- Every envVar form on a build source compiles and applies (generateValue/sync:false seeded once, fromGroup linked, fromDatabase/fromService resolved), and re-syncs as a no-op
- ADR018's Blueprint Build Sources row reads ✅

## Source + Goal linkage

- **Source:** promotes `w3/050` (residuals of w3/m270), 2026-11-05 loopx w3
- **Goal linkage:** Render parity first (ADR008); Blueprint IaC parity (ADR049, ADR097)
- **Expected outcome:** Render Blueprints using build sources apply on bex with no edge refused
- **Why now:** the last two refusals left by w3/m270; small, and the build pipeline already supports every env form
- **Render parity:** included — feature work on the REST/GraphQL/MCP Blueprint surfaces
