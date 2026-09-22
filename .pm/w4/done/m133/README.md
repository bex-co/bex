# w4 · m133 — Report resources detached by blueprint sync

**Worker:** worker4 **Goal:** Make surviving unmanaged resources visible before and after sync **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Report blueprint-scoped detach actions and sync results — **DONE** | 30m | — |
| t002 | Show detach warnings in blueprint review and results — **DONE** | 30m | t001 |
| t003 | Render parity and documentation — **DONE** | 30m | t002 |
| t004 | Simplify — **DONE** | 30m | t003 |
| t005 | Test coverage — **DONE** | 30m | t004 |
| t006 | Closeout — **DONE** | 30m | t005 |

## Definition of done

Blueprint-scoped previews identify dropped or renamed managed resources as detach actions. Successful sync results and dashboard notices identify detached resources, explicitly preserving their running/billing lifecycle. No unrelated workspace resources are diffed as removed. Failed syncs do not falsely report a completed detach. Relevant suites pass.

## Source + Goal linkage

- **Source:** [w4/119](source-119.md); m125 solved claim release but deliberately left this gap.
- **Goal linkage:** ADR008 predictable managed hosting and agent-readable changes.
- **Expected outcome:** IaC users see resources left running outside the updated blueprint before and after sync.
- **Why now:** Renames and removals otherwise silently leave billable resources behind. API and UI work exceeds one hour.
- **Render parity:** Included because preview and sync REST/GraphQL/MCP/UI surfaces change; preserve non-destructive detach semantics.

## Verification in progress

- REST, GraphQL and MCP: 18 preview/validate combinations plus three sync-result tests pass, including foreign-ID refusal, surviving workloads and durable notices.
- Mutation that suppresses detach output fails all six scoped plan checks as expected (`/tmp/bex-w4-m133-mutation.log`).
- Dashboard plan, sync hook and detail tests: 28 pass; preview scoping and locale checks: 150 pass. TypeScript and targeted ESLint pass; codegen kept only feature hunks.
- Reviews found stale no-store descriptions and duplicate legacy-manifest compilation; fixes underway.
- Render non-deletion behavior rechecked against official Blueprint docs on 2026-09-22. Diagnostic fields are additive bex behavior.

- Full dashboard suite passes: 436 files, 3593 tests. Resource mapping reuse cleanup followed by 8 passing focused hook tests.
- Final backend lint: zero issues. Legacy invalid stored manifest regression preserves error status, releases active execution, and retains resource claims with no false detach note.

## Final verification — 2026-09-22

- Full backend `go test -p 1 ./...` passes against real PostgreSQL, native OpenFGA and local OpenBao, including final cleanup (`/tmp/bex-w4-m133-backend-final.log`). Backend lint: zero issues.
- Full dashboard suite: 436 files, 3593 tests pass. Subsequent mapper-only cleanup: 8 focused tests, TypeScript and targeted ESLint pass.
- Three simplify reviews complete. Mutation suppressing detach output fails scoped REST/GraphQL/MCP plan regressions.
- No production rollout or live Render response capture claimed. Resource survival matches Render docs; plan/result/history diagnostics are bex extensions.
