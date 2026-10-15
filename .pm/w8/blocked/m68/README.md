# w8 · m68 — Preserve executable command text

**Worker:** worker8 **Goal:** accepted build/start/pre-deploy/cron commands execute with the argument bytes the user authored. **Status:** blocked — t001–t003, t005, t006 done (upstream patch prepared; backend `commandText` + dashboard `commandText` shipped with regressions); t004/t007 need user-authorized render-oss/cli submission, upstream release, a Bex CLI pin/release and the deployed fix for hosted two-client proof

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve nonblank command flags in an upstream-ready change — **DONE** | 35m | — |
| t002 | Preserve command text in shared backend writers — **DONE** | 45m | — |
| t003 | Preserve command drafts in dashboard producers — **DONE** | 35m | — |
| t004 | Render parity — **BLOCKED (upstream submission + Bex release + deploy)** | 25m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 10m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout — **BLOCKED (t004 gates + hosted two-client replay)** | 15m | t005, t006 |

## Definition of done

- Native create/update for all four command flags preserve nonblank executable text; escaped terminal space/tab reaches the launched process. The actual hosted argument must be hex7461696c20 for the recorded space example, through fresh Bex and unmodified upstream journeys.
- Shared backend and dashboard producers preserve that text; REST/GraphQL/MCP/readback agree. Nil/omit, explicit blank clears, create-required validation, static build clearing and cron legacy-fallback/role behavior remain correct. Generic metadata/path normalization is unchanged.
- Maintained regressions and required affected checks pass. Any deliberate command-padding policy expectation change is explicit. Upstream external submission/adoption, published Bex release and backend/dashboard rollout are separately authorized and verified before claiming customer repair.
- Every owned fixture/child/session created for acceptance is cleaned; immutable baseline and quota are reconciled. Complete all tasks and move the milestone and tasks to done with synchronized statuses and no stubs.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs-cli` w8/sweep284; [finding](finding.md), [durable evidence](evidence/cli-sweep284/README.md).
- **Goal linkage:** ADR008 usable hosting, ADR018 Render-compatible command execution, ADR004 deployment/startup and ADR049 Blueprint command fidelity.
- **Expected outcome:** a valid escaped shell argument survives CLI, API, stored configuration and launch.
- **Why now:**32 native local captures alter accepted commands and two Free production services actually run the wrong argument while reporting success. Independent backend writers and source-established dashboard producers can reintroduce it even after a client fix.
- **Sizing:**195m / seven tasks across upstream CLI, backend, dashboard, parity and observable released acceptance. This exceeds an inbox note.
- **Render parity:** included because this affects tenant-facing CLI/API/UI command semantics. Actual Render production is untested; unchanged exact-pin and released2.29 builds establish the upstream boundary.

Severity minor. The private upstream/backend diagnostics are research material; QA has landed no product fix, dependency bump, deployment, commit/push or external submission. Two owned parents/two children are removed, baseline6/quota6/terminating0 verified. Browser/shared live writes and fixed-release acceptance remain open.
