# w2 · m256 — Blueprint onboarding: create, plan and approve a Blueprint as the Render CLI does

**Worker:** worker2 **Goal:** the Render CLI v2.35 `ea blueprints create`, `plan`, `approve` and `update` commands work against bex: creating a Blueprint plans its first sync without applying it, a sync can be re-planned, and approving applies the planned sync with any manual env var values. **Status:** in progress — t001–t006, t008–t010 done; t007 (unmodified CLI v2.35 walk on production) next; t012 (dashboard Approve, found live 2026-11-03), then t011

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture Render's Blueprint onboarding contract and amend ADR049 — **DONE** | 45m | — |
| t002 | POST /v1/blueprints accepts Render's CreateBlueprintRequest and plans the first sync — **DONE** | 1.5h | t001 |
| t003 | POST /v1/blueprints/{id}/syncs plans a sync without applying — **DONE** | 1h | t002 |
| t004 | POST /v1/blueprints/{id}/approve applies the planned sync with manual env vars — **DONE** | 1.5h | t003 |
| t005 | existingResources: adopt and create_new — **DONE** | 1h | t002 |
| t006 | GraphQL and MCP plan and approve — **DONE** | 1h | t004, t005 |
| t007 | Unmodified Render CLI v2.35.0 walk on dev-2 | 45m | t006, w2/065 |
| t008 | Render parity — **DONE** | 15m | t007 |
| t009 | Simplify — **DONE** | 15m | t008 |
| t010 | Test coverage — **DONE** | 30m | t008 |
| t012 | Dashboard: a created Blueprint shows its plan and an Approve action, and Sync cannot apply it past the plan | 1h | t006 |
| t011 | Closeout | 10m | t009, t010, t012 |

## Definition of done

- The unmodified Render CLI v2.35.0 pointed at dev-2 runs `ea blueprints create` (plan shown, nothing created), `ea blueprints approve` (resources created), `ea blueprints plan` after a repo change (`planned` with the diff, or `no_changes`), and `ea blueprints update --branch`, each exiting 0 with Render's output shape.
- `POST /v1/blueprints` accepts Render's `CreateBlueprintRequest` and returns `{blueprint, planningOutcome, notices, sync}`; the existing bex create body keeps working.
- `POST /v1/blueprints/{id}/syncs` plans without applying; `POST /v1/blueprints/{id}/approve` applies only the planned sync, refuses a stale or already-applied one by name, and requires every manual env var the plan lists.
- `existingResources: adopt` behaves as Render documents (matching resources come under the Blueprint); `create_new` is refused by name and tracked as w2/069.
- GraphQL and MCP expose plan and approve with the same semantics; ADR049 and ADR018 record the contract.
- The dashboard page at a planned Blueprint's `dashboardUrl` shows the planned actions and an **Approve** control (with required manual env var fields). While the Blueprint is `created`, no manual sync (dashboard Sync, REST `/sync`, GraphQL `syncBlueprint`, MCP) can apply it (t012).

## Source + Goal linkage

- **Source:** w2/m251 round-5 drift (2026-11-02). Render CLI v2.35.0 (released 2026-11-02) makes `render ea blueprints create|plan|approve|update` public alpha. Its regenerated client (`pkg/client/blueprints/blueprints_gen.go`) calls `POST /blueprints` with `CreateBlueprintRequest{name, ownerId, source, autoSync, existingResources}` and expects `CreateBlueprintResponse{blueprint, planningOutcome: planned|no_changes|failed, notices, sync}`; `POST /blueprints/{id}/syncs` → `CreateBlueprintSyncResponse`; `POST /blueprints/{id}/approve` with `ApproveBlueprintRequest{manualEnvVars}` → `ApprovedSync`; `PATCH /blueprints/{id}` with `branch`. None of these is in Render's public OpenAPI. bex today: `POST /v1/blueprints` with its own body (`repo`, `branch`, `path`, `envVarValues`, `confirm`), `POST /v1/blueprints/{id}/sync` (applies at once), no approve (`lego/backend/internal/apps/rest.go` `registerBlueprintRoutes`).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); Render's CLI is bex's CLI (DO_NOT_DO: no first-party CLI; gaps become bex-api work).
- **Expected outcome:** users onboard a repo's Blueprint from the terminal with Render's plan → approve flow, and see what will change before anything is created.
- **Why now:** the commands shipped upstream the same day as this drift round; the next `bex-cli` pin bump inherits them, and today they fail against bex. PATCH `branch` alone is filed as w2/065 (public-spec change); this milestone depends on it for `update --branch`.
- **Render parity included:** new REST/GraphQL/MCP behavior.

## Work log — 2026-11-03

- **Contract (t001):** [ADR049 § Blueprint onboarding](../../../docs/ADR049-render-yaml-parity.md#blueprint-onboarding-plan-then-approve-w2m256-2026-11-03), from the CLI v2.35.0 client (`pkg/client/blueprints/blueprints_gen.go`, `internal/fakes/renderapi/blueprints.go`).
- **Create/plan/approve (t002–t004):** `POST /v1/blueprints` with Render's body saves the Blueprint `created` and plans (201 `{blueprint, planningOutcome, notices, sync}`); `POST …/syncs` plans (201 with a sync, 200 `no_changes`); `POST …/approve {manualEnvVars}` applies the newest planning attempt at its commit, through `runSync`'s reviewed-source pin, and admission turns the planned row into the run (same id). Auto-sync skips `created`. Planned rows are marked by `plan_path` (migration 0185), so an ordinary run or a busy refusal never stands in for the plan. Reconnecting supersedes an earlier connection's waiting plan.
- **Manual env vars:** required = `sync: false` keys without a value on services the plan creates; values keyed per service (`BlueprintEnvVarValueKey`), so two services can take different values for one key.
- **DoD refinement (2026-11-03, worker2):** `create_new` is an opt-in variant of Render's default `adopt`; per the parity rule it ships as its own item (w2/069) instead of holding the default back.
- **t006:** GraphQL `createPlannedBlueprint` / `planBlueprintSync` / `approveBlueprint`, MCP `create_planned_blueprint` / `plan_blueprint_sync` / `approve_blueprint` (MCP inventory 197).
- **Review fixes (t009):** structural plans answer 503 instead of a false `no_changes`; approve reads the applied run by id; name uniqueness re-checked at approval; oversized create bodies answer 413; empty approve body accepted.
- **t010:** `blueprint_onboarding_test.go` (create → approve with seeded per-service value; failed plan refused; refusals; REST status codes; plan unavailable), `TestPlannedBlueprintSyncsPG`, `TestPlannedBlueprintSupersedeAndBusyPG`.
