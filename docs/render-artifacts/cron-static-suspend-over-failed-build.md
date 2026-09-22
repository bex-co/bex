# Cron and static-site lifecycle during an unfinished build

Evidence checked **2026-09-21** for **w1/m163**. This is a primary-documentation review and a bex source/adapter audit. It is **not an authenticated observation of Render**, and the documents do not establish every timing claim in the original milestone.

## What Render documents

| Source | Dated excerpt | What it establishes |
| --- | --- | --- |
| [Render Public API OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json), checked 2026-09-21 | “Suspend the service with the provided ID.” / “Resume the service with the provided ID (if it's currently suspended).” | Separate `POST /services/{serviceId}/suspend` and `/resume` operations, both returning `202`; neither operation requires a deploy ID. Their descriptions do not specify timing, behavior of already-running cron executions, or an exception for an unfinished build. |
| [Deploying on Render](https://render.com/docs/deploys#deploy-steps), checked 2026-09-21 | “Your service continues running its most recent successful deploy (if any), with zero downtime.” | A failed build does not replace the last successful deploy. The zero-downtime section includes cron jobs and separately explains static sites' CDN implementation. |
| [Cron Jobs](https://render.com/docs/cronjobs), checked 2026-09-21 | “The new build does not affect in-progress runs (only future runs).” | Cron builds and executions have distinct lifecycles. The page defines a UTC schedule, but does not promise that schedule changes apply during a failed or unfinished build. |
| [Static Sites](https://render.com/docs/static-sites#immediate-cache-invalidation), checked 2026-09-21 | “each build is fully atomic” | A successful build is published as a unit. The page does not specify the behavior or propagation time of user suspension. |
| [Update service](https://api-docs.render.com/reference/update-service), checked 2026-09-21 | “Changes will not be deployed automatically.” | The general configuration-update documentation says a separate deploy request is needed to apply changes. It does not document a schedule-specific exception. Immediate cron schedule or command application cannot be claimed as proven Render parity from these documents. |

**Inference boundary:** independent lifecycle endpoints plus retained successful artifacts support separating availability intent from a newer build. They do not prove that Render suspends cron scheduling within a particular tick, that static suspension is instantaneous, or that schedule/command edits apply without a deploy. Those stronger claims require a live Render observation. The m163 implementation preserves bex's existing direct-intent contract rather than asserting undocumented Render timing.

## Bex contract and corrected static-site premise

When a cron job already has a deployed image, suspension, resumption and schedule edits must reach its existing CronJob on reconciliation even while the latest release has no usable image. That pass must retain the prior pod template and the latest build's pending/failed verdict. Command and environment changes remain pending release configuration (`lego/types/v1alpha1/app_identity.go:53`); applying them to the old image would execute configuration that never deployed. Manual runs use the same prior template. Suspending the scheduler prevents new scheduled Jobs; it does not promise cancellation of an already-created Job. A static site must stop resolving while suspended and resume its existing `status.activeRevision` after resumption, independently of the newest build. Static serving already satisfies that separation: it reads the App directly and excludes suspended Apps before considering the published revision, rather than depending on the controller reaching runtime dispatch.

The concrete static path is `lego/operator/internal/staticserver/resolver.go:66` (`Refresh`), with the suspension check at line 78 and published-revision check at line 81. The refresh loop is at line 104; its default interval is ten seconds (`lego/operator/cmd/staticserver/main.go:106`). A suspended site is absent from the host map, so `lego/operator/internal/staticserver/staticserver.go:209` returns HTTP 404 without reading cached or origin bytes. Resumption restores the same published prefix. This is an existing static-site response, **not** the web-service suspended interstitial. Consequently, the original t003 claim that the build halt itself bypasses static suspension is false, and “within one controller pass” is not its propagation model. Regression coverage and an HTTP observation can verify the existing contract without adding a static hold branch.

## Surface audit

The controller fix changes convergence beneath existing APIs. None of the audited lifecycle or schedule adapters adds a successful-build prerequisite.

| Surface | Suspend/resume | Cron schedule update and errors |
| --- | --- | --- |
| REST | `POST /v1/services/{id}/suspend` and `/resume` return `202` through `Service.Suspend`/`Resume` (`lego/backend/internal/apps/rest.go:759`). | `PATCH /v1/services/{id}` accepts `serviceDetails.schedule`, with top-level `schedule` as an alias; the nested value wins (`rest.go:811`). `ServicePatch` delegates schedule/command to `SetCronJob` (`settings.go:206`). Invalid schedules produce HTTP 400 through the common error envelope (`lego/backend/internal/core/http.go:286`). |
| GraphQL | `suspendService(id, confirm)` and `resumeService(id)` call the same verbs (`lego/backend/internal/apps/graphql.go:1429`). | `updateCronJob(id, schedule, command, confirm)` calls `SetCronJob` directly; schedule is required in this GraphQL mutation (`graphql.go:1396`). Validation failures become GraphQL errors. |
| MCP | `suspend_service` and `resume_service` call the same verbs (`lego/backend/internal/apps/mcp.go:730`). These are explicitly documented bex extensions over Render MCP. | `update_service` carries optional `schedule`/`command` into the same patch dispatcher (`mcp.go:749`). Common error adaptation preserves public failures as tool errors (`lego/backend/internal/mcputil/mcputil.go:34`). |
| Dashboard | `useServiceLifecycle` invokes those GraphQL mutations (`dashboard/src/features/services/hooks/use-service-lifecycle.ts:92`). Settings uses server action capabilities (`components/suspend-service-card.tsx:67`); the server checks permissions, protection and billing, without a build-state gate (`lego/backend/internal/apps/actioncaps.go:69`). | `useCronJob` invokes `updateCronJob` (`dashboard/src/features/services/hooks/use-cron-job.ts:37`). `CronDeploySection` disables editing for loading/permissions, not pending builds (`components/cron-deploy-section.tsx:38`). It re-sends the current command on schedule saves, so a nonempty command requires create permission; this is existing, explicit UI behavior (`components/cron-deploy-section.tsx:41`). |

The common lifecycle writer is `lego/backend/internal/apps/service.go:4210`: it authorizes the operation, applies protection/billing guards, persists suspension intent, then patches `spec.suspended`. The common cron writer is `service.go:3855`: it validates the schedule and type, then patches `spec.schedule`/`spec.command` without creating a release. Optional command omission preserves the current command in that writer. These transport and permission conventions are unchanged by m163.

API reads expose suspension and schedule intent (`service.go:1018`), so the dashboard's suspension polling (`use-service-lifecycle.ts:50`) is not evidence that a CronJob has stopped or static routing has refreshed. Runtime verification must inspect the CronJob/new Jobs and HTTP responses separately.

## Verification performed for this audit

On 2026-09-21, the existing focused backend checks passed:

```sh
cd lego/backend
GOWORK=off go test ./internal/apps -run 'TestCronScheduleContractAcrossSurfaces|Test.*Suspend|Test.*Resume' -count=1
```

`TestCronScheduleContractAcrossSurfaces` exercises actual REST and GraphQL validation plus the MCP handler's shared request path (`lego/backend/internal/apps/cron_schedule_surfaces_test.go:39`). These checks support adapter/validation consistency; they do not simulate Render, prove operator convergence, or replace m163's live cluster observations. The milestone's implementation verification and live evidence are recorded in its README.
