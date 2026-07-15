# Render deploy-detail comparison

Recorded 2026-07-14 for `w5/m29`, using the live deploy-page capture already described by `w9/m1` and Render's current public documentation.

## Render reference

- A deploy is opened from the service Events timeline, and its detail page is the place to view that individual deploy's logs. The explorer supports text search and renders timestamp, instance, level, and message facts. See [Logs in the Render Dashboard](https://render.com/docs/logging#logs-for-an-individual-deploy-or-job).
- An in-progress detail page carries **Cancel deploy**. See [Deploying on Render — Canceling a deploy](https://render.com/docs/deploys#canceling-a-deploy).
- Successful and failed deploys settle visibly to **Live** and **Failed**, and the log feed is the primary debugging surface. See [Your First Render Deploy](https://render.com/docs/your-first-deploy#5-monitor-your-deploy).
- Render's public deploy object carries `commit { id, message, createdAt }` for Git-backed deploys, image facts for image-backed deploys, timestamps, trigger, and eleven statuses (`created`, `queued`, build/pre-deploy/update progress and failure states, `live`, `canceled`, `deactivated`). Source: the official [`render-oss/render-mcp-server` generated client](https://github.com/render-oss/render-mcp-server/blob/main/pkg/client/types_gen.go).

## bex result

- `/services/$serviceId/deploys/$deployId` shows the stored status, trigger, deploy id, resolved commit id/message when available, image, pre-deploy result, and timestamps.
- The status timeline combines the deploy row with service events whose `details.deployId` exactly matches the deploy. It never treats an `evt-…` id as a `dep-…` id and never invents unrecorded intermediate phases.
- The deploy-window log viewer interleaves `type=build`, `type=predeploy`, and `type=app` chronologically. A missing durable store produces the explicit “Build logs need the log store” note while non-build log legs remain usable.
- Events-list and detail-page actions share one Cancel/Rollback component. Rollback navigates to the newly created deploy, not the restored historical row.
- The dashboard recognizes Render's full eleven-status vocabulary so future backend additions get correct labels, colors, polling termination, and cancel availability without another UI migration.

## Recorded drift

bex now records commit id/message best-effort for Git-backed deploys and omits the object when resolution is unavailable; it does not capture Render's commit author timestamp. Deploy rows still persist only four coarse statuses. Full transition timestamps and the deeper stored lifecycle are filed as `w2/m38`; when they land, this page can consume them without changing its route or composition model.
