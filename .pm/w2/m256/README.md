# w2 · m256 — Blueprint onboarding: create, plan and approve a Blueprint as the Render CLI does

**Worker:** worker2 **Goal:** the Render CLI v2.35 `ea blueprints create`, `plan`, `approve` and `update` commands work against bex: creating a Blueprint plans its first sync without applying it, a sync can be re-planned, and approving applies the planned sync with any manual env var values. **Status:** todo — all done but t014 (implemented, awaiting a production re-check once deployed) and t011

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture Render's Blueprint onboarding contract and amend ADR049 — **DONE** | 45m | — |
| t002 | POST /v1/blueprints accepts Render's CreateBlueprintRequest and plans the first sync — **DONE** | 1.5h | t001 |
| t003 | POST /v1/blueprints/{id}/syncs plans a sync without applying — **DONE** | 1h | t002 |
| t004 | POST /v1/blueprints/{id}/approve applies the planned sync with manual env vars — **DONE** | 1.5h | t003 |
| t005 | existingResources: adopt and create_new — **DONE** | 1h | t002 |
| t006 | GraphQL and MCP plan and approve — **DONE** | 1h | t004, t005 |
| t007 | Unmodified Render CLI v2.35.0 walk on dev-2 — **DONE** | 45m | t006, w2/065 |
| t008 | Render parity — **DONE** | 15m | t007 |
| t009 | Simplify — **DONE** | 15m | t008 |
| t010 | Test coverage — **DONE** | 30m | t008 |
| t012 | Dashboard: a created Blueprint shows its plan and an Approve action, and Sync cannot apply it past the plan — **DONE** | 1h | t006 |
| t013 | Planned Blueprint page: the reconnection note on a revived row, the right status chip, and approve-not-sync empty copy — **DONE** | 30m | t012 |
| t014 | A recreated Blueprint whose plan finds no changes is stuck in created, and planning ignores resources deleted since the last sync — implemented 2026-11-08, awaiting production re-check | 1.5h | t006 |
| t011 | Closeout | 10m | t009, t010, t012, t013, t014 |

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

## Blocked (2026-11-03)

t001–t006, t008–t010, t013 done; t012 implemented (its production check rides with t007); t007 (unmodified Render CLI v2.35.0 walk) and t011 closeout open. Remaining gate: user: allow building/running the unmodified upstream Render CLI v2.35.0 (the auto-mode classifier denied cloning and building github.com/render-oss/cli in this session), or run `ea blueprints create → approve → plan → update --branch` against production as the QA user yourself; the walk also covers t012's production check. Run `/unblock w2` to decide. The milestone returns to the queue once the gate clears.

## 解阻复核（2026-11-04，复核证据，待用户决定）

- **不用克隆上游的替代路径：** 按 `lego/cli/UPSTREAM_RENDER_CLI.md` 的常规流程，把 `bex` launcher 的 pin 从 v2.29.0 升到 v2.35.0（在 `lego/cli` 里 `go get github.com/render-oss/cli@<v2.35.0 commit>`，走 Go module proxy）。launcher 直接用上游命令实现，没有 fork，`bex ea blueprints …` 就是原版 v2.35.0 的代码路径。顺带客户安装的 bex CLI 也会获得这组命令。
- **生产认证：** 用 `bex login` 的 device flow，配合 `scripts/qa-login.sh --login-challenge` 以 QA 账号完成。
- **仍需用户决定：** 是否允许走 pin 升级这条路（auto-mode 也可能拦 `go get`），或者放行直接编译上游。

## 解阻决定（2026-11-04，来源：用户决定 D3=A）

- **决定：** 不克隆上游仓库，按 `lego/cli/UPSTREAM_RENDER_CLI.md` 的常规流程，把 `bex` launcher 的 render-oss/cli pin 从 v2.29.0 升到 v2.35.0（在 `lego/cli` 里 `go get github.com/render-oss/cli@<v2.35.0 commit>`），完成该文档列出的每项 re-diff 检查。
- **验收方式：** 用升级后的 `bex ea blueprints create / approve / plan / update --branch` 对生产走一遍 t007（QA 账号，`bex login` 的 device flow 配合 `scripts/qa-login.sh --login-challenge` 完成；资源加 `qa-` 前缀，结束后删除），同时覆盖 t012 的生产检查。
- **理由：** launcher 直接使用上游命令实现（无 fork），代码路径与原版 v2.35.0 一致；顺带让客户安装的 bex CLI 获得这组命令。
- **worker 下一步：**
  1. 升 pin，跑 `bash scripts/bex-cli-validate.sh` 和 `cd lego/cli && go test ./...`。
  2. 更新 `docs/cli-compatibility-checklist.md` 的 v2.35 条目。
  3. 发布前在生产走完 t007，证据写入 t007、t012。
  4. 按 t011 收尾。
- **剩余条件：** 如果 `go get` 或生产写操作被 auto-mode 拦截，重新 blocked，并写明需要用户放行的具体动作。

## Pin bump — 2026-11-04 (local, not yet shipped)

- `lego/cli` now pins render-oss/cli v2.35.0 (`24ac2d60fdb2`, `v1.1.3-0.20261102222634-24ac2d60fdb2`, Go 1.27.3). `bex-cli-validate.sh`, `go test ./...` and golangci-lint all pass. The UPSTREAM_RENDER_CLI.md re-diff checks are recorded in `docs/cli-compatibility-checklist.md`.
- Sandboxes left early access upstream (v2.32.0, no alias): `bex ea sandboxes` is now `bex sandboxes`. The launcher tests and the branding overlay follow it.
- **For t007:** the v2.35 CLI always sends `existingResources`, defaulting to `create_new` (served since w2/m261). Pass `--existing-resources adopt` to walk Render's adopt path.
- Server gaps the new CLI exposes, filed as **w2/m263**: idle timeout, `--timeout=0`, plan names, named refusals for groups and images, and the `ea blueprints` next-step hints.

## t007 production walk — 2026-11-05 (stopped at approve)

- **Login:** `bex login` (pinned v2.35.0) device flow, completed with `qa-login.sh --login-challenge` in an isolated `BEX_CLI_CONFIG_DIR`. Workspace `tea-daif693dqjvc73e7as3g`.
- **create, passed:** `bex ea blueprints create --repo https://github.com/bex-co/bex --branch main --path examples/static-site/render.yaml --name qa-20261105-bp --existing-resources adopt -o json` exited 0 with Render's shape: `{blueprint{status:"created"}, planningOutcome:"planned", notices:[], sync{rawPlan.actions:[create service static-site], requiredManualEnvVars:[]}}`. No service existed before approve.
- **approve, refused by the CLI before any request (exit 1):** `invalid Blueprint ID "blp-db136288mmqc73d4hpug": Blueprint IDs start with exs- followed by 20 lowercase letters or digits`. Upstream v2.35.0 validates Blueprint ids client-side, so `approve`, `plan` and `update` cannot address any bex Blueprint. Filed as **w2/m269** (expose `exs-` public ids); t007 resumes after it.
- **Also seen:**
  - `meta.next` says `render ea blueprints approve …` (m263 t005).
  - `workspace current` with none set says ``Use `render workspace set` ``: a backtick form the overlay misses (added to m263 t005).
- **Cleanup:** the Blueprint was deleted (REST DELETE 204, GET 404).

## Blocked (2026-11-05): waiting for a production deploy

Everything implementable is shipped. What remains is the production walk, and production is still pinned to `c8bbc07cbeeb`, which predates this work. Every later deploy run is superseded at write-back: run `0505f4532` succeeded its tests, then skipped build and deploy. That starvation is owned by **w6/m156** (t003, decision (a)).

**Release condition:** the `chore(deploy): pin platform images to <sha>` commit on main reaches a revision containing `eaf967d80` (m269). Then the worker runs the walk listed above as the next step.

## 解阻复核（2026-11-05，来源：复核证据）

- **卡点已消失：** 生产固定版本已推进到 `92ff0029334d`（`bab5d417e`）。它包含 t018（`c071550db`）、m269（`eaf967d80`）和 m263（`21175a4d2`）。以 QA 会话调用 `GET /v1/blueprints` 返回的已是 `exs-…`。
- **worker 下一步：** 按上文的生产验收步骤直接执行，无需用户决定。
