# Finding — env-var write responses and create-service `generateValue` (qa run e9v4, 2026-10-24)

Probes ran in-page on `https://dashboard.bex.co` (Admin session in `bex-canary`, pin `29d63e1f7bea`). Generated values are masked below as `<44-char generated>`.

## 1. Env-group create usually answers every value as `""`

| request | response |
| --- | --- |
| `POST /v1/env-groups` `{name:"qa-20261024-e9v4-grp2", ownerId, envVars:[{key:"QA_GG",generateValue:true},{key:"QA_LIT",value:"lit"}]}` | `201 {"id":"evg-dbe4a4do6rts738soma0",…,"envVars":[{"key":"QA_GG","value":""},{"key":"QA_LIT","value":""}],…}` |
| immediately `GET /v1/env-groups/evg-dbe4a4do6rts738soma0` | `200 {…"envVars":[{"key":"QA_GG","value":"<44-char generated>"},{"key":"QA_LIT","value":"lit"}]…}` |
| `GET …/env-vars/QA_GG` | `200 {"key":"QA_GG","value":"<44-char generated>"}` |

Over 7 creates in this run, 4 answered all values `""` and 3 answered real values. The `GET` immediately after a blank create always answered `200` with values, so the data was stored. Only the write response is wrong.

- `lego/backend/internal/envgroups/rest.go:101-107`: create returns `s.committedRevealed(ctx)(s.CreateEnvGroup(...))`.
- `render.go:166-176` `revealedOne(…, committed=true)`: on **any** `revealGroups` error it logs `envgroups: group %s: reveal values for the write response: %v` and returns the masked `renderEnvGroup`. That masked body serialises each value as `""` instead of omitting it.
- `revealGroups` (`render.go:128-160`) runs three steps for the new group: `authorizeGroup(can_view_sensitive, group.ID)`, then `authorizeGroupSensitiveFresh` (workspace object, uncached), then `withGroupContent` / `readGroupContents`. One of these fails intermittently right after create.
- **Cause unverified.** The bex-api log line names it. Candidates:
  - The group's FGA parent tuple is not yet visible to the check.
  - The content row is still in a non-idle save state when the reveal runs. `viewOf` reports availability for non-idle groups.
  - The Secret read races its write.
- ADR018's Environment-groups row records the masked fallback as a failure path ("a committed write falls back to the masked body with the failure logged"). It is not meant to be the common outcome.

## 2. Per-variable / per-file env-group PUT answers the wrong shape

- `PUT /v1/env-groups/<id>/env-vars/QA_X` `{value:"x"}` → `200 {"key":"QA_X"}`.
- The pinned Render spec (`lego/backend/internal/api/openapi/render-public-api-1.json`) answers `200` with `#/components/schemas/envGroup` for:
  - `PUT /env-groups/{envGroupId}/env-vars/{envVarKey}`;
  - `PUT /env-groups/{envGroupId}/secret-files/{envVarKey}`.
- bex: `rest.go:162-176` returns `SetEnvGroupVarInput`'s `EnvVarView` (`service.go:892`), and `rest.go:182-190` returns `SetEnvGroupFile`'s `SecretFileView` (`service.go:931`). The secret-file shape is read from code, not probed.

## 3. `POST /v1/services` refuses `envVars[].generateValue`

- `POST /v1/services` `{type:"web_service", …, envVars:[{key:"WHOAMI_PORT_NUMBER",value:"10000"},{key:"QA_GEN",generateValue:true}]}` → `400 {"error":"bad request: request body contains unknown field \"generateValue\""}`.
- Render's `servicePOST.envVars` is the same `oneOf [{key,value},{key,generateValue}]` array as `PUT /services/{id}/env-vars` ("If true, Render generates a strong random value… Cannot be combined with `value`").
- bex: the create body `createServiceRequest.EnvVars []envVarInput` (`lego/backend/internal/apps/rest.go:203`) uses `envVarInput{Key, Value}` (`render.go:32-35`). The same struct backs the MCP create tools (`mcp.go:202`, `:317`, `:536`).
- Controls, all correct:
  - `PUT /v1/services/<id>/env-vars/QA_D` `{generateValue:true}` → `200` with a 44-char value.
  - `{value:"x", generateValue:true}` → `400`.
  - Bulk `PUT …/env-vars` with `{key:"QA_G",generateValue:true}` → `200`, generated, and the list was replaced.
  - Env-group create accepts `generateValue` (`envgroups/service.go:165-185`, `prepareCreateEnv` `:470-490`).

## Blast radius

- `revealedOne(…, true)` / `committedRevealed` serve every REST group write: create, rename, clone, move and link (`rest.go` POST/PATCH handlers). Each can hit the same fallback. Grep `committedRevealed(` for the full list.
- `envVarInput` has 4 users (REST create service, and MCP create service / job / static). The Render MCP server's `create_web_service` envVars shape should be checked against the pinned MCP schema before MCP accepts the extra field.
- `renderEnvGroup`'s masked serialisation feeds only the fallback. The list path is separate (w4/396).
