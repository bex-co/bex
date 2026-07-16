# Live deploy following

Render's UX for a git-sourced service: after creation or manual redeploy, the dashboard navigates immediately to the in-flight deploy's detail page and streams build output live without any manual refresh.

## Observed behaviour

### Service creation (git-source)

1. User completes the create wizard and clicks **Deploy**.
2. Dashboard navigates directly to `/services/<id>/deploys/<firstDeployId>`.
3. The deploy detail page shows phase `building` — a live build-log pane streams the BuildKit / CNB stdout in real time (individual lines appear as the builder runs, not batched at the end).
4. When the build finishes, the phase flips to `live` (or `failed`) without any reload — the deploy poll detects the status change automatically.

### Manual redeploy

1. User clicks the **Manual Deploy** button from the deploy list or service header.
2. Dashboard navigates immediately to the new deploy's detail page.
3. Same live build streaming as above.

## bex implementation (w3/m14)

| Surface | Mechanism |
| --- | --- |
| Navigate on create | `createService` mutation returns `latestDeployId`; wizard navigates to `/services/$id/deploys/$deployId` when present |
| Navigate on manual deploy | `manual-deploy-button.tsx` already navigates on mutation success (`deploys/useManualDeploy`) |
| Live build streaming | `GET /v1/logs/subscribe?type=build&resource=<name>` — SSE from the build Job pod's `buildkit` container (`core.BuildContainer`); `FollowLogs` now streams `type=build` in addition to `type=app` |
| Deploy status auto-refresh | `useDeploy` polls every 3 s while `isTerminalDeployStatus` is false — no extra wiring needed |
| History fallback | `useDeployLogs` keeps the GraphQL `type=build` query for historical log reads (store-backed); the SSE leg is `enabled: !endTime` (in-flight only) |

## Gaps vs Render

- Render streams build output via WebSocket; bex uses SSE. Wire format identical (log lines) — transport is the documented divergence (ADR006, ADR010).
- Render shows a "cancel build" button inline on the build pane; bex exposes cancel via the deploy-actions menu (w2/m10), not inline in the log pane.
