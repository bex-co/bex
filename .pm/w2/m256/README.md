# w2 · m256 — Blueprint onboarding: create, plan and approve a Blueprint as the Render CLI does

**Worker:** worker2 **Goal:** the Render CLI v2.35 `ea blueprints create`, `plan`, `approve` and `update` commands work against bex: creating a Blueprint plans its first sync without applying it, a sync can be re-planned, and approving applies the planned sync with any manual env var values. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture Render's Blueprint onboarding contract and amend ADR049 | 45m | — |
| t002 | POST /v1/blueprints accepts Render's CreateBlueprintRequest and plans the first sync | 1.5h | t001 |
| t003 | POST /v1/blueprints/{id}/syncs plans a sync without applying | 1h | t002 |
| t004 | POST /v1/blueprints/{id}/approve applies the planned sync with manual env vars | 1.5h | t003 |
| t005 | existingResources: adopt and create_new | 1h | t002 |
| t006 | GraphQL and MCP plan and approve | 1h | t004, t005 |
| t007 | Unmodified Render CLI v2.35.0 walk on dev-2 | 45m | t006, w2/065 |
| t008 | Render parity | 15m | t007 |
| t009 | Simplify | 15m | t008 |
| t010 | Test coverage | 30m | t008 |
| t011 | Closeout | 10m | t009, t010 |

## Definition of done

- The unmodified Render CLI v2.35.0 pointed at dev-2 runs `ea blueprints create` (plan shown, nothing created), `ea blueprints approve` (resources created), `ea blueprints plan` after a repo change (`planned` with the diff, or `no_changes`), and `ea blueprints update --branch`, each exiting 0 with Render's output shape.
- `POST /v1/blueprints` accepts Render's `CreateBlueprintRequest` and returns `{blueprint, planningOutcome, notices, sync}`; the existing bex create body keeps working.
- `POST /v1/blueprints/{id}/syncs` plans without applying; `POST /v1/blueprints/{id}/approve` applies only the planned sync, refuses a stale or already-applied one by name, and requires every manual env var the plan lists.
- `existingResources: adopt` and `create_new` behave as Render documents (adopt brings matching resources under the Blueprint; create_new suffixes every managed resource name).
- GraphQL and MCP expose plan and approve with the same semantics; ADR049 and ADR018 record the contract.

## Source + Goal linkage

- **Source:** w2/m251 round-5 drift (2026-11-02). Render CLI v2.35.0 (released 2026-11-02) makes `render ea blueprints create|plan|approve|update` public alpha. Its regenerated client (`pkg/client/blueprints/blueprints_gen.go`) calls `POST /blueprints` with `CreateBlueprintRequest{name, ownerId, source, autoSync, existingResources}` and expects `CreateBlueprintResponse{blueprint, planningOutcome: planned|no_changes|failed, notices, sync}`; `POST /blueprints/{id}/syncs` → `CreateBlueprintSyncResponse`; `POST /blueprints/{id}/approve` with `ApproveBlueprintRequest{manualEnvVars}` → `ApprovedSync`; `PATCH /blueprints/{id}` with `branch`. None of these is in Render's public OpenAPI. bex today: `POST /v1/blueprints` with its own body (`repo`, `branch`, `path`, `envVarValues`, `confirm`), `POST /v1/blueprints/{id}/sync` (applies at once), no approve (`lego/backend/internal/apps/rest.go` `registerBlueprintRoutes`).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); Render's CLI is bex's CLI (DO_NOT_DO: no first-party CLI; gaps become bex-api work).
- **Expected outcome:** users onboard a repo's Blueprint from the terminal with Render's plan → approve flow, and see what will change before anything is created.
- **Why now:** the commands shipped upstream the same day as this drift round; the next `bex-cli` pin bump inherits them, and today they fail against bex. PATCH `branch` alone is filed as w2/065 (public-spec change); this milestone depends on it for `update --branch`.
- **Render parity included:** new REST/GraphQL/MCP behavior.
