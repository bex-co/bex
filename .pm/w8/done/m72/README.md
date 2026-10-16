# w8 · m72 — Provide Poetry in native Python builds and runtime

**Worker:** worker8 **Goal:** a native Python service can execute its documented Poetry toolchain with the selected compatible version during build and in the running image. **Status:** done — all seven tasks; deployed acceptance and exact cleanup in sweep341

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Define Poetry versions and Python compatibility | 30m | — — **DONE 2026-10-15** |
| [t002](done/t002.md) | Provide Poetry before build and retain it at runtime | 45m | t001 — **DONE 2026-10-15** |
| [t003](done/t003.md) | Verify shared consumers, tool isolation and cache inputs | 25m | t002 — **DONE 2026-10-15** |
| [t004](done/t004.md) | Render parity | 20m | t003 — **DONE 2026-10-15** |
| [t005](done/t005.md) | Simplify | 10m | t004 — **DONE 2026-10-15** |
| [t006](done/t006.md) | Test coverage | 40m | t004 — **DONE 2026-10-15** |
| [t007](done/t007.md) | Closeout — **DONE 2026-10-16** | 20m | t005, t006 |

## Definition of done

- Fresh published Bex and unchanged pinned Render clients create Free Python services using the finding's compatible Poetry setting and authored build. Both reach Live; actual build/runtime output identify the resolved Poetry/Python versions and marker. A Python-only control and an omitted Poetry-override case retain their documented behavior.
- Maintained execution regressions fail before the repair and pass after for the generated build/runtime artifact, primary Python coexistence, default/override/unsupported combinations, application dependency isolation, shared consumers and selection/cache transitions. Appropriate module and reviewed-toolchain checks pass.
- Existing REST/GraphQL/MCP/Blueprint/dashboard env settings drive the same native behavior with accurate version/error reporting; unchanged pinned wire is preserved. Finish and explicitly qualify other primary-version/consumer variants without claiming unexercised live results.
- Clean only exact-owned fixtures/deployments, preserve baseline/quota, and complete every task plus hosted acceptance before moving to done.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs-cli` w8/sweep307,2026-10-15 UTC. [Finding](finding.md) · [complete evidence](evidence/cli-sweep307/README.md).
- **Goal linkage:** ADR008 reliable Render-alternative hosting, ADR018 native runtime parity and ADR060 D7 reviewed native-toolchain selection.
- **Expected outcome:** Python and Poetry version settings select an available compatible toolchain in both execution phases.
- **Why now:** both unchanged native requests preserve valid settings yet fail with missing Poetry/exit127; working Python controls confirm3.12.15, both env values and Poetry ENOENT at build/runtime. Current assembly never installs it.
- **Sizing:**190m/seven tasks. Version compatibility, isolated tool provisioning, shared/cache controls and actual hosted acceptance exceed a sub-hour note.
- **Render parity included:** execution behind public configuration surfaces changes; official Poetry inclusion/version semantics are the target. Independent Render production is untested.

The original307 evidence establishes the historical failure. Implementation314d7276d and the deployed acceptance below complete this repair; QA supplies no product implementation or platform deployment.

## Closed 2026-10-16 — sweep341

[Complete acceptance and cleanup](evidence/cli-sweep341/README.md): both native first builds/runtime execute Poetry1.8.5 with exact Python3.12/marker and JavaScript/pip controls; application Python cannot import Poetry. A third omitted-setting/Python-only control confirms default2.1.3 without losing primary behavior. Nine shared API reads and five maintained tests/18 named events including actual generated-image execution pass. Three parents/three children are fully removed; baseline/quota and temporary state pass. Other version pairs/browser/API creation remain explicit live limits. [m77](../m77/README.md) had its Python half accepted at sweep341; native Ruby acceptance in sweep342 now completes it.
