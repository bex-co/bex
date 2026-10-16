# w8 · m73 — Supply pnpm in native Node builds and runtime

**Worker:** worker8 **Goal:** a native Node service can run the documented pnpm command during its build and at runtime through the unchanged CLI. **Status:** done — all six tasks; deployed acceptance and exact cleanup in sweep338

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Supply pnpm before native Node build and retain it at runtime | 55m | — — **DONE 2026-10-15** |
| [t002](done/t002.md) | Audit shared callers and toolchain cache transitions | 20m | t001 — **DONE 2026-10-15** |
| [t003](done/t003.md) | Render parity | 15m | t002 — **DONE 2026-10-15** |
| [t004](done/t004.md) | Simplify | 10m | t003 — **DONE 2026-10-15** |
| [t005](done/t005.md) | Meaningful execution coverage | 35m | t003 — **DONE 2026-10-15** |
| [t006](done/t006.md) | Live closeout — **DONE 2026-10-16** | 20m | t004, t005 |

## Definition of done

- Fresh explicit Free Node creates through published Bex and unchanged Render pin run pnpm successfully during the authored build and runtime, reach Live/HTTP200 with exact markers and preserve Node/npm/Yarn controls. No user-installed workaround is required. Preserve the original two-client negative and diagnostic receipts.
- Meaningful generated-artifact execution passes for all reviewed Node22/24/26 lines, static build extraction and shared/cache transitions. Preserve primary language choice, m61 command/PATH/ENTRYPOINT and env/file behavior and non-Node controls. Current source string observations alone do not close this gate.
- REST/GraphQL/MCP/Blueprint/dashboard configuration still reaches the same operator behavior without client repair or a new API field. Render parity and any unmeasured broader utility/version behavior are explicit. Appropriate tests, lint, pin inventory and simplify pass.
- Future owned parents/deployment children/configs/processes are fully cleaned with immutable baseline and quota retained. Every task and observable acceptance holds before moving the milestone to done; code shipping alone is insufficient.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs-cli` w8/sweep315,2026-10-15 UTC; [finding](finding.md) and [complete receipts](evidence/cli-sweep315/README.md).
- **Goal linkage:** ADR008 dependable Render-alternative hosting, ADR018 unchanged CLI/native-runtime compatibility and ADR060 reviewed toolchain assembly.
- **Expected outcome:** pnpm is available before the authored Node build and in the resulting runtime image while npm/Yarn keep working.
- **Why now:** both unchanged native creates fail with missing pnpm/127, while two same-service Node controls prove correct Node24 and expose pnpm ENOENT at build/runtime. The current generator supplies no pnpm setup.
- **Sizing:**155m/six tasks; consistent setup across supported primary images, shared/cache validation and actual hosted execution exceed a sub-hour inbox fix.
- **Render parity included:** existing public native runtime/command/environment surfaces feed this operator change. Render documents pnpm in build/runtime; independent Render hosting and broad all-native utility parity remain unverified.

The original sweep315 diagnostics below establish the reported failure. Implementation f5b6c6e29 and hosted acceptance in sweep338 complete this repair; QA supplies no product implementation or platform deployment.

## Closed 2026-10-16 — sweep338

[Full acceptance, observer qualifications and cleanup](../../evidence/native-node-default-branch-pnpm-sweep338/README.md). Both native first omitted-branch creates reach Live/master/HTTP200 with independent commit/package/markers, both explicit-master edits create no deployment, and build/runtime pnpm/npm/Yarn pass without an install workaround. The original Bex runtime timeout and canceled faulty timing observer are excluded as QA limits. Shared readback/export, focused source/Docker execution pass; Bex delayed stability passes and Render cold-wake recovery remains unmeasured; two parents/four children are absent with baseline/quota preserved. The original failure evidence is historical. QA supplies no product implementation or platform deployment.
