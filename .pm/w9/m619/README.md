# w9 · m619 — A Contributor changes no service or datastore settings, as on Render

**Worker:** worker9 **Goal:** a Contributor's role allows on bex what Render's role table allows: deploy and operate, never change a service's or datastore's configuration. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Classify every `can_operate` write against Render's table; amend ADR012's matrix, ADR024, ADR087 and record the table | 45m | — |
| [t002](t002.md) | Service settings on `can_create`: the `servicePatchTable` setting rows, scaling, autoscaling, cache profile, maintenance mode and the cron schedule | 1h | t001 |
| [t003](t003.md) | Custom domains (add, verify, delete) and a service's `ipAllowList` on `can_create` | 30m | t001 |
| [t004](t004.md) | Postgres and Key Value settings (update, plan, version) on `can_create` | 40m | t001 |
| [t005](t005.md) | Dashboard: each moved control disabled with the Contributor's reason | 1h | t002, t003, t004 |
| [t006](t006.md) | Role-ladder and key-ceiling pins: a Contributor and a Contributor's API key are refused every moved verb | 45m | t002, t003, t004 |
| [t007](t007.md) | Live walk on dev-9: a Contributor in a Scale workspace | 30m | t005, t006 |
| [t008](t008.md) | Render parity | 30m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 45m | t008, t009 |
| [t011](t011.md) | Closeout | 10m | t010 |

## Definition of done

On dev-9, a Contributor in a Scale workspace gets a 403 on REST, GraphQL and MCP for every service and datastore setting change: plan, instance count, autoscaling, health check path, build filter, auto-deploy, maintenance mode, edge cache profile, subdomain policy, IP rules, custom domains, the service notification override, the cron schedule, and Postgres and Key Value settings, plan and version. Nothing is written. Restart, suspend, resume, cron runs and cache purge still succeed. The dashboard shows each moved control disabled with its reason, and a Developer is unaffected. ADR012's matrix, ADR024, ADR087 and ADR018's members row state the new line, and the role-ladder tests pin it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w9` (2026-11-23), item 1, verified before filing. Render: role table and role descriptions at render.com/docs/team-members (read 2026-11-23): Contributor ❌ "Modify service configuration", ❌ "Manage inbound IP rules", "Can't create, modify, or delete most resources". bex: `deploy/gitops/authz/model.fga:86`, `lego/backend/internal/apps/settings.go` `servicePatchTable`, `apps/service.go`, `apps/domains.go`, `apps/edge_cache.go`, `postgres/service.go`, `keyvalue/service.go`.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)): a member's role must allow on bex exactly what it allows on Render.
- **Expected outcome:** a Contributor operates services (deploys via w9/m622, restarts, suspends, cron runs) but no longer changes their configuration, including billed plan and instance changes. The key catalog (w9/m496), which already puts service management at `can_create`, and the member's own verbs agree.
- **Why now:** signup has been open since 2026-11-21, so Scale workspaces with Contributor members are real customers. A Contributor can move a service to a larger, billed plan today, which Render reserves for a Developer. API keys act with their creator's role since w9/m481, so every over-broad relation also reaches keys. Order: before w9/m622, which amends the same ADR024 section.
- **Recorded assumption:** restart, suspend, resume, cron run trigger and cancel, cache purge, and Postgres restart and failover stay with a Contributor. Render's role table restricts none of them. The service notification override moves to Developer, not Admin, as `api/key_permission_ops.go:1024` and Render's API-key catalog place it.
- **Render parity closing task:** included, because the milestone changes REST, GraphQL, MCP and dashboard behavior.
