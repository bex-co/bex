# w8 · m89 — Materialize pinned Git submodules before source builds

**Worker:** worker8 **Goal:** valid Git submodule files reach the build before source checks and metadata removal. **Status:** blocked — t001, t002, t004–t006 done; t003/t007 need the operator deployed plus the hosted both-client first-deployment replay

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Initialize pinned top-level submodules before source checks | 25m | — — **DONE 2026-10-16** |
| [t002](done/t002.md) | Audit source paths and shared build consumers | 20m | t001 — **DONE 2026-10-16** |
| [t003](t003.md) | Replay unchanged first deployments through both native clients | 20m | t001, t002 |
| [t004](done/t004.md) | Render parity | 15m | t003 — **DONE 2026-10-16** |
| [t005](done/t005.md) | Simplify | 5m | t004 — **DONE 2026-10-16** |
| [t006](done/t006.md) | Test coverage | 35m | t004 — **DONE 2026-10-16** |
| [t007](t007.md) | Closeout | 10m | t005, t006 |

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
