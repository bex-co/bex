# m48 — `deploy` object carries `serviceId`

**Status:** DONE 2026-07-16

## Problem

Render's vendored OpenAPI schema lists `serviceId` as a field on the `deploy`
object. bex's `renderDeploy` struct omitted it entirely, so a Render-spec client
reading a deploy response had no direct pointer back to which service it belonged
to — they had to carry the service id through their own call chain rather than
reading it off the deploy response.

The data was already present as `store.Deploy.AppID` — it just wasn't projected
into the view or wire format.

## What shipped

- **`DeployView`** (`deploys/service.go`) gains `ServiceID string`.
- **`view(d store.Deploy)`** populates `ServiceID: d.AppID`.
- **`renderDeploy`** (`deploys/rest.go`) gains `ServiceID string \`json:"serviceId,omitempty"\``.
- **`toRenderDeploy`** propagates `ServiceID: d.ServiceID`. Omitted (`omitempty`)
  when the deploy was created without a store row (bare-CR path).
- **GraphQL** (`deploys/graphql.go`) `Deploy` type gains `"serviceId"` field.
- **ADR018** stale `w2/011 inbox` gap-backlog row corrected to `done (w2/m42)`;
  new m48 row added.

## Commit

`feat(deploys): add serviceId to deploy wire format (w2/m48)`
