# w8 · m137 — Inbound IP rules for web services, static sites and environments are a Scale/Enterprise workspace feature, as on Render

**Worker:** worker8 **Goal:** Setting a non-empty inbound IP allowlist on a web service, static site or environment is refused unless the workspace is on Scale or Enterprise, as on Render. Postgres and Key Value allowlists stay available on every workspace plan. **Status:** done (2026-10-29)

## Tasks (in order)

| id   | title                                                                            | est | depends_on                 |
| ---- | -------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Admission: refuse non-empty service/static/environment allowlists below Scale     | 45m | —                          |
| t002 — **DONE** | Blueprint, clone and create paths carry the same refusal                          | 30m | w8/m137/t001               |
| t003 — **DONE** | Existing rules on Hobby/Pro workspaces and plan downgrades                        | 30m | w8/m137/t001               |
| t004 — **DONE** | Dashboard: the IP rules sections on Hobby/Pro say "Scale plan"                    | 30m | w8/m137/t001               |
| t005 — **DONE** | Render parity: REST/GraphQL/MCP/dashboard and ADR018 agree                        | 15m | w8/m137/t002, w8/m137/t003, w8/m137/t004 |
| t006 — **DONE** | Simplify the changed code                                                        | 15m | w8/m137/t005               |
| t007 — **DONE** | Test coverage: plan × resource type × empty/non-empty, downgrade, datastores     | 30m | w8/m137/t005               |
| t008 | Close out the milestone — **DONE 2026-10-29** | 30m | w8/m137/t006, w8/m137/t007 |

## Definition of done

Live on production after the release pipeline deploys bex-api and the dashboard, in a Hobby workspace (`bex-canary`), with owned Free fixtures deleted by ID with 404 proofs:

- `bex services create --type web_service --plan free --image docker.io/traefik/whoami:v1.10.3 --env-var PORT=80 --ip-allow-list "cidr=203.0.113.0/24,description=x" --confirm -o json` exits 1 with a 400 naming the Scale plan, and creates nothing.
- `bex services update <id> --ip-allow-list …` on an existing Hobby web service → the same 400. An empty list is accepted.
- `PATCH /v1/environments/{id}` with a non-empty `ipAllowList` → the same 400.
- Control: `bex kv update <id> --ip-allow-list …` on a Free Key Value → accepted (datastore rules stay on every plan).
- The dashboard IP rules sections on Hobby show the Scale-plan hint.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep g7h2, 2026-10-23 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, platform pin `29d63e1f7bea`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`, GraphQL `plan: "hobby"`), human device login. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). [render.com/docs/inbound-ip-rules](https://render.com/docs/inbound-ip-rules): web services and static sites, and environment- and workspace-level rules, are available to "Scale" and "Enterprise" orgs only; Render Postgres and Key Value rules are available to "All workspaces". Free is "not eligible for service/environment/workspace-level rules". The same restriction class as w8/m127 (free SMTP), w8/m130 and w8/m131, which shipped. ADR018:154 records no plan gate.
- **Expected outcome:** workspace plan features match Render's. Pricing parity ([ADR030](../../../../docs/ADR030-pricing.md): Hobby/Pro/Scale) is meaningful only if Scale-only features are Scale-only.
- **Why now:** minor-to-major (pricing/feature parity, a Scale feature given away on Hobby). Render parity task included: REST/GraphQL/MCP/dashboard and Blueprint surfaces.

## Evidence (sweep g7h2)

```text
GraphQL (dashboard session) query { workspaces { id name plan } } → bex-canary tea-daif693dqjvc73e7as3g plan "hobby"
$ bex services create --type web_service --name qa-20261023-g7h2-ipal --image docker.io/traefik/whoami:v1.10.3 --plan free \
    --env-var PORT=80 --ip-allow-list "cidr=203.0.113.0/24,description=doc-only" --confirm -o json
exit 0 → ipAllowList [{"cidrBlock":"203.0.113.0/24","description":"doc-only"}]
deploy live; curl https://qa-20261023-g7h2-ipal.onbex.co/ from a non-listed client → 403 (enforced)
```

Environments created in the same workspace in sweeps b1c3/c8d1 also accepted `ipAllowList` (default `0.0.0.0/0` + `::/0`). Fixture deleted by ID afterwards.

## Root cause

`lego/backend/internal/apps/settings_check.go:107` (`checkIPAllowList`) validates entries per App type only, and `apps/service.go:4857` (`SetIPAllowList`) and create carry no workspace-plan check. The environment ACL write (`environments.Service.SetACL`, ADR032:79-84) accepts `ipAllowList` on any plan. Workspace plans exist (`internal/store/plans.go:35-45`: hobby, pro, scale, enterprise), so the gate has a data source.

## Fix (target)

- One predicate, e.g. `inboundIPRulesAllowed(workspacePlan)` (`scale`/`enterprise`), applied to non-empty allowlists on web/static App create/update/clone/Blueprint and environment create/update. Message: `inbound IP rules for web services, static sites and environments require the Scale plan or higher`. Empty lists are always accepted.
- t003: existing non-empty rules on Hobby/Pro workspaces stay enforced (never silently open a restricted service) and become read-only except clearing. A downgrade from Scale keeps existing rules and refuses new non-empty edits. Record the count of affected workspaces and this transition in ADR018.

## Blast radius

App and environment allowlist writers on REST, GraphQL, MCP, dashboard and Blueprints, every workspace below Scale. Postgres/Key Value allowlists unchanged.

## Unverified

- Render's exact API status and text for this refusal (no Render account); the target uses 400 like bex's other plan gates.
- Render's handling of existing rules after a downgrade.

## Dedupe

Searched `.pm` open and done, ADR018:154, ADR030 and ADR032 for allowlist + Scale/Enterprise/workspace plan: none. w8/m115 and w1/m150/m171 concern enforcement and logging, not plan eligibility.

## Partial live evidence (2026-10-24 09:33–09:36 UTC, platform pin `ba5ad0e65498`, bex v0.3.3, `bex-canary` (Hobby), `/qa-find-bugs-cli` sweep q9r2)

- `PATCH /v1/environments/{id} {"ipAllowList":[{"cidrBlock":"203.0.113.0/24"…}]}` → 400 `ipAllowList: inbound IP rules for web services, static sites and environments require the Scale plan or higher`.
- `bex services create --type web_service --plan free … --ip-allow-list "cidr=203.0.113.0/24,description=x"` → exit 1, same 400; nothing created.
- First ~8 minutes after the pin landed (09:24) the old bex-api still accepted both (rollout lag, not a defect).
- Not covered: Key Value control, Blueprint, dashboard copy, legacy rules. DoD (t008) still open.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m137
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。

## Closeout — 2026-10-29 (`/loopx w8`)

Live on production (pin `479bd3fe243b`), 10:20–10:27 UTC, released Bex v0.3.3 and pinned Render v2.27.0. GraphQL `workspaces` confirms bex-canary is `hobby` and `bex` is `scale`. Every refusal is the 400 `ipAllowList: inbound IP rules for web services, static sites and environments require the Scale plan or higher`.

- **Create (Hobby):** both CLIs' `services create --type web_service … --ip-allow-list …` and `bex services create --type static_site …` exit 1; nothing is created.
- **Update (Hobby, owned free web `srv-dbhhs4619k3s73ep2i90` and static `srv-dbhhs5619k3s73ep2ic0`):** both CLIs' `--ip-allow-list` (exit 1), REST `PATCH`, GraphQL `setServiceIpAllowList` (`entries` and `cidrs`) and MCP `update_service` (with and without `dryRun`) are refused; an empty list is accepted on REST, GraphQL and MCP. MCP `create_web_service`/`create_static_site` dry runs with a rule are refused.
- **Environment (owned project `prj-dbhhshe19k3s73ep2iig`):** REST `PATCH /v1/environments/{id}` and `POST /v1/projects` with a restricting list, GraphQL `updateEnvironment`/`createEnvironment`, MCP `update_environment`/`create_environment` are refused; the allow-all pair is accepted.
- **Blueprint:** GraphQL `validateBlueprint` and both native `blueprints validate` runs mark a web service and a static site carrying `ipAllowList` invalid on bex-canary and valid on `bex`.
- **Postgres and Key Value stay available on every plan (no writes):** a Blueprint with KV/Postgres `ipAllowList` validates on bex-canary; MCP `update_key_value`/`update_postgres` dry runs on the existing free KV and Postgres echo the rule; both resources were unchanged afterwards.
- **Scale control (dry run only):** MCP `create_web_service`/`create_static_site` dry runs in `bex` return previews carrying the rule.
- **Dashboard (Hobby):** web/static Networking and the environment dialog show the Scale-plan hint and no editor; the free Key Value Networking section keeps its allowlist editor.
- **Inventory (read-only):** 64 hobby, 2 scale, 1 enterprise workspaces; none of 37 Apps and neither existing environment holds a restricting rule below Scale.
- Not re-run: a real Key Value allowlist write (accepted live on 2026-10-27) and the legacy-rule downgrade path (no such rule exists). Follow-ups noted: the Blueprint error path names `services[N].plan` rather than `services[N].ipAllowList`; `projects[].environments[].ipAllowList` is rejected by the schema on every plan.
- Cleanup: both services and the project deleted by ID, `GET` → 404.
