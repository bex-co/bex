# Render deploy-detail comparison

Recorded 2026-07-14 for `w5/m29` and rechecked 2026-07-15 for `w2/m38`, using the live deploy-page capture already described by `w9/m1`, Render's current public documentation, generated REST client, and official MCP tool source.

## Render reference

- A deploy is opened from the service Events timeline, and its detail page is the place to view that individual deploy's logs. The explorer supports text search and renders timestamp, instance, level, and message facts. See [Logs in the Render Dashboard](https://render.com/docs/logging#logs-for-an-individual-deploy-or-job).
- An in-progress detail page carries **Cancel deploy**. See [Deploying on Render — Canceling a deploy](https://render.com/docs/deploys#canceling-a-deploy).
- Successful and failed deploys settle visibly to **Live** and **Failed**, and the log feed is the primary debugging surface. See [Your First Render Deploy](https://render.com/docs/your-first-deploy#5-monitor-your-deploy).
- Render's public deploy object carries `commit { id, message, createdAt }` for Git-backed deploys, image facts for image-backed deploys, `createdAt`/`updatedAt`/`startedAt`/`finishedAt`, trigger, and eleven statuses (`created`, `queued`, build/pre-deploy/update progress and failure states, `live`, `canceled`, `deactivated`). Its REST list accepts status plus exclusive created/updated/finished before/after bounds. Sources: the official [`render-oss/render-mcp-server` generated client](https://github.com/render-oss/render-mcp-server/blob/main/pkg/client/types_gen.go) and [Render Public API OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json).
- Render documents the order build → optional pre-deploy command → deploy and exposes three overlapping-deploy policies. bex preserves its existing newest-wins behavior (`cancel_running_deploys`) rather than adding a policy setting in this milestone. Source: [Deploying on Render](https://render.com/docs/deploys).
- Render's official `list_deploys` MCP tool currently exposes `serviceId`, `limit`, and `cursor`; bex additionally exposes the REST status/time filters through MCP so all three of its own adapters remain equivalent. Source: [`render-oss/render-mcp-server/pkg/deploy/tools.go`](https://github.com/render-oss/render-mcp-server/blob/main/pkg/deploy/tools.go).

## bex result

- `/services/$serviceId/deploys/$deployId` shows the stored status, trigger, deploy id, resolved commit id/message when available, image, pre-deploy result, and all four deploy timestamps.
- Deploy rows persist the truthful observed subset of all eleven statuses. The status timeline combines the deploy row with service events whose `details.deployId` exactly matches the deploy, timestamps the current state with `updatedAt`, and renders an earlier `live` step plus the later replacement instant for `deactivated`. It never treats an `evt-…` id as a `dep-…` id or invents an unrecorded intermediate phase.
- The deploy-window log viewer interleaves `type=build`, `type=predeploy`, and `type=app` chronologically. A missing durable store produces the explicit “Build logs need the log store” note while non-build log legs remain usable.
- Events-list and detail-page actions share one Cancel/Rollback component. Rollback navigates to the newly created deploy, not the restored historical row.
- The dashboard labels and filters all eleven statuses, polls every open state, and stops for every terminal state. REST, GraphQL, and MCP return the same status/timestamps and accept the same status plus created/updated/finished time filters.

## Recorded drift

The lifecycle/timestamp gap recorded by `w5/m29` is closed by `w2/m38`. Remaining drift is explicit:

- bex records commit id/message best-effort for Git-backed deploys and omits the object when resolution is unavailable, but does not capture Render's commit author timestamp. Owned by `.pm/w2/011.md`.
- Omitting REST/GraphQL `limit` still returns the full history for pre-pagination bex clients, while Render REST defaults to 20. This is a deliberate compatibility contract, not an unowned implementation gap.
- bex exposes status/time filters on MCP in addition to Render's official tool's current limit/cursor inputs. This is a deliberate AI-native surface-parity superset; it does not alter the returned deploy semantics.
