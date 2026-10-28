<!-- shared:deployments -->
## Deployments and health checks

A bex deploy is asynchronous. A successful trigger means only that bex accepted or queued the deploy; it does not mean the new version is live. Before triggering a deploy, confirm the workspace, service, intended source commit or image, and current live deploy. Capture the new deploy ID, monitor it to a terminal state, and do not report success until its status is `live` and the deployed workload has been verified.

Before configuring or troubleshooting deployments and health checks, fetch the current bex references:

- [deploys.md](https://render.com/docs/deploys.md) (HTML: <https://render.com/docs/deploys>)
- [health-checks.md](https://render.com/docs/health-checks.md) (HTML: <https://render.com/docs/health-checks>)

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read both references.

Read the relevant sections of both documents. Confirm current service-type support, command timeouts, health-check thresholds, rollout behavior, and configuration syntax rather than relying on remembered values.

Automatic deployment behavior depends on the service's source and settings. A service linked to a branch through a connected Git provider can deploy on each commit, wait for supported CI checks, or require manual triggers. Services that pull a prebuilt image and services linked by a public Git repository URL require manual deploys; moving a mutable image tag does not itself trigger one. When manually deploying a historical commit, verify whether that trigger method changes automatic-deploy settings. If automatic deploys remain enabled, a later branch update can replace the selected commit.

The normal lifecycle is build command, optional pre-deploy command, then start command and readiness checks. If a command fails or times out, the deploy fails and later steps do not run. A pre-deploy command runs after the build on a separate instance before the new version is deployed. Neither the build nor pre-deploy instance can access an attached persistent disk, and pre-deploy filesystem changes do not carry into the running service. Use pre-deploy for release tasks such as database migrations, not for producing runtime artifacts. Pre-deploy commands are supported for paid web services, private services, and background workers.

bex normally deploys services without downtime by starting the new version before retiring the current one. Web and private services continue routing traffic to the current instances until the new instances are ready. Multi-instance services roll out one instance at a time; if a replacement does not become healthy, bex cancels the rollout and retains or restores the previous version. Attaching a persistent disk disables zero-downtime deploys, so plan explicitly for an interruption.

A workflow deployment has a different completion model. It builds and caches the image used by future task runs and includes a task-registration step, but it does not execute those tasks. After a workflow deploy succeeds, verify that the expected task definitions are registered. Trigger and monitor a task run separately only when execution is intended. For workflow-specific deployment behavior, fetch the current workflow overview:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read the [Markdown reference](https://render.com/docs/workflows.md) or its [HTML version](https://render.com/docs/workflows).

Health checks apply to web services and private services. Both use TCP socket checks by default. Only web services support a configured HTTP health-check path. bex sends an HTTP `GET`; any `2xx` or `3xx` response received within five seconds succeeds, while other status codes, timeouts, and connection failures fail. Private services support TCP checks only.

For a new deploy, all new instances must pass their checks at the same time within the current readiness window, documented as 15 minutes, before bex routes traffic to them. For a running service, consecutive failures currently remove an instance from routing after 15 seconds and restart it after 60 seconds. Treat these thresholds as platform behavior, not user-tunable health-check interval settings, and verify them in the fetched reference before relying on exact timing.

Design an HTTP health endpoint to represent readiness for production traffic. Keep it fast and side-effect free. Check critical dependencies only when losing that dependency should make the entire instance unavailable; otherwise a transient dependency failure can unnecessarily remove healthy application capacity. Verify the configured path, response status, response time, listening port, and `0.0.0.0` binding when checks fail.

After traffic moves, bex gives the previous instance a drain period, then sends `SIGTERM`. The application should stop accepting new work, finish or safely abandon in-flight work, close connections, and exit within its shutdown delay. Ensure the actual application process receives the signal: container entrypoints and shell wrappers should use `exec` or explicitly forward signals. bex sends `SIGKILL` if the process remains running after the delay. The documented default shutdown delay is 30 seconds and the usual maximum is 300 seconds, but support varies by service configuration; confirm the current reference and Blueprint or API validation before setting `maxShutdownDelaySeconds`.

A service restart is a special deploy of the same commit and user-defined environment-variable values as the current running instance. It does not incorporate configuration changes saved after that deploy. Use a standard deploy when pending configuration must take effect.

Only one deploy runs at a time for a service. A workspace's overlapping-deploy policy determines whether a new trigger waits or cancels the in-progress deploy. Do not assume the policy or wait for every intermediate revision: inspect the workspace setting and identify the deploy that actually became live.

Treat rollback as a new deploy based on a retained previous build artifact, not as restoration of the entire service to an earlier point in time. It does not reverse database migrations, messages, external writes, or other side effects from the release, so confirm that the previous application version remains compatible with current external state. Disks and their data are not rolled back, some settings continue using current configuration, mutable image tags can resolve to a different image, and eligible artifacts are retained only for a limited period. Before rolling back, fetch the current rollback reference:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read the [Markdown reference](https://render.com/docs/rollbacks.md) or its [HTML version](https://render.com/docs/rollbacks).

Read which configuration comes from the target deploy and which remains current. After the rollback is created, inspect the service's resulting automatic-deploy setting instead of assuming whether it remains enabled, and deliberately restore the intended setting after the underlying issue is resolved.

Deploy hooks are secret credentials that authorize deploys. Never print, commit, or expose their full URLs. Before using one, fetch the current deploy-hook reference:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read the [Markdown reference](https://render.com/docs/deploy-hooks.md) or its [HTML version](https://render.com/docs/deploy-hooks).

Distinguish an immediately started deploy from a queued one, and continue monitoring the resulting deploy rather than treating the HTTP response as completion.

After a deploy becomes live, verify the behavior appropriate to the service type: make a request to a web or private service, confirm a worker is processing work, confirm a cron job or workflow invocation completes, and inspect recent runtime logs for new errors. If a deploy fails, diagnose the first failed lifecycle stage before retrying.
<!-- /shared:deployments -->
