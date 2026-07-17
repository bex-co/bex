# m47 — `POST /v1/services` create returns `{service, deployId}`

**Status:** DONE 2026-07-16

## Problem

Render's `POST /v1/services` returns `{service, deployId}` (the `serviceAndDeploy`
schema, verified against the render-oss/cli generated client). bex's
`serviceAndDeploy` struct was missing the `DeployID` field entirely — the create
response silently returned only `{service}`. A Render-spec client navigating to the
new deploy's detail page after `services create` had to do a second `list_deploys`
call to find the deploy id, or drop the navigation link entirely.

## What shipped

- **`serviceAndDeploy`** (`apps/render.go`) gains `DeployID string \`json:"deployId,omitempty"\``.
- **`POST /v1/services` handler** (`apps/rest.go`) propagates `app.LatestDeployID`
  into the envelope. `LatestDeployID` is populated on the `AppView` by `Create`
  from `row.FirstDeployID` (returned by `store.CreateApp`) when the control-plane
  store is active. Omitted (never faked) when the store is inactive.
- **`recordingStore.CreateApp`** (`apps_test.go`) now returns `FirstDeployID: "dep-test"`
  so store-backed unit tests can observe the field without a real DB.
- **`TestREST_CreateResponseContainsDeployIDWhenStoreActive`** (`createowner_test.go`)
  asserts `deployId == "dep-test"` in the parsed `serviceAndDeploy` response.
- **ADR018** "Create service" row updated; gap-backlog row added.

## Commit

`feat(services): return deployId in POST /v1/services create response (w2/m47)`
