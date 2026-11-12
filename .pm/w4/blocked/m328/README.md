# w4 · m328 — A service's Events feed records an env-group link only from the Link and Unlink buttons, and no link event names its group

**Worker:** worker4 **Goal:** every change to a service's linked environment groups records one `env_group_linked` or `env_group_unlinked` row in that service's feed, naming the group, whichever path made it. **Status:** blocked — t001–t003 and t005–t007 done (shipped); t004 live replay waits for the next production deploy, then t008 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Backend: a named link/unlink row from every path — Link, Unlink, create with `serviceIds`, group delete, Blueprint `fromGroup` — **DONE** | 45m | — |
| [t002](done/t002.md) | Store and API: the group's id and name on the audit row, the feed, the by-id lookup, REST, GraphQL and MCP — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Dashboard: the Events row reads "Environment group <name> linked/unlinked", en and zh — **DONE** | 20m | t002 |
| [t004](t004.md) | Live verification on an owned service | 20m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 10m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 10m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 20m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

Eight tasks, 195m (3h15m).

## Source + Goal linkage

- **Source:** promoted from inbox note w4/563 (live `/qa-find-bugs w4` loop, round 62, 2026-11-11 UTC) on 2026-11-11: naming the group needs typed audit columns, the feed and lookup projections, every API surface and the dashboard, more than one sub-hour task.
- **Goal linkage:** the Events feed is the owner's record of why a service's environment changed (ADR006 events).
- **Render parity:** `env_group_linked`/`env_group_unlinked` are bex-named types; included only to keep their details in the same shape as Render's event details on every surface.

## Finding (from w4/563)

Why: the feed is the owner's record of why the service's environment changed. When the group arrives through the create dialog or disappears with a group delete, the feed shows a "Config Change" deploy with no cause. When the cause is recorded, it doesn't say which group.

- **Source / severity / estimate:** live `/qa-find-bugs w4` loop, round 62, 2026-11-11 01:53–02:01 UTC (`muse.env`, workspace `bex-canary`, pin `a0d5802594e9`). **Minor**: an incomplete history; the deploys themselves are right. ~35m.
- **Fixtures** (all deleted; DELETE 204, then GET 404):
  - Free image service `qa-20261110-eg62` (`srv-dbpsl4fbq3ic73c0a9bg`, live at 01:54:29);
  - group `qa-20261110-eg62-a` (`evg-dbpsmgnbq3ic73c0a9ig`);
  - group `qa-20261110-eg62-b` (`evg-dbpsmgthi5vc738gj13g`).
- **Probe** (REST from the page; `GET /v1/services/srv-dbpsl4fbq3ic73c0a9bg/events` after each step):
  1. **Created with `serviceIds`.** At 01:56:18, `POST /v1/env-groups {"name":"qa-20261110-eg62-a",…,"serviceIds":["srv-dbpsl4fbq3ic73c0a9bg"]}` returned 201.
     - Events gained only `deploy_started @01:56:18` (`envUpdated: true`). **No `env_group_linked`.**
     - The audit log has `envgroups.CreateEnvGroup` with metadata `{"relation":"can_create"}`, and no service.
  2. **Linked from the button.** At 01:56:43, the service Environment tab's "Link qa-20261110-eg62-b" gave `env_group_linked @01:56:43` with details `{}`. The audit row is `envgroups.LinkService` with metadata `{"service":"srv-…"}`.
  3. **Unlinked from the button.** At 01:58:04, "Unlink qa-20261110-eg62-a" gave `env_group_unlinked @01:58:04`, also with details `{}`.
  4. **Deleted while linked.** At 01:59:49, `DELETE /v1/env-groups/evg-dbpsmgthi5vc738gj13g` (group b, still linked) returned 204.
     - Events gained `deploy_started @01:59:49` (`envUpdated: true`). **No `env_group_unlinked`.**
  5. **The Events tab** shows "Environment group unlinked · 1m" with no group name, and "Deploy started · Config Change" for steps 1 and 4.
- **Root cause:**
  - **The feed maps only the two button verbs** (plus Blueprint `LinkEnvGroup`). The env-group rows are derived from audit verbs: `lego/backend/internal/events/service.go:456-458` maps `envgroups.LinkService`, `envgroups.UnlinkService` and `envgroups.LinkEnvGroup`.
  - **Create-with-services writes no per-service row.** It links through `createEnvGroupAuthorized` → `prepareCreateServices`/`persistCreate` (`lego/backend/internal/envgroups/service.go:462-483`). That records one `envgroups.CreateEnvGroup` row, with no service metadata.
  - **Delete detaches without a per-service row.** `DeleteEnvGroup` (`:809-835`) detaches each linked service through the internal `s.detach` (`:1261-`), which records nothing per service.
  - **No link row names the group.** The `LinkService` and `UnlinkService` audit rows carry only `service`, so the derived feed details are `{}`.
- **Fix (target behavior):**
  - **Every change to a service's linked groups records one row** in that service's feed: `env_group_linked` or `env_group_unlinked`. That covers the Link and Unlink buttons, REST `POST`/`DELETE /v1/env-groups/{id}/services/{serviceId}`, create with `serviceIds` (the dashboard create dialog and REST, GraphQL, MCP), group delete, and Blueprint `fromGroup`.
  - **The details name the group:** `{"envGroupId":"evg-…","envGroupName":"…"}`. The dashboard renders "Environment group <name> linked" or "… unlinked" in en and zh.
  - Webhook delivery of these types follows from the same rows.
- **Tests:**
  - Create with two `serviceIds` gives two `env_group_linked` rows, one per service, each naming the group.
  - Deleting a group linked to a service gives `env_group_unlinked` on that service.
  - The Link and Unlink controls keep one row each.
- **Unverified:**
  - Group move between environments (`MoveEnvGroup`, `service.go:674` and `graphql.go:268`), which may detach services.
  - Clone (`envgroups.CloneEnvGroup`).
  - Webhook deliveries.
- **Render:** n/a. `env_group_linked` and `env_group_unlinked` are bex-named types (`events/service.go:55`).
- **Dedupe:**
  - w4/487 (done) covers secret-file writes, and recorded that `POST/DELETE …/env-groups/{eg}/services/{id}` emit link and unlink events.
  - w6/067 (done) covers shadowed keys.
  - w4/554 (open) covers the link and unlink copy for image services.
  - None of these covers create-with-services or group delete.
  - Not deploy lag: `git log a0d5802594e9..origin/main -- lego/backend/internal/events lego/backend/internal/envgroups` is empty.
- **DoD (live):** on an owned service:
  1. Create a group with `serviceIds` naming it. The service's Events lists `env_group_linked` naming the group.
  2. Delete that group. Events lists `env_group_unlinked` naming it.
  3. Link and Unlink from the Environment tab still each record one row, now with the group's name, in en and zh.
