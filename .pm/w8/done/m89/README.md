# w8 · m89 — Materialize pinned Git submodules before source builds

**Worker:** worker8 **Goal:** valid Git submodule files reach the build before source checks and metadata removal. **Status:** done

**Estimate:**130m across7 tasks. Source implementation5123a1dc6 and completed clone/audit/test work are accepted by [the original hosted both-client first-deployment replay](evidence/cli-sweep412/README.md).

## Verified outcome —2026-10-17

Both unchanged native clients create fresh original Free Node24 fixtures against the exact original public parent and Gitlink. Both first children reach Live without an authored fetch prefix or replacement deploy. Native build logs and six real HTTP200 responses prove exact parent files and the required1072-byte basic/LICENSE hash, on each fixture's original boot. Six shared configuration reads/two explicit-env reads agree. Exact two-parent/two-first-child/URL/local cleanup, baseline preservation, current quota accounting and zero termination pass.

[Original failure and researched repair](finding.md), [original evidence](evidence/cli-sweep393/README.md), [fresh acceptance](evidence/cli-sweep412/README.md). Actual serving SHA/OCI, recursive/nested behavior and unrun hosted providers/types/paid/browser/physical teardown remain explicit limits. The replay changes no product code; evidence/status moves are uncommitted.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Initialize pinned top-level submodules before source checks — **DONE** | 25m | — |
| [t002](done/t002.md) | Audit source paths and shared build consumers — **DONE** | 20m | t001 |
| [t003](done/t003.md) | Replay unchanged first deployments through both native clients — **DONE** | 20m | t001, t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 5m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 35m | t004 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- Supported top-level Gitlinks are populated at the parent's pinned commits before native/Dockerfile consumers and source-path checks; original parent/ref/metadata behavior and visible failure classification are preserved.
- Actual clone regressions fail before and pass after, including no-module, ordinary/relative URLs, non-tip child pin, module-contained source paths and meaningful source failures. Separate kpack/prebuilt paths retain their own contracts.
- Fresh published Bex and unchanged Render-pin original first deployments reach Live/HTTP200 with exact source/required file, without the authored fetch workaround. Shared configuration and exact owned cleanup pass.
- Required operator checks/lint, Render parity, simplify, test coverage and consistent full closeout hold before done-folder moves. Untested hosted/OCI/physical teardown boundaries stay explicit.

## Source + Goal linkage

- **Source:** [functional CLI sweep393 finding](finding.md) and [full evidence](evidence/cli-sweep393/README.md), both native first builds fail and exact-fetch controls serve the expected pinned file.
- **Goal linkage:** [ADR008 reliable hosting](../../../../docs/ADR008-vision.md), [ADR018 Render parity](../../../../docs/ADR018-render-parity.md), [ADR060 source/build reliability](../../../../docs/ADR060-build-worker-reliability-and-performance.md).
- **Expected outcome:** repositories with standard top-level modules build from complete pinned source without tenant-authored dependency fetch repairs.
- **Why now:** a valid ordinary hosting journey fails in both unchanged native clients, and the actual shared clone producer reproduces the missing file before build execution. Existing image/tool support is sufficient for initialization.
- **Sizing:**130m/2h10m across seven tasks. Shared clone ordering/transport/failure preservation, source-path consumers, behavioral regressions and actual unchanged first-deploy acceptance exceed one sub-hour note.
- **Render parity included:** automatic Git-submodule data is user-visible deployment behavior through existing public source configuration; no new field or CLI fork is warranted.
