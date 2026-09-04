# w6 · m126 — Creating a project returns the internal view, not Render's project shape — 5 of 7 project handlers skip the mapper, and conformance only checks reads

**Worker:** worker6 **Goal:** creating, renaming or re-linking a project returns the same Render-shaped object that reading it does, so a client can rely on one shape per resource **Status:** done — every project handler now funnels through `renderProject` (POST/PATCH inline like the reads; the three link PUTs via the new ctx+error `core.HandleLinksMapped`, since `renderProject` reads `environmentIds` and so cannot use the context-free `HandleMapped`/`HandleLinks` view). `internal/api` now validates project create/update/read responses against the pinned Render schema and drift-guards every write's key set against a read (`TestProjectResponsesConformToRenderSchema`, `TestProjectWriteResponsesMatchReadShape`) — reverting any write handler to the internal view turns CI red (demonstrated). **t002 decision — divergence recorded (not born-empty):** `POST /v1/projects` accepts Render's required `environments` array (so a Render-shaped/CLI client is not 400'd by the strict decoder) but does not provision them; the response's `environmentIds` truthfully reports the project's real, initially-empty set. Recorded in [ADR018](../../../docs/ADR018-render-parity.md). GraphQL/MCP were already internally consistent (all project verbs return the bex-native `ProjectView` on those extension surfaces); only REST, the Render-public-schema-bound surface, had the drift.

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |     |
| ---- | ------------------------------------------------------------------------------------------ | --- | ---------- | --- |
| t001 | Emit Render's project shape from every project handler, not only the two reads               | 50m | —          | — **DONE** |
| t002 | Decide what `POST /v1/projects` does about Render's required `environments` input            | 40m | —          | — **DONE** (divergence recorded: input accepted, not provisioned) |
| t003 | Close the conformance gap: project operations, and CREATE/UPDATE response bodies             | 50m | t001       | — **DONE** |
| t004 | Render parity                                                                                 | 25m | t001, t002, t003 | — **DONE** (GraphQL/MCP already consistent; REST corrected) |
| t005 | Simplify                                                                                      | 20m | t004       | — **DONE** (`HandleLinks` now delegates to `HandleLinksMapped` — one implementation) |
| t006 | Test coverage                                                                                 | 40m | t004       | — **DONE** |
| t007 | Closeout                                                                                      | 15m | t005, t006 | — **DONE** |

## Definition of done

- **Create returns Render's shape.** `POST /v1/projects` returns a body containing `owner`, `environmentIds` and `updatedAt`, and containing **none** of `ownerId`, `serviceIds`, `databaseIds`, `keyValueIds`. Today it returns exactly the reverse — keys `createdAt, databaseIds, id, keyValueIds, name, ownerId, serviceIds`.
- **Create and read agree.** The key sets of `POST /v1/projects` and `GET /v1/projects/{id}` for the same project are identical. They differ today.
- **Every write path matches.** `PATCH /v1/projects/{id}` and all three `PUT .../{service,database,keyvalue}-links` return that same shape. _These four were **not** observed this run — a browser `PATCH` throws `Failed to fetch` under `w6/m121`'s CORS method list — so their current shape is read from the code (`Rename` returns `ProjectView`; the links handlers pass `identity`). Verify them; do not inherit the inference._
- **A project is not born empty,** or the divergence is recorded — whichever `t002` decides.
- **Drift fails the build.** `cd lego/backend && go test ./internal/api/...` fails if a project handler returns the internal view, and fails if a create/update response drifts from Render's schema. Demonstrate by reverting one handler and watching it go red.
- **Environments still pass** — they are correct today via `core.HandleMapped` and are exactly what a refactor of that helper would break.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, **60th run**, 2026-08-27, journey 1 (projects / environments). Workspace `tea-d98210cbbpdc73dcrkvg`. All probes re-runnable.

  Created a project, then read it back:

  ```
  POST /v1/projects  {"ownerId":"tea-…","name":"qa-20260827-proj"}  -> 201
    keys: createdAt, databaseIds, id, keyValueIds, name, ownerId, serviceIds

  GET  /v1/projects/prj-da8al3vm2e9c73ft6710  -> 200
    {"id":"prj-da8al3vm2e9c73ft6710","name":"qa-20260827-proj",
     "owner":{"id":"tea-d98210cbbpdc73dcrkvg","name":"","email":"","type":"team"},
     "environmentIds":["env-da8al7vm2e9c73ft672g"],
     "createdAt":"2026-08-27T21:22:23.4899Z","updatedAt":"2026-08-27T21:22:23.4899Z"}
  ```

  | field            | POST            | GET               |                              |
  | ---------------- | --------------- | ----------------- | ---------------------------- |
  | `owner`          | **absent**      | present (object)  | Render **required**          |
  | `environmentIds` | **absent**      | present           | Render **required**          |
  | `updatedAt`      | **absent**      | present           | Render **required**          |
  | `ownerId`        | present (flat)  | absent            | not in Render's schema       |
  | `serviceIds`     | present         | absent            | not in Render's schema       |
  | `databaseIds`    | present         | absent            | not in Render's schema       |
  | `keyValueIds`    | present         | absent            | not in Render's schema       |

- **Which one is correct — named, not left open.** Render's pinned `project` schema has exactly six properties and requires all of them: `required: ['id','createdAt','updatedAt','name','owner','environmentIds']`, `props:` the same six. There is no `serviceIds`/`databaseIds`/`keyValueIds`/`ownerId` in Render's project object at all. **The GET shape is correct; the POST shape is the defect.** The list form is correct too — it builds `renderProjectWithCursor{Project: rendered}`.

- **Root cause — the mapper is applied by hand, and only on the read paths.** `lego/backend/internal/projects/rest.go`:

  ```
  :95-97    GET   /v1/projects (list)               -> renderProjectWithCursor{Project: rendered}   CORRECT
  :112-118  GET   /v1/projects/{id}                 -> s.renderProject(...)                         CORRECT
  :101-110  POST  /v1/projects                      -> s.Create(...)                — raw ProjectView
  :120-129  PATCH /v1/projects/{id}                 -> s.Rename(...)                — raw ProjectView
  :138      PUT   /v1/projects/{id}/service-links   -> core.HandleLinks(…, identity) — raw
  :139      PUT   /v1/projects/{id}/database-links  -> core.HandleLinks(…, identity) — raw
  :140      PUT   /v1/projects/{id}/keyvalue-links  -> core.HandleLinks(…, identity) — raw
  ```

  `identity` is defined explicitly at `:77` (`identity := func(p ProjectView) ProjectView { return p }`), so the links handlers pass the internal view through deliberately. Both `Create` (`service.go:332`) and `Rename` (`service.go:382`) return `ProjectView`. **Count: 2 of 7 handlers emit Render's shape; 5 emit the internal view — and every one of the 5 is a write.**

- **The control — the sibling resource makes this impossible to get wrong.** `lego/backend/internal/environments/rest.go:169-179` wires the mapper into the handler helper itself:

  ```go
  mux.HandleFunc("POST /v1/environments", core.HandleMapped(http.StatusCreated, …, toRenderEnvironment))
  mux.HandleFunc("GET /v1/environments/{id}", core.HandleMapped(http.StatusOK, …, toRenderEnvironment))
  ```

  Verified live: the environment **create** response carried the full Render shape (`databasesIds`, `redisIds`, `envGroupIds`, `protectedStatus`, `networkIsolationEnabled`, `ipAllowList`, `serviceIds`, `keyValueIds`). Environments are right for a **structural** reason — the mapper is not optional there.

- **The layer cannot express the fix as-is.** `core.HandleMapped` (`internal/core/http.go:360-368`) takes `view func(T) V` — no context, no error. But `renderProject` (`projects/rest.go:67-73`) is `func (s *Service) renderProject(ctx, p ProjectView) (renderProject, error)`, because it calls `s.environmentIDs(ctx, p.ID)` to populate the Render-required `environmentIds`. So "just use `HandleMapped` like environments" does not compile — which is probably why the write paths skipped it. `t001` must decide the mechanism rather than discover this.

- **Why nothing caught it — conformance only checks reads.** `internal/api/conformance_test.go`'s `operationPath` map (documented at `:360` as "maps an operationId to the bex REST path under test") covers exactly **ten** operationIds, every one a read: `list-services · retrieve-service · list-postgres · retrieve-postgres · retrieve-redis · list-secret-files-for-service · list-custom-domains · list-events · retrieve-event · list-redis`. No project operation is under test, and **no create/update/replace response of any resource is**. There is also **no** allowlist entry for any project operation in `conformance_allowlist_test.go` — so unlike the `serviceDetails` divergence, this is not waived, it is simply unexamined.

- **A second, smaller divergence — projects are born with no environments.** Render's `projectPOSTInput` requires `['name','ownerId','environments']`. bex's POST decodes only `{name, ownerId}` (`rest.go:102-105`) and accepted a body with no `environments`, returning 201. The resulting project had **zero** environments (`GET /v1/environments?projectId=…` → `[]`) until one was created by hand. `t002` owns this as its own claim, not folded into the shape defect.

- **Goal linkage:** [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md) and [docs/ADR032-environments.md](../../../docs/ADR032-environments.md). `DO_NOT_DO.md` #31 commits bex to being a server the **official Render CLI** can drive rather than building its own — which makes a non-conformant create response a direct hit on that decision, since the CLI generates its types from this schema.

- **Expected outcome:** one shape per resource, whichever verb produced it.

- **Why now:** journey 1's promise is that resources land in the right project and environment "everywhere they are listed". **The placement itself is correct** — this run confirmed the environment's `serviceIds`, the service's own `environmentId`/`projectId`, and GraphQL all agreed. What is wrong is the project object's shape on write, which is the first response any client sees for a resource it just created.

- **Render parity:** included (t004) — this is precisely a REST wire-shape change. Confirm GraphQL and MCP project reads are consistent with the corrected shape; **neither was probed this run**.

- **Blast radius:** seven handlers in one file, five of which change; `renderProject` has 2 call sites today and gains 5. `core.HandleMapped` has callers beyond projects (environments at minimum) — if a ctx+error variant is added, existing callers stay untouched; if the helper itself changes, every caller needs regression coverage, environments first.

- **Adjacent classes:** the LIST response already nests under `{project, cursor}` per Render's `projectWithCursor` and must keep doing so; `DELETE` returns 204 with no body and is unaffected; an unauthorized caller must keep receiving the existing authz error **before** any shape work, so the fix cannot turn a 403 into a 200-with-empty-object.

- **Unverified this run — carried as work, not presented as observation:** the `PATCH` and three links-`PUT` response shapes (browser `PATCH`/`PUT` are blocked by `w6/m121`'s CORS method list, so these are code-read only); **GraphQL and MCP** project shapes; the **dashboard's** project page; and whether any resource **other than projects** has the same write-path mistake — `t003` is expected to reveal that, and it was not surveyed here.
