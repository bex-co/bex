# Finding — Hobby datastore caps count paid instances (run k5t8, 2026-10-22/23)

**Severity:** major. **Workspace:** `bex-canary` (`tea-daif693dqjvc73e7as3g`), Hobby. It already holds one free Postgres (`qa-20261017-64552e-pg14`, another run's fixture, not touched). During the probe it also held my free Key Value `qa-20261022-k5t8-kv` (`red-dbdado07tisc73d570cg`, since deleted).

## Probes (dry-run, no writes, from the dashboard page with `credentials:'include'`)

```text
mutation { createDatabase(ownerId:"tea-daif693dqjvc73e7as3g", name:"qa-20261022-k5t8-dry", plan:"basic-256mb", dryRun:true){ __typename } }
→ {"data":{"createDatabase":null},"errors":[{"message":"workspace is limited to 1 Postgres database; delete an existing Postgres database to create another","path":["createDatabase"],"extensions":{"code":"WORKSPACE_RESOURCE_LIMIT","limit":1}}]}

mutation { createDatabase(…, plan:"bogus-plan-x", dryRun:true) … }
→ "plan must be one of free|basic-256mb|basic-1gb" (POSTGRES_PLAN_UNSUPPORTED)   ← plan validation runs first, so basic-256mb passed it

mutation { createKeyValue(ownerId:"tea-daif693dqjvc73e7as3g", name:"qa-20261022-k5t8-dry", plan:"starter", dryRun:true){ __typename } }
→ "workspace is limited to 1 key-value store; delete an existing key-value store to create another" (WORKSPACE_RESOURCE_LIMIT, limit 1)

GET /v1/owners/tea-daif693dqjvc73e7as3g/limits
→ {"services":{"used":8,"terminating":0,"limit":25},"postgres":{"used":1,"terminating":0,"limit":1},"keyValues":{"used":1,"terminating":0,"limit":1}}
```

Dashboard: New → New Postgres, Free, Create database → inline alert "Postgres limit reached — workspace is limited to 1 Postgres database; delete an existing Postgres database to create another" with an **Upgrade plan** button.

Render ([render.com/docs/free](https://render.com/docs/free)): "Only one Free Render Postgres database can be active for any given workspace." "Only one Free Key Value instance can be active for any given workspace." No comparable limit for paid instances. The bex capture `docs/render-artifacts/key-value.md:15` quotes the same Key Value rule.

## Root cause

- `lego/backend/internal/store/plans.go:104-111` `QuotaCapsForPlan`: Hobby → `Postgres: 1, KeyValues: 1`; every other plan → `25/25`. The caps are plan-blind about the **instance** plan.
- `lego/backend/internal/store/namespaces.go:501-517` writes them into the tenant `ResourceQuota` as `count/databases.app.bex.co` / `count/keyvalues.app.bex.co` (`namespaces.go:74-75`). An object-count quota counts every CR, free or paid, and Kubernetes object-count quotas cannot select by label, so the free-only rule cannot live there.
- Refusal: `postgres/service.go:730` and `keyvalue/service.go:471` `CountCap` → `core/quota.go:116-124` `quotaCapExceeded`.
- No admission anywhere counts **free** instances (`grep -rn -i "one free" lego/backend/internal` → none), so a Pro workspace may hold 25 free Postgres.

## Blast radius (callers of the shared caps)

- `store.QuotaCapsForPlan` — 2 callers: `store/namespaces.go:501` (ResourceQuota) and `workspaces/service.go:1372` (`ResourceLimits`, the REST `/limits`, GraphQL `workspaceLimits` and MCP `get_workspace_limits` view).
- `CountCap.Exceeded` — `apps/blueprint_preview_creates.go:146` (the Blueprint preview's over-cap refusal) plus the three create paths.
- The services cap (`apps/service.go:2673`) shares `quotaCapExceeded` and must stay unchanged.
- Adjacent paths that change a database's plan: upgrading free → paid, and downgrading paid → free when a free one already exists. Postgres `SetPlan` and Key Value plan update must apply the free rule too.

## Unverified this run

- A real (non-dry-run) paid create. Not attempted, because it would buy an instance.
- Render's behavior on a paid → free downgrade when another free instance exists.
- Whether Render has any paid-instance count limit on Hobby. Its docs mention none.
