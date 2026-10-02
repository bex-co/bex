# w5 · m109 — Enforce the twelve-hour cron runtime limit

**Worker:** worker5 **Goal:** Bound scheduled and manual cron execution consistently with Render's documented twelve-hour limit. **Status:** done (2026-10-01)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render runtime limit and Kubernetes timing — **DONE** | 20m | — |
| t002 | Enforce one deadline policy for scheduled and manual cron runs — **DONE** | 30m | t001 |
| t003 | Verify deadline termination and subsequent scheduling — **DONE** | 40m | t002 |
| t004 | Render parity and surface disposition — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 25m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- Dated Render research establishes the twelve-hour limit and records known/unknown timer origin, termination grace and status behavior for scheduled and manual runs.
- Both scheduled and manual execution paths enforce the selected limit. Queueing, image pull, container start and Job suspension timing are explicitly compared with Kubernetes semantics; any unverified difference is documented.
- A shortened-deadline local drill proves termination, terminal API/dashboard history, release of scheduling and a later run. The record explicitly says it is an accelerated drill, not a real twelve-hour execution.
- Cancellation and existing single-run/preemption behavior continue to work; no Render retry guarantee is inferred or retry policy changed.
- Operator and affected backend tests plus lint pass; REST/GraphQL/MCP/UI outcomes are consistent and disposable fixtures are removed.

## Source + Goal linkage

- **Source:** User-approved w5 brainstorm and Render parity review, 2026-10-01; [w5/m106 research](../m106/README.md) records the public limit while current scheduled/manual Job construction sets backoffLimit without an execution deadline.
- **Goal linkage:** ADR008 predictable hosting resource usage and ADR038/ADR018 cron parity.
- **Expected outcome:** Scheduled and manually triggered cron runs terminate at the researched execution limit, expose a truthful terminal result, and allow subsequent scheduled work.
- **Why now:** Both cron Job construction paths lack a duration bound; add the documented limit after establishing timer origin, grace and status behavior rather than assuming a Kubernetes field exactly reproduces Render.
- **Render parity included:** This changes user-visible deploy or cron behavior. Check REST, GraphQL, MCP and dashboard semantics against the t001 research, distinguishing direct Render requirements from bex correctness and undocumented details.

## Execution notes

Start with t001 before implementation. Work only in isolated dev-5 for live checks. Queue order is m107 → m108 → m109; m108 and m109 share cron code, so land m108 before changing it for m109. Research and all implementation tasks remain todo at filing.

## Research — 2026-10-01

[Render cron documentation](https://render.com/docs/cronjobs#single-run-guarantee) stops an active run after twelve hours and documents no manual-run exemption. Its timer origin, termination grace and timeout status are unspecified. No authenticated Render experiment is claimed.

[Kubernetes Job API](https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/) measures `activeDeadlineSeconds` from Job `status.startTime`, when the controller begins processing, rather than container startup. Pending scheduling and image pulling therefore consume this window (inference from the documented origin). Suspending the Job resets that timer on resume. Bex service suspension suspends the CronJob schedule, not its existing Jobs: [CronJob suspension](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#schedule-suspension) affects future runs.

[Job termination](https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup) produces Failed/DeadlineExceeded after the deadline; controller delay and Pod termination grace can extend actual process termination. Kubernetes v1.31+ waits for Pods to terminate before terminal Failed, so FailureTarget alone must not release manual scheduling. The existing terminal projection already respects that distinction.

**Selected policy:** set Job-level `activeDeadlineSeconds=43200` in both scheduled templates and new manual Jobs; retain backoffLimit=0. [CronJob template changes](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#modifying-a-cronjob) affect future Jobs only, so safely identified active owned runs also receive the cap during existing history observation, preserving their start time and any shorter deadline. Existing Job deadline updates are permitted by [Kubernetes validation](https://github.com/kubernetes/kubernetes/blob/v1.34.0/pkg/apis/batch/validation/validation.go#L591-L602). Already-overdue adopted Jobs can fail promptly. Terminal/deleting/foreign/unidentified Jobs are untouched. Exact Render timing remains unverified; this uses a conservative Job-lifetime bound, not a claimed twelve hours measured from container startup.

The existing Job Failed → CronRunFailed → `unsuccessful` projection serves REST, GraphQL and MCP; the dashboard already renders it as failed. No timeout-specific wire enum, UI or retry policy is introduced. A shortened local deadline tests the actual termination mechanism, explicitly not a twelve-hour execution.

## Implementation and regression evidence

`cronRunActiveDeadlineSeconds` is shared by the scheduled Job template and new manual Jobs. The existing `cronRuns` Job-list pass applies the cap to active owned Jobs; no extra list/read is introduced. A shared ownership predicate excludes foreign and unrelated Jobs. Existing shorter deadlines, start time and retry policy are preserved; terminal/deleting/unidentified Jobs are untouched. Legacy older manual Jobs with no recoverable trigger identity remain outside automatic adoption.

[Before-fix regression](evidence/regression-before.txt) observes nil deadlines in both constructors, expecting literal 43200. The final focused tests pass and cover fourteen adoption/exclusion cases, no-op reconciliation, unchanged start time, FailureTarget still active, terminal DeadlineExceeded as Failed, timeout GC without replay, and a fresh manual trigger with its own deadline. REST/GraphQL/MCP cron adapter/error regressions pass; existing projections and dashboard statuses require no code change.

## Validation and closeout

- Operator `make test` (codegen/envtest), four-module `make lint` (including deadcode), and affected backend cron adapter/error tests passed. No backend/types/dashboard source change was required.
- Three Simplify reviews (reuse, quality, efficiency) found no worthwhile changes. The ownership predicate, Job list, terminal-state helpers and optimistic patch utility are reused; adopted deadlines cause no repeated no-op writes.
- [Runtime report](evidence/README.md) and [sanitized observations](evidence/runtime.json) prove actual default 43,200-second scheduled/manual objects, then real natural-scheduled and manual termination under a temporary 12-second Go overlay. DeadlineExceeded remains pending through Pod grace, then becomes unsuccessful across REST/GraphQL/MCP and Failed in the dashboard.
- A later natural CronJob tick and a fresh manual run each succeeded once; cancellation returned 204, and controller-observed suspension/resumption passed. 234 samples observed at most one active-or-terminating Job. This is sampled evidence, not proof against every possible scheduler race.
- All fixtures, both workspaces/namespaces, identity, private credentials, observer/reconciler processes and temporary deadline override were removed. dev-5 API health remained 200.
- Limits: scoped polling instead of shared-manager watch delivery; accelerated deadline rather than twelve elapsed hours; timeout Pods' final exit codes were not retained, but their termination and terminal Job conditions were observed. Render's timer/grace/status details remain unverified; unidentified legacy Jobs are not automatically patched. Retry behavior is unchanged.
