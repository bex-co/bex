# w9 · m180 — Build Sources: close the gaps between ADR097 and the code

**Worker:** worker9 **Goal:** Build Sources behave as ADR097 and Render say on every path. A create can link services, a successful build always reaches every auto-deploy linked service, linked image/Docker services get the right port and container policy, and Blueprints and the API agree on names, scope and export. **Status:** in progress — t001–t009 done; t010 after the production check

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Create accepts `serviceIds` and `envGroupIds`, as on Render — **DONE** | 45m | — |
| t002 | Durable build fan-out: no linked service misses a build across a bex-api restart — **DONE** | 60m | — |
| t003 | A service linked to a Docker or image source gets the carrier's port mode and container policy — **DONE** | 45m | — |
| t004 | Build source names are unique on every surface; Blueprint scope and export keep project and `fromGroup` — **DONE** | 60m | — |
| t005 | Close the service-vs-carrier quota race — **DONE** | 45m | — |
| t006 | Guard SQL enumerations of `apps` against carriers; fix ADR097 and ADR018 text drift — **DONE** | 30m | — |
| t007 | Render parity: REST, GraphQL, MCP and dashboard against Render's Build Sources — **DONE** | 20m | t001, t002, t003, t004, t005, t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007, t008 |
| t010 | Closeout | 10m | t009 |

## Definition of done

On production:

- `POST /v1/build-sources` with `serviceIds` links those services.
- After a build source build, every linked auto-deploy service deploys the new image, including when bex-api restarts during the fan-out (verified by the recovery path in a test and one live build).
- A web service linked to an image source serves on the image's port with the `render-v1` policy.
- A duplicate build source name is refused on REST, GraphQL and MCP.
- Generate Blueprint exports a project-scoped build source under `projects[].buildSources` with its `fromGroup` links, and the export re-plans as a no-op.
- ADR097's Decision text matches the implementation.

## Source + Goal linkage

- **Source:** user request 2026-11-08 (`/pm for ADR097 for w9`), from a read-only audit of [ADR097](../../../docs/ADR097-build-sources.md) against the code at `9217e3b49`:
  - `apps/build_sources.go:162-164` (create refuses `serviceIds`);
  - `store/reconciler.go:1002-1005` (fan-out in a fire-and-forget goroutine);
  - `apps/service.go:2608-2646` (port mode and policy read only the service's own source; the open follow-up from w2/m254 t005);
  - `store/store.go:2118` and `blueprint_build_sources.go:211-213,358` (name uniqueness, root-vs-project scope);
  - `blueprint_generate.go:491-520` (export drops project scope and `fromGroup`);
  - `core/build_source_guard_test.go:66` (guards only `AppList`, not SQL over `apps`);
  - the ADR097 quota residual (`store/namespaces.go:187`).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); hosting reliability.
- **Expected outcome:** monorepo users relying on one build feeding several services never end up with a stale service or a wrong port, and Render clients' create bodies work unchanged.
- **Why now:** w2/m254 closed on its happy path. These gaps surface only on restarts, image sources and Blueprints, which are exactly where users will hit them next.
- **Overlap:** w3/057 already tracks two ADR018 corrections (line 82's unexplained ◐ and line 420's stale `w2/m254` "open"); this milestone does not repeat them. t006 fixes only the "16 operations" count.
- **Parity:** included, because these are user-facing REST/GraphQL/MCP/Blueprint behaviors with Render as the reference.

## Live check (2026-11-08, production 548af90d4)

- **Setup:** QA user (cookie session) in workspace `tea-daif693dqjvc73e7as3g` (bex-canary). All `qa-bs-` fixtures were deleted and verified 404, the workspace is back to its 5 services and no build sources, and the Kratos session was logged out. No CLI grant was minted.
- **1. Create with `serviceIds` links (PASS):**
  - `POST /v1/build-sources` (image `docker.io/nginxinc/nginx-unprivileged:alpine`) with `serviceIds:["srv-dboi0u1ogl4s73954gog"]` returned 201 `bds-dboi10pf86qs73b56vhg` with `serviceLinks:[qa-bs-own]`, and the service's `buildSourceId` read back the source.
  - **Observation:** an image source records its first build at create (`bld-dboi10pf86qs73b56vig`), and that build's fan-out deployed the newly linked auto-deploy service (`dep-dboi15pf86qs73b56vjg`). A PATCH link saves without deploying, but create-with-`serviceIds` deploys through the first build's fan-out. Render documents nothing here; decide whether that is intended.
- **2. Duplicate name (REST, GraphQL PASS; MCP FAIL, pre-existing):**
  - REST answered 409 `BUILD_SOURCE_NAME_TAKEN`, and GraphQL `createBuildSource` returned an error with `extensions.code: BUILD_SOURCE_NAME_TAKEN`.
  - MCP `create_build_source` fails for every input: "validating root: required: missing properties: [ownerId]". Passing `ownerId` is refused ("ownerId is not supported by MCP tools; use workspaceId").
  - Cause: the tool binds `CreateBuildSourceRequest` directly, and its `OwnerID` is tagged `json:"ownerId"` without `omitempty` (`lego/backend/internal/apps/build_sources.go:120`), so the SDK's inferred input schema requires it while the advertised schema (`workspaceId`) does not. This predates m180, which changed only the tool description (`build_sources_graphql.go:313-318`).
- **3. Linked image service gets the image port (FAIL):**
  - Web services linked to an image source cannot deploy. `srv-dboi061ogl4s73954gmg` (created with `buildSourceId`) and `srv-dboi0u1ogl4s73954gog` (linked at create) both went `update_failed`: "image port discovery requires this service's built image repository" (`dep-dboi07pf86qs73b56vgg`, `dep-dboi15pf86qs73b56vjg`). The URL answered 503.
  - Cause: t003 now gives the linked service the carrier's image-port mode, but the operator's `resolveImageNetwork` reads only the App's own built image (`ownImageReference`) or an external `Spec.Image` (`lego/operator/internal/controller/image_network.go:182-211`). A linked service's image is `Spec.BuildSource.Image`, so it falls through to the error at :211.
  - Before m180 this path was not taken: the service fell back to port 10000 and nginx on 8080 would not have answered either.
  - Fix: resolve the network from `Spec.BuildSource.Image`, treating an external ref like `Spec.Image` and a registry build like the own-image branch.
- **4. Fan-out deploys each auto-deploy linked service exactly once (PASS, normal path):**
  - Git source `bds-dboi2j1f86qs73b570b0` (express-hello-world) and linked `srv-dboi6o9f86qs73b570f0` (start `node app.js`, auto-deploy yes) went live and served 200.
  - A triggered build `bld-dboicahf86qs73b570gg` succeeded in 150 s and opened exactly one deploy, `dep-dboicspf86qs73b570h0`, which went live. After another 90 s there was still one, and the service served 200.
  - The restart-recovery path was not exercised live; it is covered by the PG and reconciler tests.
- **5. Export of a project-scoped source with `fromGroup` (PARTIAL FAIL):**
  - For `bds-dboif31ogl4s73954h70`, created with `projectId` and `envGroupIds:[evg-dboif2pf86qs73b570j0]`, and its linked `srv-dboif69f86qs73b570kg`, Generate Blueprint wrote the source under `projects[{name: qa-bs-proj, buildSources: [qa-bs-scoped]}]` (PASS).
  - **No `fromGroup`** appeared under it, not even after an explicit `POST /v1/build-sources/{id}/env-groups/{evg}` (200). The group's `serviceLinks` read `[]`. Not even the unselected-group `sync: false` expansion appeared, so the carrier App's `Spec.EnvFromSecrets` looks empty in production.
  - Export reads `carrier.Spec.EnvFromSecrets` (`blueprint_generate.go:531` → `generateEnvVars` ~:684), and the link path is envgroups `linkFetched`/`rollLinked` (`envgroups/service.go:1060-1112`). Not verified (no prod kubeconfig here): whether the group's vars actually reach the carrier's build Job. If not, this is a functional gap, not just export.
- **6. Carrier builds are not service deploy webhook events: NOT RUN.** It needs a webhook receiver; it is covered by `TestPGCarrierBuildsAreNotServiceDeployWebhookEvents` (real Postgres).
- **7. A project's service list never moves a carrier (PASS):** `PUT /v1/projects/prj-dboieq1ogl4s73954h5g/service-links` with `["bds-dboi2j1f86qs73b570b0","srv-dboi6o9f86qs73b570f0"]` moved the service into the project and left the carrier with no `projectId`.
