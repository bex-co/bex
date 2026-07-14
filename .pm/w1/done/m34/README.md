# w1 · m34 — Build filters: `buildFilter.paths` + `ignoredPaths`

**Worker:** worker1 **Goal:** Render's `buildFilter` (glob `paths`/`ignoredPaths` deciding whether a push triggers an auto-deploy) exists end-to-end: CRD field, webhook glob matching, settable/readable on REST/GraphQL/MCP + dashboard. **Status:** DONE (2026-07-14) — CRD field, doublestar webhook matcher composed with `rootDirMatches`, and REST/GraphQL/MCP + dashboard Build & Deploy editors all shipped; field shape verified against Render's OpenAPI (top-level, not `serviceDetails`).

## Tasks (in order)

| id   | title                                                             | est | depends_on |
| ---- | ----------------------------------------------------------------- | --- | ---------- |
| t001 | CRD: `spec.buildFilter{paths[],ignoredPaths[]}` + codegen — **DONE** | 30m | —          |
| t002 | Webhook: glob matching in the push path-filter — **DONE**         | 45m | t001       |
| t003 | REST: create + `PATCH` + read-back (Render shape) — **DONE**      | 40m | t001       |
| t004 | GraphQL `setBuildFilter` + create arg; MCP tool + args — **DONE** | 40m | t003       |
| t005 | Dashboard: Build & Deploy rows for paths/ignoredPaths — **DONE**  | 45m | t004       |
| t006 | Render parity — **DONE**                                          | 30m | t002, t005 |
| t007 | Simplify — **DONE**                                               | 30m | t006       |
| t008 | Test coverage — **DONE**                                          | 45m | t006       |
| t009 | Closeout — **DONE**                                               | 15m | t008       |

## Definition of done

A push touching only `ignoredPaths`-matched files does not open a deploy; a push matching `paths` does (empty `paths` = everything, Render's semantics). `buildFilter` is settable at create and via update, and readable back, on REST, GraphQL, MCP, and the dashboard Build & Deploy section — field shape verified against Render's OpenAPI.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker to work on until feature parity` 2026-07-14 (item 3); `docs/ADR018-render-parity.md` "Remaining `PATCH` field `buildFilter` is not editable — ◐, low" (understated: zero hits in `lego/` — the field is entirely unbuilt).
- **Goal linkage:** GOAL #3 (git push to deploy) precision + Render parity; the general form of w1/m18's rootDir webhook path-scoping.
- **Expected outcome:** monorepo users stop getting spurious deploys from unrelated paths; a named ◐ ledger row closes across all four surfaces.
- **Why now:** w1 is down to m33; this is the natural sibling of m18's `rootDirMatches` machinery while it's still fresh. Render parity task included — all-surface change.

## Completion notes (2026-07-14)

- **t001 — CRD:** `AppSpec.BuildFilter *BuildFilterSpec{Paths[],IgnoredPaths[]}` (`lego/types/v1alpha1/app_types.go`), mirroring the `Autoscaling` optional-nested-struct precedent; deepcopy + CRD manifest regenerated (`make generate manifests`). Globs are repository-root-relative and compose with `rootDir`, documented on the field.
- **t002 — Webhook matcher:** added `github.com/bmatcuk/doublestar/v4` (deliberate choice — `path.Match` has no `**`; doublestar matches Render's documented `*`/`**`/`?`/`[class]` dialect). `buildFilterMatches` gates `redeployMatching` **after** `rootDirMatches` (AND composition): a push deploys iff a changed file matches an include glob (or `paths` empty) **and** is not ignored — ignored wins over included, matching Render. Fails open when the payload carries no changed-path data.
- **t003–t004 — surfaces:** threaded `buildFilter` through the same seams as `rootDir` — `CreateRequest`/`specFromCreate`/`applyCreateToSpec`, `AppView`/`view`, and a `SetBuildFilter` verb (no `restartedAt` bump — the filter only changes future pushes, it doesn't rebuild). One neutral `BuildFilterView{paths,ignoredPaths}` type serves REST body/response, GraphQL, MCP, and blueprint (identical Render shape everywhere). REST `POST`/`PATCH` top-level `buildFilter` (verified top-level, **not** under `serviceDetails`, against Render's `servicePOST`/`servicePATCH` OpenAPI); GraphQL `createService(buildFilter:)` + `setBuildFilter`; MCP `create_web_service` arg + `set_build_filter`; blueprint `render.yaml` `buildFilter`. Malformed globs rejected at the boundary (`store.ValidGlob`); an all-empty filter canonicalizes to nil. `apps.SetBuildFilter` → `build_filter_changed` in the service activity feed.
- **t005 — dashboard:** Settings → Build & Deploy gains a **Build Filters** editor (`build-deploy-section.tsx`): two `PathList` sub-editors (Included/Ignored Paths) with add/remove rows and one bulk `setBuildFilter` save, mirroring the static-site RoutesEditor. Read via the `server` query's new `buildFilter { paths ignoredPaths }` selection; `useBuildFilter` hook + `SetBuildFilter` mutation. GraphQL types regenerated offline (backend `TestDumpGraphQLSchema` → `SCHEMA_JSON` codegen). Git-sourced services only. en/zh locales added; `yarn test`/`yarn lint` green.
- **t006 — parity:** ADR018 gained a dedicated **Build Filters** row (✅ all four surfaces) and the stale "buildFilter not editable — ◐" note in the plan/type row was closed; ADR006's blueprint field-map moved `buildFilter` from "ignored" to a ✅ row. **Conscious divergence documented:** Render evaluates build filters independently of the root directory; bex ANDs `buildFilter` after its `rootDir` prefix filter (per t002's "composed with rootDirMatches" intent) — a deliberate narrowing consistent with bex's existing rootDir-as-autodeploy-filter. The task README said `serviceDetails.buildFilter`; the OpenAPI confirms **top-level**, and bex places it top-level (a sibling of `rootDir`) — a doc drift in the task, resolved to the OpenAPI-correct placement.
- **t007 — simplify:** 4-agent pass (reuse/simplification/efficiency/altitude). One real fix applied: the new `gqlStrList` duplicated the existing `gqlutil.StringList` (used 9+ places) — replaced. Everything else confirmed already clean or consistent with established patterns (JSON-stringify dirty-check mirrors RoutesEditor; the `toBuildFilter` null-filter is required for TS narrowing of GraphQL `[String]` and mirrors `toStaticRoutes`; `matchesAnyGlob`/`cleanGlobs`/`getBuildFilter` are DRY helpers; `buildFilterArg` carries MCP jsonschema tool descriptions). Altitude confirmed faithful mirroring of the rootDir precedent at the right depth.
- **t008 — tests:** `webhook_test.go` `TestBuildFilterMatches` (the DoD decision table — ignored-only ⇒ no deploy, paths-match ⇒ deploy, empty ⇒ unchanged, both ⇒ ignored-wins, globstar, leading-slash) + `TestRedeployMatchingComposesBuildFilterWithRootDir` (rootDir-composed end-to-end); new `buildfilter_test.go` (REST/GraphQL/MCP/blueprint create+set+read parity, PATCH, clearing, image-backed rejection, bad-glob + blank-entry validation, arrays-not-null); `validators_test.go` `TestValidGlob`; dashboard `build-deploy-section.test.tsx` (renders editor, add+save, remove+save, shows existing globs). Full `make test` (operator, envtest), backend `go test ./...`, and dashboard `yarn test` all green; backend lint 0 issues.
</content>
