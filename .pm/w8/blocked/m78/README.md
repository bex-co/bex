# w8 · m78 — Respect per-service Git sources in persistent Blueprints

**Worker:** worker8 **Goal:** a persistent Blueprint sync builds each service's explicitly declared repo/branch instead of replacing them with the containing Blueprint source. **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Separate containing source defaults from explicit deploy overrides | 55m | — — **DONE 2026-10-15** |
| [t002](done/t002.md) | Audit shared compiler callers and source binding | 35m | t001 — **DONE 2026-10-15** |
| [t003](done/t003.md) | Render parity | 25m | t001, t002 — **DONE 2026-10-15** |
| [t004](done/t004.md) | Simplify | 15m | t003 — **DONE 2026-10-15** |
| t005 | Test coverage | 45m | t003 |
| t006 | Closeout | 20m | t004, t005 |

## Definition of done

- A fresh own persistent provided-manifest sync with a different containing repo builds the service's explicit repo and branch, serves the exact expected commit/package/hash/literals on its first accepted deployment, survives projector convergence and creates no deploy on identical reapply. Both native validation clients accept the document; no native CLI apply coverage is invented.
- Maintained real prepare/create/preview/apply/canonical-row regressions fail before and pass after for repo-only, branch-only, both, and root/nested explicit declarations. Omitted source inheritance, image source, deliberate direct-deploy overrides, reviewed-file provenance and legitimate rollback behavior remain correct. Different-repo omitted-branch behavior stays coordinated with m76 rather than falsely certified here.
- Shared compiler callers and REST/GraphQL/MCP/dashboard readback are coherent; m70 source coupling and m66 canonical persistence are preserved. Required checks pass, with unsupported/untested neighbors explicit.
- Exact-owned parents, children and Blueprint are absent, baseline inventories remain and quota termination is0. All statuses/moves match observable completion.

## Source + Goal linkage

- **Source:** [functional CLI QA sweep322](evidence/cli-sweep322/README.md), [source-context diagnosis](finding.md), discovered during m70's hosted replay.
- **Goal linkage:** ADR008 reliable hosting, ADR018 Render-compatible service sources and ADR049 authored Blueprint intent.
- **Expected outcome:** each accepted service declaration selects the source the user wrote, while the containing file supplies only applicable defaults.
- **Why now:** one actual persistent sync replaces Express/master with Node/main and fails its build, whereas the inline exact-source control succeeds; four actual preparation probes reproduce the priority error. The old stale-image repair faithfully applies this already-wrong prepared tuple.
- **Render parity included:** this changes a user-facing Blueprint/config path shared by REST, GraphQL, MCP and the dashboard. Six tasks total195m.
