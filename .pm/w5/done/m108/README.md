# w5 · m108 — Prevent canceled cron runs returning after history eviction

**Worker:** worker5 **Goal:** Keep an old manual trigger settled after its canceled run leaves the bounded display history. **Status:** done (2026-10-01)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render cancellation and reproduce history eviction — **DONE** | 30m | — |
| t002 | Persist manual-run settlement independently of history — **DONE** | 60m | t001 |
| t003 | Verify existing Apps, restarts and fresh triggers — **DONE** | 45m | t002 |
| t004 | Render parity and surface disposition — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- A focused reproduction demonstrates or disproves resurrection after terminal-history eviction and replacement of spec.cancelRun; if disproven, close with concrete evidence instead of a speculative patch.
- When confirmed and fixed, a canceled manual Job remains absent after more than ten later runs, a later cancellation, controller restart and service suspend/resume. A fresh manual trigger executes exactly once through the existing mechanism.
- Run settlement survives bounded display-history retention without unbounded status growth, suppressing valid fresh triggers, or resurrecting historical triggers during migration.
- REST, GraphQL, MCP and dashboard retain truthful cancellation/run-history semantics. This is bex correctness supporting Render cancellation parity; no Render history-retention implementation is claimed.
- Operator/type and affected backend tests plus lint pass; local lifecycle evidence and fixture cleanup are recorded.

## Source + Goal linkage

- **Source:** User-approved w5 brainstorm follow-up, 2026-10-01; residual documented in app_controller.go manualRunSettled. Builds on [w4/m114 t001](../../../w4/blocked/m114/done/t001.md) and [w5/m106](../m106/README.md), which cover retained cancellation history and scheduling during preemption, respectively.
- **Goal linkage:** ADR008 dependable hosting and ADR038/ADR018 cron execution semantics.
- **Expected outcome:** A canceled manual run never reappears merely because more than ten newer runs replace its history or the cancellation slot is reused; a fresh trigger still executes.
- **Why now:** The current settled check uses one cancellation slot or a matching terminal entry in a ten-run history while RunAt persists. Reproduce the documented residual now that preemption work has shipped.
- **Render parity included:** This changes user-visible deploy or cron behavior. Check REST, GraphQL, MCP and dashboard semantics against the t001 research, distinguishing direct Render requirements from bex correctness and undocumented details.

## Execution notes

Start with t001 before implementation. Work only in isolated dev-5 for live checks. Queue order is m107 → m108 → m109; m108 and m109 share cron code, so land m108 before changing it for m109. Research and all implementation tasks remain todo at filing.

## Research — 2026-10-01

[Render cron documentation](https://render.com/docs/cronjobs#single-run-guarantee) guarantees one active run, cancels the active run before a manual replacement, and delays a scheduled run while another is active. [The cancellation endpoint](https://api-docs.render.com/reference/cancel-cron-job-run) is `DELETE /v1/cron-jobs/{cronJobId}/runs`, returning 204 for cancellation. Neither source specifies internal retention, acknowledgement storage or replay after history eviction; this milestone is Bex correctness supporting that user-visible cancellation contract. No authenticated Render experiment is claimed.

Code triage confirms the suspected composition: `spec.runAt` persists; `manualRunSettled` consults only the current cancellation slot or terminal entries in `status.runs`; `cronRuns` caps that display history at ten. w4/m114 protects retained history; w5/m106 protects scheduling during preemption. Neither retains a settlement acknowledgement independent of history. A behavioral reproduction is being run before changing that mechanism.

## Reproduction

Confirmed before implementation: the regression creates/cancels the manual Job, lets eleven newer completed Job objects evict it from the ten-row history, replaces cancellation, and suspend/resumes with a fresh reconciler. The original Job is recreated. [Failing output](evidence/regression-before.txt). The fix therefore proceeds as **Work**, separate from m106 preemption.

## Implementation and compatibility

One internal `status.manualRunHandledAt` token acknowledges a created/observed Job or accepted cancellation, separately from terminal status. It persists using an optimistic App status patch before cancellation deletion or bounded history replacement; conflicts retry without writing a stale failure status. Active Jobs still pause scheduling, missing acknowledged Jobs are not recreated, and a new `spec.runAt` remains eligible.

A manual Job's exact trigger annotation and App owner identify it if a new trigger arrives before run-history persistence. Foreground preemption waits for that earlier Job's Pods; unrelated App-owned/foreign Jobs are excluded. A known legacy current Job can receive the binding before acknowledgement, including long fitted names.

Existing matching Job/history/cancellation evidence is adopted. Legacy intents whose evidence was already completely evicted cannot be distinguished from unhandled fresh requests and retain previous behavior; no age-based guess is made. External deletion between Job creation and its first acknowledgement can likewise erase the only evidence; manual Jobs have no TTL, and ordinary process restart retains the Job. These limits are explicit in ADR038; no arbitrary-deletion exactly-once guarantee is claimed.

Surface audit: REST/GraphQL/MCP and dashboard keep the existing run vocabulary, ten-entry display history and permissions. Evicted get/per-run cancel remains not-found; the internal token never synthesizes a pending row. Cancel-current remains 204. Existing backend cron adapter/error tests and ten dashboard cron-section tests pass; no UI/API schema change is needed.

## Validation and closeout

- Final operator `make test` (codegen/envtest), four-module `make lint` (including deadcode), types `go test ./...` and backend `go test ./...` passed. Optional local backend DB/OpenFGA integrations may skip when their test environment is absent.
- Existing backend cron REST/GraphQL/MCP adapter and coded-error checks passed; dashboard cron-section tests: 10 passed. No dashboard behavior or API schema changed.
- Regression coverage exercises the original eviction failure, legacy running/terminal history, missing legacy evidence, terminal Jobs outside the history cap, active acknowledged Jobs, persistence failure before deletion, create-before-ack recovery, status conflict fencing, new-trigger preemption before history, foreign/unrelated Job exclusion, and long-name legacy adoption.
- Simplify reuse/quality/efficiency reviews completed. Replaced manual annotation-map initialization with the existing metadata helper; no other worthwhile changes.
- [Live evidence](evidence/README.md) and [sanitized observations](evidence/runtime.json) prove cancellation, twelve actual accelerated later executions, history eviction, later cancellation, fresh reconciler, observed suspension/resumption and one fresh execution. REST/GraphQL/MCP histories agree; dashboard states were checked. Fixtures, workspaces, identity, namespaces and private credentials were removed; API remains healthy.
- The first live cycle missed observing the short suspension window, so the complete sequence was repeated with explicit Hibernated/CronJob-suspended and Running/resumed waits. This is scoped polling and accelerated Job creation, not shared-manager or natural-schedule evidence. Legacy missing-all and arbitrary external-deletion limits remain explicit; no Render retention internals are claimed.
