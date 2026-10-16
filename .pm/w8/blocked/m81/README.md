# w8 · m81 — Provide uv for native Python lockfile projects

**Worker:** worker8 **Goal:** Python services with uv.lock build and run with the supplied uv tool on the selected Python. **Status:** blocked — t001–t005 done 2026-10-16; t006 needs the operator deployed plus a hosted fresh-fixture replay

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Supply uv through native Python resolution | 55m | — — **DONE 2026-10-16** |
| [t002](done/t002.md) | Audit shared native callers and activation cases | 30m | t001 — **DONE 2026-10-16** |
| [t003](done/t003.md) | Render parity | 20m | t002 — **DONE 2026-10-16** |
| [t004](done/t004.md) | Simplify | 10m | t003 — **DONE 2026-10-16** |
| [t005](done/t005.md) | Test coverage | 40m | t003 — **DONE 2026-10-16** |
| [t006](t006.md) | Closeout | 20m | t004, t005 |

Six tasks,175m total.

## Definition of done

- Fresh permitted Free Python lockfile fixtures through both native clients reach their first uv-requiring deployment Live/HTTP200 without bypassing the command, installing tools in user code or repairing PATH. Default and valid explicit UV_VERSION behavior, exact source/lock/artifact markers and a real uv sync/run flow work.
- Maintained real resolver/generated-image execution regressions fail before and pass after for activation/source-root/version and meaningful neighboring cases. Selected Python, existing Poetry/JavaScript/Bun behavior and user environment/PATH are preserved; cache/version input behavior is verified.
- REST/GraphQL/MCP/dashboard semantics stay coherent, relevant required checks pass, and live versus source-only limits are explicit. Current Python-only diagnostic recoveries do not meet the original uv requirement.
- Owned parents/children/test metadata are absent, original baselines remain and quota termination is0. Closeout moves all completed tasks and the milestone through /pm with synchronized statuses and no stubs.

## Source + Goal linkage

- **Source:** [functional native CLI sweep350](evidence/cli-sweep350/README.md), [operator supply diagnosis](finding.md).
- **Goal linkage:** ADR008 dependable hosting, ADR018 Render-compatible native Python and ADR060 reviewed toolchain assembly.
- **Expected outcome:** an accepted uv.lock project can use its package manager during build and start without per-app tool bootstrap.
- **Why now:** both first native deployments prove the root lock file then fail uv127; Python-only diagnostic recoveries prove exact source and working Python/pip/Poetry while uv stays absent. Four current resolver observations and one actual generated image confirm the omitted supplier; no fix is on freshly fetched main.
- **Render parity included:** this is a tenant-facing native build/runtime behavior reached through the existing public runtime/env surfaces. [m72](../done/m72/README.md) and [m77](../done/m77/README.md) remain complete within their tool-specific accepted scopes.
