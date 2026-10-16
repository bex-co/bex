# w8 · m76 — Honor a GitHub repository's default branch on create

**Worker:** worker8 **Goal:** omitting branch in a GitHub service create uses the repo default and makes the first deployment work through the unchanged CLI. **Status:** done — all six tasks; deployed acceptance and exact cleanup in sweep338

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Resolve omitted GitHub create branches from repository metadata | 55m | — — **DONE 2026-10-15** |
| [t002](done/t002.md) | Audit branch selection consumers and exports | 35m | t001 — **DONE 2026-10-15** |
| [t003](done/t003.md) | Render parity | 20m | t002 — **DONE 2026-10-15** |
| [t004](done/t004.md) | Simplify | 10m | t003 — **DONE 2026-10-15** |
| [t005](done/t005.md) | Meaningful default-branch regression coverage | 45m | t003 — **DONE 2026-10-15** |
| [t006](done/t006.md) | Live closeout — **DONE 2026-10-16** | 20m | t004, t005 |

## Definition of done

- Fresh published-Bex and unchanged-Render native creates omit branch on the wire, return/store the verified repository default, and reach Live/HTTP200 with exact build/runtime markers on the first deployment. No manual branch correction is required.
- Deterministic source tests cover main/master/trunk metadata, explicit branches, image/no-repo, unavailable metadata, actual service adapters, persistence and the first commit/clone. Preserve explicit branch and auto-deploy-off edit behavior; exports retain selected branches, including explicit main on a non-main-default repo.
- Shared default sites and providers are audited/dispositioned, unknown enclosing repository context is preserved for m75, and no global main→master constant change or unmeasured provider integration is introduced. REST/GraphQL/MCP/dashboard create semantics/docs agree with the returned branch.
- Appropriate backend/github/API tests, lint and simplify pass. All owned fixtures/children/configs/processes are removed with immutable baseline and quota preserved. Complete every task and observable gate before moving tasks/milestone to done; code shipping alone is insufficient.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs-cli` w8/sweep319,2026-10-15 UTC; [finding](finding.md) and [native/source/public-metadata evidence](evidence/cli-sweep319/README.md).
- **Goal linkage:** ADR008 reliable hosting, ADR006/018 Render-compatible API/CLI creation and ADR026 existing GitHub source metadata.
- **Expected outcome:** an omitted branch resolves to the actual repository default before any persisted service/first build uses it.
- **Why now:** two unchanged native creates omit branch yet return main and fail cloning it; public metadata says master, and both same-service explicit-master deployments reach Live with exact markers. Current source unconditionally selects the main constant.
- **Sizing:**185m/six tasks; metadata/default preparation, persistence/commit/export and shared-surface validation exceed a sub-hour note.
- **Render parity included:** public source creation semantics across existing REST/GraphQL/MCP/dashboard surfaces. Current live evidence is public GitHub Node only; other providers/contexts remain explicit audit dispositions.

The original sweep319 diagnostics below establish the reported failure. Implementation 4ae5ae08e and hosted acceptance in sweep338 complete this repair; QA supplies no product implementation or platform deployment.

## Closed 2026-10-16 — sweep338

[Full acceptance, observer qualifications and cleanup](../../evidence/native-node-default-branch-pnpm-sweep338/README.md). Both native first omitted-branch creates reach Live/master/HTTP200 with independent commit/package/markers, both explicit-master edits create no deployment, and build/runtime pnpm/npm/Yarn pass without an install workaround. The original Bex runtime timeout and canceled faulty timing observer are excluded as QA limits. Shared readback/export, focused source/Docker execution pass; Bex delayed stability passes and Render cold-wake recovery remains unmeasured; two parents/four children are absent with baseline/quota preserved. The original failure evidence is historical. QA supplies no product implementation or platform deployment.
