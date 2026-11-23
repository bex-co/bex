# w6 · m565 — Each agent sandbox's billing line names its real size and its session

**Worker:** worker6 **Goal:** a person reading the billing page can tell what each agent sandbox charge was: its real CPU and memory, and the session (repository and branch) it ran for. Today they read `starter · ghcr.io/bex-co/bex-agent-sandbox · <uuid>`. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [An agent sandbox is labeled with its template's size, not `starter`](done/t001.md) — **DONE** | 45m | — |
| t002 | [Every agent sandbox's row names its session, even after the session moved on](done/t002.md) — **DONE** | 45m | — |
| t003 | [Past rows read their real size from the recorded weight](done/t003.md) — **DONE** | 45m | t001 |
| t004 | [Render parity](done/t004.md) — **DONE** | 30m | t002, t003 |
| t005 | [Simplify](done/t005.md) — **DONE** | 20m | t004 |
| t006 | [Test coverage](done/t006.md) — **DONE** | 45m | t004 |
| t007 | [Closeout](done/t007.md) — **DONE** | 10m | t005, t006 |

## Definition of done

- A new agent sandbox on `agent-4c-8g` is labeled `4c-8g · ghcr.io/bex-co/bex-agent-sandbox`, and one on `agent` is labeled `2c-4g · …`, on the billing page and on REST, GraphQL and MCP usage reads.
- Every agent sandbox row that `agent_session_sandboxes` links to a session reads `<repo> (<branch>)` (or `agent session <id>` with no repository), whether or not the session still names that sandbox.
- November's rows in the `bex` workspace read their real size: on production, no agent sandbox row with a recorded weight of 2422 or 4845 milli says `starter`.
- Amounts are unchanged.

## Source + Goal linkage

- **Source:** the recommendation the user approved on 2026-11-22 ("其他的按照你推荐的修"), from reading the `bex` workspace's billing page with them.
  - `createAgentSandbox` stamps `plan := PlanStarter` whatever the template (`lego/backend/internal/sandbox/service.go:1257`), and `starter` is Render's 0.5 CPU / 512 MB plan (`planResources`, `lego/backend/internal/sandbox/models.go`). On production 237 of the `bex` workspace's agent sandboxes ran 4 CPU / 8 GiB (weight 4845) and 284 ran 2 CPU / 4 GiB (2422), all labeled `starter`.
  - `SandboxUsageMetadata` (`lego/backend/internal/store/resource_names.go:145`) names a sandbox from `agent_sessions.sandbox_id` and `agent_session_dispatches.previous_sandbox_id` only, so a sandbox reaped after its session moved on falls back to the plan · image label. `agent_session_sandboxes` links 301 of November's 355.
- **Goal linkage:** ADR023 usage metering and ADR030 pricing: a bill a customer can read and check ([ADR008](../../../docs/ADR008-vision.md)).
- **Expected outcome:** the billing page's agent sandbox lines say what ran and for which session, so a cost like November's $231 can be traced without a database query.
- **Why now:** the same page is where m563 and m566's savings will be checked, and today it hides which sessions cost what.
- **Render parity:** included. Render's billing page lists each sandbox charge by the sandbox; the label must be the same on REST, GraphQL, MCP and the dashboard.

## Unblocked (2026-11-23)

The gate cleared by itself (verified evidence, no user decision): the newest pin on `origin/main`, `cded300699` (`chore(deploy): pin platform images to 9fdf7e0a1171`, 2026-11-23 17:33Z), carries `a8d6ea3b12` (`git merge-base --is-ancestor a8d6ea3b12 9fdf7e0a1171`). Next: t003's acceptance check and the definition of done on production, as the workstream line said, then t007.

## Production check (2026-11-23, t003 and the definition of done)

Pin `9fdf7e0a1171`: every `bex-system` Deployment ran `bex-operator@sha256:df6e69bd31bf…`, the digest that pin writes, with `bex-api` rolled at 17:38Z. Read as the QA user (`bash scripts/qa-login.sh`, logged out afterwards) in workspace `bex`: REST `GET /v1/usage?ownerId=…`, GraphQL `usage(ownerId:)`, and MCP `get_usage` through a temporary API key bound to `bex` (revoked afterwards), against the `bex-db` replica read-only.

- **Past rows.** November's `usage_hourly` holds 366 `bex` agent sandboxes metered as `starter` whose `sandbox_meter_states.weight_milli` is 2422 (121) or 4845 (245). On all three reads none of them says `starter` on its name, rows or charge lines: every row and charge line reads `2c-4g` (2422) or `4c-8g` (4845), and the three reads agree sandbox for sandbox. No sandbox in the workspace reads `starter` anywhere. The billing page, every resource expanded, shows 245 `Sandbox compute 4c-8g`, 123 `2c-4g` and one `1c-2g` line, and no `starter` under Sandboxes.
- **Session names.** The 310 of them that `agent_session_sandboxes` (or a dispatch's previous sandbox) links to a session read exactly that session's `<repo> (<branch>)` or `agent session <id>` on all three reads. Seven whose sessions were deleted read the `agent session <id>` name retained while they ran; the other 49 read `2c-4g · …`/`4c-8g · …` or `4c-8g sandbox`.
- **New sandboxes.** A sandbox created after the deploy is stamped with its template's size: two on `agent` (17:51Z and 17:59Z, m564's disposable session) metered `2c-4g` at 2422 with retained labels `2c-4g · ghcr.io/bex-co/bex-agent-sandbox` and `2c-4g · …` (its snapshot image), and the `rt-unify-duplicates` automation's on `agent-4c-8g` (`bf7127e4-59a3-40af-80d5-bf57461c7453`, 18:30Z, session `ags-dc28cbedfl0s73a7eab0`) metered `4c-8g` at 4845 with retained label `4c-8g · ghcr.io/bex-co/bex-agent-sandbox` and `usage_hourly` tier `4c-8g`. On REST, GraphQL and MCP its rows and charge lines read `4c-8g` and its name `bex-co/bex (bex-agent/ags-dc28cbedfl0s73a7eab0)`; the two `agent` ones read `2c-4g`.
- **Amounts.** Each of the 366 costs its `usage_hourly` `sandbox_compute_seconds` quantity at $0.0895 per vCPU-hour to the cent, with its memory and CPU rows `unbilled`; together $234.72 of the Sandboxes category's $234.77, and the month to date reads $415.98 on REST, GraphQL, MCP and the billing page. `usage_hourly` still says `starter` for them.
- **Reproduce.** The usage reads above with the cookie jar `qa-login.sh` prints; on the replica, `SELECT service_id, tier, sum(quantity) FROM usage_hourly WHERE workspace_id = '<bex>' AND resource_kind = 'sandbox' AND kind = 'sandbox_compute_seconds' AND window_start >= '2026-11-01' GROUP BY 1, 2` joined to `sandbox_meter_states` and `agent_session_sandboxes`.
