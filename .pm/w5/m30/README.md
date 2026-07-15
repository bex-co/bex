# w5 · m30 — Dockerfile path + start command dashboard controls

**Worker:** worker5 **Goal:** Make `dockerfilePath` and `startCommand` readable and editable in the dashboard after create, and make `dockerfilePath` available during Docker-service creation. **Status:** todo

## Tasks (in order)

| id   | title                                                          | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | Add safe GraphQL setting mutations for both fields              | 1h  | —          |
| t002 | Project runtime and current command/path into the dashboard     | 35m | —          |
| t003 | Add `startCommand` inline editing to Build & Deploy             | 45m | t001, t002 |
| t004 | Add Dockerfile-path inline editing with Docker-build gating     | 45m | t001, t002 |
| t005 | Add `dockerfilePath` to Docker-service creation                 | 40m | —          |
| t006 | Render parity                                                   | 30m | t003–t005  |
| t007 | Simplify                                                        | 30m | t006       |
| t008 | Test coverage                                                   | 50m | t006       |
| t009 | Closeout                                                        | 15m | t007, t008 |

## Definition of done

For a repo-backed service, the dashboard detail query returns the persisted `runtime`, legacy-compatible `builder`, `startCommand`, and `dockerfilePath`. Settings → Build & Deploy lets an applicable user edit and clear the start command, and lets a Dockerfile-built service edit and clear the Dockerfile path; the Dockerfile control is absent for native, image-backed, cron, and static services where it has no meaning. The Docker branch of the service-create wizard accepts a non-default Dockerfile path and sends it in `createService`. Both setting mutations enforce authorization and field applicability, values survive refetch/save/reload, the mock-cluster build uses the selected Dockerfile, and `yarn typecheck && yarn lint && yarn test` plus the affected backend tests pass.

## Source + Goal linkage

- **Source:** `.pm/w6/015.md` (filed by `w6/m21`'s t005 Render-parity check, 2026-07-14), reserved as `w5/m30` by `/pm-brainstorm more milestones for each worker` round 5 and fully materialized via `$pm` on 2026-07-14.
- **Goal linkage:** Render parity (service-create-fields + Build & Deploy settings row); w5's dashboard charter. Closes the create-only asymmetry `startCommand` had vs its siblings (`rootDir`/`preDeployCommand`/`autoDeploy`, each already inline-editable) and the total UI absence of `dockerfilePath`.
- **Expected outcome:** a user deploying a Dockerfile at a non-default path, or changing a start command after create, has a dashboard path — not just bex.yml/REST/GraphQL/MCP.
- **Why now:** the operator and create/read contracts landed in `w6/m21`, so this is the remaining user-facing seam; the exact inline-edit pattern is established and the gap blocks otherwise-supported monorepo/Docker deployments from being configured in the dashboard. Repository inspection during materialization found that `server.graphql` does not select the four required fields and GraphQL has no narrow setting mutations, so t001–t002 make the UI safe and truthful instead of driving the broad create-or-update mutation. Render parity is included because this changes the dashboard and GraphQL surfaces.
