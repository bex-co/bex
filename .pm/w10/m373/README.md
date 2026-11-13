# w10 · m373 — Service and deploy reads match Render's schema both ways; the rendertest allowlist excuses only signed-off divergences

**Worker:** worker10 **Goal:** a client generated from Render's pinned OpenAPI with required-field validation parses every bex service and deploy read, and `rendertest.Allowlist` no longer excuses omissions that exist only because the test fixtures run without production configuration. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The rendertest suites run on production-shaped fixtures; drop the metadata entries that only fixtures reproduce | 1.5h | — |
| [t002](t002.md) | Every service read carries Render's required `buildPlan`, and a web service its `healthCheckPath` (absorbs w4/626) | 40m | t001 |
| [t003](t003.md) | The remaining serviceDetails entries: conform each, or record it with the user's sign-off | 1h | t002 |
| [t004](t004.md) | A Git-built service's deploys carry no `image` on any tenant read (absorbs w4/621) | 45m | t001 |
| [t005](t005.md) | `numInstances` appears only where Render's schema has it; a cron job's header has no Instances fact (absorbs w4/606) | 30m | t001 |
| [t006](t006.md) | ADR018 and the allowlist record what is left | 20m | t003, t004, t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 40m | t007, t008 |
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

None yet.
