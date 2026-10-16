# w8 · m77 — Supply JavaScript tools in native Ruby and Python builds and runtime

**Worker:** worker8 **Goal:** native Ruby and Python services can run the documented JavaScript asset tools during build and runtime while preserving their primary toolchains. **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | [Supply JavaScript tools in native Ruby and Python images](done/t001.md) | 60m | — — **DONE 2026-10-15** |
| [t002](done/t002.md) | [Audit shared image consumers and cache transitions](done/t002.md) | 45m | t001 — **DONE 2026-10-15** |
| [t003](done/t003.md) | [Render parity](done/t003.md) | 30m | t001, t002 — **DONE 2026-10-15** |
| [t004](done/t004.md) | [Simplify](done/t004.md) | 15m | t003 — **DONE 2026-10-15** |
| [t005](done/t005.md) | [Test coverage](done/t005.md) | 55m | t003 — **DONE 2026-10-15** |
| t006 | [Closeout](t006.md) | 25m | t004, t005 |

## Definition of done

- The same first-build and runtime JavaScript-tool acceptance also passes on explicit Python3.12 through both native clients, preserving Python/pip/Poetry, existing Bun selection and the primary runtime. Source matrix/caller coverage and untested live variants are explicit.

- Both fresh explicit Free Ruby services, created through published Bex and unchanged Render pin, run Node/npm/Yarn in the authored build and deployed runtime without user setup, reach Live/HTTP200 with exact markers, and retain the expected Ruby/Bundler/Gem controls. Preserve the original two failures and controls.
- Real generated image execution for default and all supported Ruby lines3.3/3.4/4.0 proves the secondary tools, primary version selection, command/env/PATH/file behavior and stable/changed cache identities. Source text alone is insufficient.
- Shared caller review and REST/GraphQL/MCP/Blueprint/dashboard parity are complete or precisely qualified; no CLI fork, unrelated tool-bundle expansion or accidental old milestone closeout occurs. Coordinate m73 pnpm work.
- Exact-owned service and deployment children are absent, former URLs return404, baseline services remain and quota termination is0. Required checks pass, evidence is linked, and all board statuses/moves match the real outcome.

## Source + Goal linkage

- **Source:** [functional CLI QA sweep321](evidence/cli-sweep321/README.md), [root-cause research](finding.md), and Render's native Ruby tool contract.
- **Goal linkage:** ADR008 usable hosting and ADR018 Render-compatible runtime behavior; normal Ruby frontend asset builds must work through the unchanged CLI.
- **Expected outcome:** Node/npm/Yarn are available when a Ruby build or start command needs them, while the selected Ruby runtime remains intact.
- **Why now:** both actual native clients reproduce the failure, and both Live Ruby-only controls isolate the missing tools. Primary Node pnpm and Ruby PATH/version fixes do not cover it. The shared image path needs coordinated assembly and cache review before closeout.
- **Render parity included:** a user-facing native Ruby build/runtime correction affects the common REST/GraphQL/MCP/Blueprint/dashboard service surface.

## Scope extension — sweep328

[Two-client Python evidence](evidence/cli-sweep328/README.md) confirms the same major companion-tool gap: two first builds fail on missing Node/exit127; both Python-only controls are Live with all four JavaScript tools absent in build/runtime. An actual current-source Python3.12 build still fails, while six generated primary-version artifacts are retained. Both parents/four children are cleaned. This extends m77 without another issue. The plan is six TODO tasks,230m; no product fix or ship is included.
