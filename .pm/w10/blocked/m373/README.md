# w10 · m373 — Service and deploy reads match Render's schema both ways; the rendertest allowlist excuses only signed-off divergences

**Worker:** worker10 **Goal:** a client generated from Render's pinned OpenAPI with required-field validation parses every bex service and deploy read, and `rendertest.Allowlist` no longer excuses omissions that exist only because the test fixtures run without production configuration. **Status:** blocked (t001–t009 done 2026-11-13/14; t010 waits on two gates: DoD 1 needs a Render capture or the user's sign-off for each open entry in § Findings, and DoD 3-4 need the production deploy that carries this milestone)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The rendertest suites run on production-shaped fixtures; drop the metadata entries that only fixtures reproduce — **DONE** | 1.5h | — |
| [t002](done/t002.md) | Every service read carries Render's required `buildPlan`, and a web service its `healthCheckPath` (absorbs w4/626) — **DONE** | 40m | t001 |
| [t003](done/t003.md) | The remaining serviceDetails entries: conform each, or record it with the user's sign-off — **DONE** | 1h | t002 |
| [t004](done/t004.md) | A Git-built service's deploys carry no `image` on any tenant read (absorbs w4/621) — **DONE** | 45m | t001 |
| [t005](done/t005.md) | `numInstances` appears only where Render's schema has it; a cron job's header has no Instances fact (absorbs w4/606) — **DONE** | 30m | t001 |
| [t006](done/t006.md) | ADR018 and the allowlist record what is left — **DONE** | 20m | t003, t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 40m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 6h10m**.

## Definition of done

1. `allowlistSize` (`lego/backend/internal/rendertest/rendertest_test.go`) equals the entries left, and those are only the 3 dry-run entries, the 6 event-detail entries (Render-required facts bex does not record) and entries whose ADR018 row carries the user's recorded sign-off. A full backend run with the test database (`bash scripts/ship-gates.sh`) passes and reports no unexercised entry.
2. Removing `region` from one production-configured service read in a scratch change fails the rendertest suite (shown once in § Findings, not committed).
3. On production, after the deploy that carries this milestone, as the QA user in `bex-canary`: `GET /v1/services/<id>` for a fresh Free web service with no health-check path returns `serviceDetails.healthCheckPath == ""` and `buildPlan == "starter"`; a worker, a cron job and a static site return `buildPlan`; the cron job and the static site return no `numInstances`; `GET /v1/services?limit=1` does the same per row; the cron job's header shows no Instances fact.
4. On production, a Git-built service's config-change deploy and its rollback carry no `image` on REST, GraphQL, MCP, event details, the deploy header and the Events row, and an image service's deploys still carry it.
5. ADR018 rows 295-296 and the allowlist paragraph state what is left and why.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w10` (2026-11-12/13), filed by `/pmx them all to w10` on 2026-11-13, re-checked at filing against `origin/main` `3c6f11dde`, item 2. Absorbs [w4/626](../../w4/done/626.md), [w4/621](../../w4/done/621.md) and [w4/606](../../w4/done/606.md) (`/qa-find-bugs w4` rounds 248-260, 2026-11-12/13), plus the brainstorm's count of the allowlist: 109 entries, of which 66 cite "Resource metadata omitted when unknown", 34 "Partial serviceDetails projection", 6 event details and 3 dry-run previews.
- **Render:** the pinned `render-public-api-1.json` (refreshed by w3/m367, `d8c71580b`): `webServiceDetails` requires `buildPlan` and `healthCheckPath`; every other member requires `buildPlan`; `cronJobDetails` and `staticSiteDetails` declare no `numInstances`; `deploy.image` is "Not present for Git-backed deploys".
- **Goal linkage:** Render parity first (ADR008); bex-api's Render-compatible REST contract (ADR006, ADR018).
- **Expected outcome:** a schema-strict Render client reads bex services and deploys without errors, and the allowlist again guards production's shape instead of excusing fixture gaps.
- **Why now:** the QA loop filed three schema findings in 24 hours that the validator could not catch because the allowlist excused them; the "Partial serviceDetails projection" row has no user sign-off, so the root AGENTS.md rule says amend it; and w3/m367 has just re-pinned Render's 2026-11-13 contract, so the burn-down runs on the current spec.
- **Open decision (recorded as the proposal's recommendation, not a user decision):** any serviceDetails entry t003 cannot make conform needs the user's sign-off in ADR018 before it stands; the recommendation is to conform every one.
- **Render parity:** included (REST, GraphQL, MCP and dashboard reads change).

## Findings

**2026-11-13 (`/loopx w10`).** The allowlist went from 109 entries to 40: 22 "Partial serviceDetails projection", 9 "Resource metadata omitted when unknown" (Postgres `version` ×5, `project` ×4), 6 event details, 3 dry-run previews.

- **What the 66 metadata entries were.** A production read (QA user, `GET /v1/postgres`, `/v1/key-value`, `/v1/projects`, 2026-11-13) carries `owner` (with email), `region`, `version`, `dashboardUrl` and `updatedAt`. The entries came from fixtures: controller-runtime's fake client drops `managedFields` unless `WithReturnManagedFields()` (bex reads an unannotated object's `updatedAt` from them), and most test servers run without `Region`, `DashboardURL` or a workspace store. Fix: `rendertest.Stamped` stamps a `managedFields` entry and both `fakeClient` helpers return them; `rendertest.Wrap` takes the server's unconfigured `Source`s (`unconfiguredSources(srv)` in the api suite; `AllSources` for the bare apps mux), so only a server that lacks a source is excused. `TestRenderConformance` now runs with a region and dashboard URL, and `TestResourceRESTMetadataUsesOwnWorkspaceAndAuthoritativeSources` with every source, so a dropped field fails there. Two tests that asserted an unannotated object omits `updatedAt` now expect the managedFields time (the no-source fallback stays covered in `resourcemeta`).
- **Postgres `version` (open).** `spec.version` empty means "operator default", and the view reads `status.currentVersion`, so a create without a version answers no `version` until the operator reports. This happens in production too. Stating it needs the operator's default in bex-api.
- **Postgres `project` (open).** Render's pinned `postgresDetail` lists `project` as required but defines no property for it; bex sends `projectId`. Its shape is unknowable from the contract; a Render capture (needs a Render account, w10/002's gate) settles it.
- **t003, the remaining serviceDetails entries** (temporarily removed and attributed to the tests that reproduce them):
  - `plan` (133 tests) and `url` (134): fixture Apps without `spec.plan` or `status.url`. `plan` is likely fixture-only but many tests key behavior on an empty plan, so it needs care; `url` is also missing on a create before the first live revision, which Render answers with the URL it assigned at creation — a real gap bex can close by deriving the URL from the slug, as it already does for a static site before its first deploy.
  - `envSpecificDetails` (179): an image service without a command omits it. Real in production; Render's shape for an image service with no command is not in the contract (needs a capture).
  - `runtime`/`env` (2): a build-source-linked service has no runtime. Real; same capture question.
  - `publishPath` (6) and `disk/id` (3, no control-plane store): fixture-shaped.
- **Not changed:** `logs.DeployProgress.Image` (`api/server.go:889`) still narrates a Git deploy's internal image in the deploy log stream; `AppView.Image` is unchanged.

**2026-11-14 (`/loopx w10`, t003).** The allowlist went from 40 to 34: 16 "Partial serviceDetails projection", 9 "Resource metadata omitted when unknown", 6 event details, 3 dry-run previews.

- **Fixture gaps closed.** `plan`: every service bex-api or a Blueprint creates carries a tier, so `rendertest.Stamped` gives an App with no recorded writer create's default (`starter`); `rendertest.KubectlApplied` keeps a hand-applied CR tierless. `publishPath`: six static-site fixtures lacked the publish path every create path requires. `disk/id`: the bare apps mux has no workspace resolver, so its create writes no control-plane disk row; the field is now part of `SourceWorkspaces`.
- **DoD 2 shown (scratch, not committed).** With `{"region", region}` removed from `renderServiceDetails`, `go test ./internal/api/ -run TestRenderConformance` fails on `/N/service/serviceDetails/region: property "region" is missing` for every listed service.
- **Open: private service `url`.** Render's `privateServiceDetails` has required `url` in every pinned spec since 2026-07-20. bex omits it on a docs-based 2026-07-26 reading with no live capture (`docs/render-artifacts/service-addresses.md`, ADR041 D4). The value Render sends is unknown. A web service whose platform subdomain is off and has no custom domain also has no `url`.
- **Open: image service `envSpecificDetails`.** Without a command there is none; with one, the block carries only the commands the official CLI clones (`dockerCommand`, `preDeployCommand`, a cron's `startCommand`), so it matches neither `dockerDetails` (which requires `dockerContext` and `dockerfilePath`) nor `nativeEnvironmentDetails`. Render's shape for `runtime: image` is not in the contract.
- **Open: `runtime`/`env`.** A build-source-linked service takes its runtime from the carrier App, and `view()` has no context to read it; the matching `envSpecificDetails` would also have to come from the carrier. A bex `buildpack` builder with no explicit runtime names no Render runtime; this is a bex extension.
- **What closes them:** a Render capture of a private service, an image service and a build-source-linked service (needs a Render API key, w10/002's gate), or the user's sign-off on each row in ADR018.
