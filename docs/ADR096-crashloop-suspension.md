# ADR096 — A service that crash-loops for 24 hours is suspended (`stuck_crashlooping`)

**Status:** Accepted, 2026-11-03 (w2/m253). Render parity: no divergence.

## Context

Render documents that it "suspends a service that repeatedly fails to start and become healthy over a 24-hour period … often caused by a service configuration error, a bug in application code, or an undersized compute plan", and emails the workspace when it does ([render.com/docs/notifications](https://render.com/docs/notifications)). Its public API reports the reason in `suspenders` with the value `stuck_crashlooping` (`suspenderType` in the pinned OpenAPI). bex served the value nowhere: a crash-looping service ran, restarted and paged forever (eden-cms-v2 was liveness-killed 33 times in 16 hours, 2026-08-20).

Render does not publish the exact rule, so bex records its own.

## Decision

1. **Rule.** A service is suspended when it has been failing to start and become healthy for 24 hours: its Ready condition is `False` with a start-failure reason — `CrashLoopBackOff` (the kubelet's verdict for a container that keeps exiting) or `HealthCheckFailing` (running but never passing its health check) — and has stayed so since the later of Ready's last transition and the sweeper's first sighting at the current spec generation. The sighting is recorded in `app.bex.co/crashloop-observed` (`<generation>@<time>`) and cleared on recovery. It exists because Ready alone cannot restart the window: while a service is suspended its Ready stays `False` (reason `Suspended`), so after a resume, or after a deploy that fails the same way, Ready's transition time is still the old one; a resume or deploy changes the spec generation instead, which starts a fresh window. Any healthy moment also moves Ready's transition time. Reasons that are not start failures (`Unschedulable`, `ContainerCreating`, image-pull waits, quota stalls, node loss) never count.
2. **Exclusions.** Services already suspended (by the owner, the Free instance-hours budget or a previous crash loop), services in `Building`/`Deploying` (a fix may be on its way), and cron jobs and static sites (no long-running instance). Paid and free services are treated alike, as on Render.
3. **Mechanism.** `internal/crashloop.Sweeper` in bex-api lists Apps every five minutes; when an eligible service's window has closed it first enqueues the email (so a failed notification suspends nothing and retries) and then sets `spec.suspended` and the annotation `app.bex.co/crashloop-suspended` (the suspension time) through the same write path as the Free-hours suspension. The operator parks the service as for any suspension; the control-plane reconciler observes the transition as `service_suspended` (Events feed, push, webhooks).
4. **Reporting.** REST, GraphQL and MCP report `suspended` with `suspenders: ["stuck_crashlooping"]`; the dashboard's Resume card explains why and what to fix.
5. **Notification.** The workspace is emailed through the failure-alert queue (w2/m252) as a failure notification: a service's `notificationsToSend` and each member's failure preference apply. The alert's source key is the service and the moment its window started, so retried or racing sweeps mail once.
6. **Resume.** An owner's resume is allowed at once (unlike the Free-hours suspension) and clears the annotation. If the cause is not fixed, the service starts failing again and is suspended after another full window.
7. **Window.** `BEX_CRASHLOOP_SUSPEND_AFTER` (a Go duration) overrides the 24 hours for development and tests; production leaves it unset.

## Consequences

- Forgotten crash loops stop consuming node capacity and stop paging; owners learn why the service stopped.
- A service that serves briefly between crashes is not suspended; Render's wording ("fails to start and become healthy") supports that reading, and it avoids suspending a degraded but partly working service.
