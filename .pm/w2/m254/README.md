# w2 · m254 — Build Sources: one build feeding several services, as on Render

**Worker:** worker2 **Goal:** bex serves Render's `/build-sources` contract (git- or image-backed, build-time env, linked services, builds list and trigger) and `buildSourceId` on service create, PATCH and GET, on every surface. **Status:** in progress — t001–t008, t013–t015 done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design ADR: Build Sources on bex's CR and build model — **DONE** | 1h | w2/m251/t004 |
| t002 | Types, storage and build fan-out — **DONE** (fan-out itself lands with t006) | 2h | t001 |
| t003 | REST CRUD for /v1/build-sources — **DONE** | 1.5h | t002 |
| t004 | Build-time env vars, secret files and env-group links — **DONE** | 1.5h | t003 |
| t005 | buildSourceId on service create and PATCH — **DONE** | 1h | t003 |
| t006 | Trigger a build and list builds — **DONE** | 1h | t004, t005 |
| t007 | GraphQL and MCP equivalents — **DONE** | 1h | t006 |
| t008 | Dashboard: Settings → Build → Source "Update source" dialog — **DONE** | 1.5h | t007 |
| t013 | Deploys report buildId; deploy and rollback accept a build — **DONE** | 45m | t006 |
| t014 | POST /v1/build-sources answers 500 after creating the source: PGStore.CreateApp overwrites the bds- id with a srv- id — **DONE** | 45m | t003 |
| t015 | Meter build source build minutes — **DONE** | 1h | t006 |
| t016 | A native-runtime git build source never builds: the operator requires a startCommand the carrier never has — implemented, awaiting production verification | 1h | t006 |
| t017 | A service linked to a native build source starts like a native service: native-init and bash | 1h | t016 |
| t009 | Render parity | 15m | t008, t013, t014, t015, t016, t017 |
| t010 | Simplify | 15m | t009 |
| t011 | Test coverage | 30m | t009 |
| t012 | Closeout | 10m | t010, t011 |

## Definition of done

- On production, the pinned Render CLI or REST creates a git-backed build source, links two services to it, and one triggered build deploys both.
- `buildSourceId` round-trips on `GET/POST/PATCH /v1/services`; create with `buildSourceId` plus `repo`/`branch`/`image`/`runtime` is rejected as Render does, and a region mismatch returns `build_source_region_mismatch`.
- Build-time env vars, secret files and env-group links on a build source work through REST, GraphQL and MCP; the dashboard Settings → Build → Source dialog switches a service's build source.
- Deploys report `buildId`, and `POST /v1/services/{id}/deploys` with `buildId` deploys that build (round-5 drift, w2/m251).
- ADR018's Build Sources row is `✅` (or `◐` with each divergence signed off).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-02 #4; live OpenAPI 2026-11-02 (tag `Build Sources`, 16 operations; `buildSourceId` on `service`, `servicePATCH`, `webServiceDetailsPOST`, `privateServiceDetailsPOST`, `backgroundWorkerDetailsPOST`, `cronJobDetailsPOST`). Render dashboard flow: service Settings → Build → Source → Edit → "Update source" dialog.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** monorepo/multi-service users build once and deploy many, exactly as on Render; Render clients that send `buildSourceId` work against bex.
- **Why now:** the field is on Render's core service object; Render clients will read and write it. Depends on w2/m251's pin refresh.
- **Render parity included:** new REST/GraphQL/MCP/UI surface.

## Work log — 2026-11-03

- **t001:** [ADR097](../../../docs/ADR097-build-sources.md). A build source is a carrier App of type `build_source` that the operator builds but never runs; linked services deploy a copy of its image in their own repository (cross-repository blob mount + manifest PUT with the builder credential), so no per-link Zot ACL entry and no registry restart; bex-api fans a successful build out to auto-deploy services and records `buildId` on deploys.
- **Size:** the operator side (carrier type, skip-runtime path, `registry.CopyManifest`, linked-service image selection) plus 20 REST operations, the fan-out and the dashboard dialog is roughly three times the 11h estimate. Per the `/loopx w2` checkpoint, the remaining independent items (m255, 065 → m256, 064, 067, 068) are drained first and m254 is implemented last as one uninterrupted block.
- **t002 (part 1, operator + CRD):** `spec.type: build_source` and `spec.buildSource {name, image}` on the App CRD, with CEL refusals for a link plus `repo`/`image` or on a type Render does not link. The operator settles a carrier's build as `Running` + `Ready=Built` with no Deployment, Service or Ingress. A linked service deploys `buildSource.image`: a platform build is copied into its own repository under the same digest (`registry.CopyManifest`: cross-repository blob mounts + manifest PUT with the builder credential), an external image runs as is, and an unselected build waits (`AwaitingBuildSource`). The selected image is part of the artifact identity, so selecting another build is a new release. Remaining for t002: bex-api storage (carriers as `apps` rows of type `build_source`, whose deploy rows are their builds) and excluding carriers from every service enumeration behind a guard test.
- **t002 (part 2, storage):** id kinds `bds-` (build source; `bsr` is taken by Blueprint syncs) and `bld-` (a build, derived from the carrier's deploy row). Migration 0188: `apps.build_source_id` / `build_source_image` (FK `NO ACTION`, so a linked carrier cannot be deleted alone but a workspace cascade still succeeds); a carrier row is never grouped into a project or environment nor itself linked (CHECK); `apps_check` admits a linked service with no repo/image; the product-analytics type CHECKs admit `build_source`. `ListApps` and `CountAppsForTenant` exclude carriers; the projector stamps `spec.buildSource` (carrier CR named by its `bds-` id) and propagates a newly selected build. Remaining for t002: the CR-enumeration guard over bex-api's 38 App listings, which lands with t003 when carriers are first created. Follow-ups recorded by the storage review: service by-id routes (`GetApp` callers) must refuse a `build_source` row (t003), and carrier builds are not metered yet because usage rollups read `ListApps` — Render bills Build Source build minutes, so metering carrier builds lands with t006.
- **t002 (part 3, enumeration guard):** `core.ServiceApps` / `IsBuildSource` keep carriers out of every App enumeration that means "the services": all core name/id resolvers behind service verbs (a carrier resolves as not found), the service list, workspace resource limits, free-hours and crash-loop sweeps, platform rebuilds, Blueprint workspace snapshots, GitHub push matching, env-group service links and service-reference propagation. `TestEveryAppEnumerationDeclaresBuildSources` fails on any new App enumeration that neither filters nor states why carriers belong (purge, billing enforcement, reconciler, inventory, credential binding). t002 is complete.
- **t003 (REST CRUD):** `POST/GET /v1/build-sources`, `GET/PATCH/DELETE /v1/build-sources/{id}` with Render's object (git or image, `serviceLinks`, `projectId` scope in `apps.scope_project_id`, migration 0189). A carrier is created through the service create path and edited through the service setters under `core.WithBuildSources`; delete refuses a linked source (`BUILD_SOURCE_LINKED`). The App count quota is widened by exactly the carrier rows so services stay capped by the API server; carriers have their own ceiling (25). Create refuses `serviceIds`/`envVars`/`secretFiles`/`envGroupIds` by name until t004/t005. For t005: a native-runtime source builds without a start command, so the operator must run a linked service's carrier image with that service's own start command.
- **t004 (build-time environment):** the twelve env-var / secret-file / env-group routes under `/v1/build-sources/{id}` mount the same handlers as the service routes (`secrets.registerEnvRoutes`, envgroups link/unlink) behind `Base.BuildSourceScope`, which refuses anything but a carrier. Writes are the staged, save-without-deploying ones; `autoDeployGated` keeps every group link or group change from rolling a carrier out. Create seeds `envVars` and `secretFiles`; `envGroupIds` and `serviceIds` on create wait for t005.
- **t005 (service links):** `buildSourceId` on create (refuses `repo`/`branch`/`image`/`buildFilter`/`runtime`/build commands/`rootDir`/registry credential by name; a native-runtime source requires a `startCommand`, a Docker or image source refuses one; `region` cannot mismatch on a single-region installation), on PATCH (`LinkBuildSource`: only `serviceDetails.envSpecificDetails.startCommand` may ride along; the service's own source is cleared and the link saved source-pending, without deploying) and on the service object. The link stamps `spec.buildSource` with the source's current build, or none until it has built. The operator runs a linked service's `startCommand` through the shell (no repo/image ⇒ not a native-baked launch), which is what a native carrier image built without one needs.
- **t005 review follow-ups:** (1) a web/private service linked to a Docker or image source does not get image-port detection (`configureNewImageCompatibility` keys on the service's own repo/image), so it falls back to port 10000 — revisit with t006 when the carrier's built image metadata is available; (2) a deploy of a linked service whose source has not built yet holds the prior release (`AwaitingBuildSource`/`holdPendingArtifact`) and its deploy row can stay pending until a build is selected — t006's fan-out selects the first build.
- **t006 (builds and fan-out):** `POST/GET /v1/build-sources/{id}/builds` and `GET …/builds/{buildId}` replace the named 404s. A build is the carrier's deploy row, and its id is that row's xid under `bld-` (`id.Rekind`), so a get is one indexed lookup. Trigger builds git sources only (Render). When a carrier deploy closes live, the reconciler's `BuildFanout` hands the build to `deploys.BuildSucceeded`. That deploys it (`new_commit`) to each linked service that has auto-deploy on, plus any service still waiting for the source's first build. Every deploy of a linked service selects the source's current build, and a restart keeps the serving build. A deploy from an unbuilt source is refused (`409`), and the build is written only while the row still links that source. Follow-ups: fan-out deploy rows carry no commit or image yet (t013 adds `buildId`); carrier build metering moved to t015.
