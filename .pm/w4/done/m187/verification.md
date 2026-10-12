# m187 — live DoD replay (t008)

**Date:** 2026-10-12 UTC. **Build:** fix `7ec90cf81` is an ancestor of `86af4f59e`, the head of the latest successful `deploy.yml` run `38254721878` (deploy (bex via Argo), success at 13:18Z). **Session:** one `scripts/qa-login.sh` login (the shared jar was backed up and restored byte-for-byte), workspace `bex` (`tea-d98210cbbpdc73dcrkvg`). Dashboard at 1400×900. REST/GraphQL/MCP calls went to `https://api.bex.co` using the same session cookie.

## Fixtures (all `qa-20261012-m187-*`, all deleted)

| kind | id |
| --- | --- |
| project A / B / C / D | `prj-db6ets95i38c73fd5kqg` / `prj-db6etqp5i38c73fd5kpg` / `prj-db6f2oh2s0sc73a8hi4g` / `prj-db6f3ah2s0sc73a8hia0` |
| env A/stage, A/prod | `evm-db6eu092s0sc73a8hhm0`, `evm-db6eu2p5i38c73fd5kt0` |
| env B/target-env, C/c-env, D/d-env | `evm-db6eu2p5i38c73fd5ks0`, `evm-db6f2op5i38c73fd5ldg`, `evm-db6f3ah5i38c73fd5lf0` |
| Free image web (`busybox:1.37.0` httpd, marker `qa-m187-marker`) | `srv-db6eu412s0sc73a8hhng` |
| env group, var `QA_PLACEMENT=qa-m187-group-saved` | `evg-db6eu895i38c73fd5ku0` (revision `egr1_AAAAAAAAAAE` the whole run) |

Starting owner inventory: services 5, postgres 4, key-value 2, projects 3, env-groups 1, blueprints 1. Public `https://qa-20261012-m187-web.onbex.co/` returned `qa-m187-marker` HTTP 200 after create deploy `dep-db6eu9p5i38c73fd5kug` reached live.

## 1. Move to project clears the foreign environment — PASS

- **Group linked.** The group (Stage) was linked to the service (A/Stage). At **14:33:07.900Z**, from a fresh A/Stage page, I opened the web row's Actions menu, chose Move to project, then `qa-20261012-m187-b`. This made exactly one mutation, `SetProjectServices {id: B, serviceIds:[srv]}`, which returned HTTP 200 with `serviceIds:[srv]`. At 14:33:19Z:
  - GraphQL `service`: `projectId` = B, `environmentId: null`.
  - GraphQL `services(ownerId)` showed the same.
  - A has `serviceIds:[]` and Stage has `serviceIds:[]`. B has `[srv]` and TargetEnv has `[]`.
  - REST `GET /v1/services/{id}` returned 200 with `projectId` = B and no `environmentId` key.
  - MCP `get_service` was not an error: `projectId` = B, with `environmentId` absent.
  - `GET /v1/services?ownerId=…&name=qa-20261012-m187-web&environmentId=<Stage>` returned `[]` (200).
  - The group link and its value survived. The fresh service Environment page showed `Linked (1) qa-20261012-m187-group`. Available to link listed only the pre-existing unscoped `qa-20261003-keys-r85`.
- **Group unlinked.** I unlinked the group, and config deploy `dep-db6evt95i38c73fd5l1g` reached live. Then `setEnvironmentServices(Stage,[srv])` put the service back in A/Stage. At **14:36:01.162Z**, from a fresh A/Stage page, Move to project → B again made one `SetProjectServices` call. At 14:36:11Z, GraphQL Get/List, REST and MCP read the same as above (B, null/omitted), and the REST Stage filter returned `[]`. At 14:36:48Z, the fresh service Environment page showed `Linked (0)`, and Available to link had only `qa-20261003-keys-r85`. The Stage group was not offered.
- The source page that was not reloaded still showed the web row in the 5 s after the mutation. It was gone at 14:34:04Z before reload, and fresh loads were correct. This is the same brief UI refresh lag the finding observed (m178's area), not API placement.

## 2. Passing controls — PASS

The public marker returned HTTP 200 after every step.

- Explicit `setEnvironmentServices(Stage)` from B/null gave A/Stage: A lists the service and B does not. Later, `setEnvironmentServices(TargetEnv)` from unassigned gave B/TargetEnv on GraphQL, REST and MCP.
- Stage→Prod (`setEnvironmentServices(Prod)`): A/Prod, with Stage `[]` and Prod `[srv]`.
- `deleteEnvironment(Prod)` at 14:37:28Z: the service stayed in A with `environmentId: null`. `GET /v1/environments/<Prod>` returned 404.
- Unassigned A→B gave B/null, and B→A gave A/null. Each was one `setProjectServices` call.
- UI Remove from project at 14:38:00.232Z made one mutation, `SetProjectServices {id: A, serviceIds: []}`. The service then had project null and environment null.

## 3. Linked-group deletion detaches instead of refusing — PASS

- **UI Project Settings → Delete Project (B).** The service was in B/TargetEnv, and the unlinked group had been moved Stage→TargetEnv (value and revision preserved). The group was linked, and config deploy `dep-db6f1rh2s0sc73a8hi2g` was live. In Project Settings I clicked Delete Project, got the dialog `Delete project "qa-20261012-m187-b"?`, and confirmed with Delete Project. No unlink came first. The page navigated to `/`, and I captured no error toast. At 14:40:24Z:
  - `GET /v1/projects/B` and `GET /v1/environments/<TargetEnv>` both returned 404.
  - The service was Running with `projectId: null` and `environmentId: null`. REST omitted both keys.
  - The group had `environmentId: null`, `serviceLinks:[srv]`, revision `egr1_AAAAAAAAAAE`, and value `qa-m187-group-saved`.
  - The marker returned 200.
- **REST `DELETE /v1/projects/C`** at 14:41:28Z returned **204**. Fixture C was equivalent: service and linked group in C/c-env. Project and environment then returned 404, and the service and group were unscoped with link, value and revision intact. The marker returned 200.
- **MCP `tools/call delete_project {id: D}`** at 14:42:01Z returned HTTP 200 with `result.structuredContent {"id": "prj-db6f3ah2s0sc73a8hia0"}` and no `isError`. Fixture D was equivalent. Project and environment returned 404, and the service and group were unscoped with link and value intact.
- **Direct REST `DELETE /v1/environments/<Stage>`** at 14:42:33Z returned **204**, with the service and linked group both in A/Stage. The environment returned 404. The service kept `projectId` = A (A lists it) with null environment, and REST omitted `environmentId`. The group was unscoped with the link and value retained. The marker returned 200.

## 4. Group-move guards — PASS

- At 14:39:11Z the group was linked to the service in B/TargetEnv. `moveEnvGroup(id, Stage)` was refused with `ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES`, `serviceIds:[srv]`, `targetEnvironmentId` = Stage. The group stayed in TargetEnv with `serviceLinks:[srv]`, revision `egr1_AAAAAAAAAAE`, and value unchanged.
- Moving the unlinked group Stage→TargetEnv succeeded, preserving the value and revision `egr1_AAAAAAAAAAE`.

## 5. Cleanup — PASS

- 14:42:53Z: `deleteEnvGroup`, then `deleteService`, then REST `DELETE /v1/projects/A` (204).
- 14:43:45Z: by-id reads returned 404 for the service, the group, projects A/B/C/D and all five environments. The public URL returned 404.
- The six owner-filtered ID sets matched the starting inventory exactly (5/4/2/3/1/1, same IDs).
- 14:44:09Z: `qa-login.sh --logout` on this run's jar returned `ok logged-out`, and whoami with that cookie returned 401. The cookie and state files were removed and the browser cookies cleared (it landed on `/auth/login`). The shared `qa-session.jar` from another session was left as it was found.

## Not verified

- No Kubernetes, CR, workload or PVC inspection: I did not use production kube credentials.
- Not live-exercised: sibling App kinds, datastore moves, protected/isolation-on environments, concurrency, recovery and mobile/zh.
- The UI Delete Project mutation response was not captured: the page navigated before the listener read it. The outcome comes from the API reads above.
- **Observation (outside this milestone):** right after `setEnvironmentServices(c-env)` moved the service out of the unscoped state, `moveEnvGroup(c-env)` refused once with `ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES` even though GraphQL already reported the service in c-env. A retry about 20 s later succeeded. `validateLinkedServiceEnvironments` (`lego/backend/internal/envgroups/service.go`) reads the App CR environment label, which trails the committed assignment. The same sequence for D and Stage succeeded on the first try.
- **Operator follow-up (not done):** re-saving production projects that still carry pre-fix (project, foreign environment) drift is the operator's decision.
