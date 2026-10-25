# w4 · m241 — A Hobby workspace's one-instance limit applies to free datastores only, as on Render

**Worker:** worker4 **Goal:** every workspace may hold one **free** Postgres and one **free** Key Value, as [Render documents](https://render.com/docs/free) ("Only one Free Render Postgres database can be active for any given workspace"; the same for Key Value). Paid instances do not count toward that limit. Today a Hobby workspace is capped at one Postgres and one Key Value **of any plan**: once it holds a free database, a paid `basic-256mb` create is refused with `WORKSPACE_RESOURCE_LIMIT` and an "Upgrade plan" button. A Pro workspace can hold 25 free ones. **Status:** blocked — t001–t006 done 2026-10-25 (loopx w4); t007 waits for the release pin, then QA replays the dry-run DoD in `bex-canary`

## Tasks (in order)

| id              | title                                                                         | est | depends_on                 |
| --------------- | ----------------------------------------------------------------------------- | --- | -------------------------- |
| [t001](done/t001.md) | Admission: one free Postgres and one free Key Value per workspace, every plan | 60m | —                          | — **DONE**
| [t002](done/t002.md) | Hobby total datastore caps stop limiting paid instances                       | 30m | w4/m241/t001               | — **DONE**
| [t003](done/t003.md) | Blast radius: limits view, Blueprint preview, plan changes, refusal copy      | 45m | w4/m241/t001, w4/m241/t002 | — **DONE**
| [t004](done/t004.md) | Render parity                                                                 | 20m | w4/m241/t003               | — **DONE**
| [t005](done/t005.md) | Simplify                                                                      | 15m | w4/m241/t004               | — **DONE**
| [t006](done/t006.md) | Test coverage                                                                 | 25m | w4/m241/t004               | — **DONE**
| [t007](t007.md) | Closeout (live DoD replay)                                                    | 15m | w4/m241/t005, w4/m241/t006 |

## Definition of done

Live in `bex-canary` (Hobby; it holds one free Postgres, `qa-20261017-64552e-pg14`), after the release pin includes the fix. Every probe is a GraphQL **`dryRun: true`** create, so nothing is bought:

- `createDatabase(ownerId:"tea-daif693dqjvc73e7as3g", name:"qa-<date>-dry", plan:"basic-256mb", dryRun:true)` → no `WORKSPACE_RESOURCE_LIMIT` error (today: "workspace is limited to 1 Postgres database").
- `createDatabase(…, plan:"free", dryRun:true)` → still refused. The refusal says the workspace already has its one **free** Postgres and names the existing one or the free rule, with a stable code a client can key on. It does not tell the user to upgrade the workspace plan.
- `createKeyValue(…, plan:"starter", dryRun:true)` with one free Key Value present → admitted. `plan:"free"` → refused the same way.
- `GET /v1/owners/tea-daif693dqjvc73e7as3g/limits` reports a free-instance limit that agrees with what the dry-runs did. The dashboard Usage page and New Postgres / New Key Value dialogs say the same.
- Control: an invalid plan still answers `POSTGRES_PLAN_UNSUPPORTED` first; the services cap is unchanged.
- Any fixture created for the replay is deleted (`GET` → `404`); QA session revoked.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop run k5t8, 2026-10-22/23 UTC (`muse.env`, `bex-canary` `tea-daif693dqjvc73e7as3g`, pin `b3cd5bc74de8`). Probe, root cause and blast radius in [finding.md](finding.md). The run's fixtures (project `qa-20261022-k5t8-proj` with environment Production, project `qa-20261022-k5t8-rest`, Key Value `qa-20261022-k5t8-kv`) were all deleted (`GET` → `404`); session revoked.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); Postgres ([ADR009](../../../docs/ADR009-postgresql-management.md)); Key Value ([ADR021](../../../docs/ADR021-keyvalue-management.md)); pricing ([ADR030](../../../docs/ADR030-pricing.md)); per-tenant quota ([ADR043](../../../docs/ADR043-tenant-namespace-isolation.md)); ledger ([ADR018](../../../docs/ADR018-render-parity.md)).
- **Expected outcome:** a Hobby user who has tried the free database can buy a paid one in the same workspace. The free tier is bounded the way Render bounds it, on every plan.
- **Why now:** severity major — revenue-blocking and misleading. The refusal tells the user to upgrade the workspace plan, which Render never requires for a paid database. w4/339 fixed only the message's grammar and assumed the limit itself was right. The Render parity closing task applies: REST, GraphQL, MCP, Blueprint, the limits surface and the dashboard all change.

## Live evidence (2026-10-25, platform pin `bd6423e5e46f`, bex v0.3.3, `bex-canary`, `/qa-find-bugs-cli` loop)

`bex blueprints validate` with a `starter` Key Value and a `basic-256mb` Postgres (one free Postgres already in the workspace) → `valid: true`. The same with `plan: free` for both → invalid, `this workspace already has a free Postgres database (qa-20261017-64552e-pg14); delete it or choose a paid instance type` (free Key Value admitted, none present).
