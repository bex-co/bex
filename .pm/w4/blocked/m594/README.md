# w4 · m594 — In a protected environment only Admins read datastore passwords, change datastore IP rules or touch environment groups' variables, as on Render

**Worker:** worker4 **Goal:** in a protected environment, the remaining actions Render reserves to Admins are Admin-only on bex too: viewing a Postgres's or Key Value's password and connection URLs, changing a Postgres's or Key Value's inbound IP rules, and viewing or changing an environment group's variables and secret files (and deleting the group). A non-Admin is refused with `PROTECTED_ENVIRONMENT_ADMIN_REQUIRED` on REST, GraphQL and MCP, and the dashboard shows each control disabled with Render's _This action is limited to admins._ before the click. A Blueprint's own sync keeps Render's exception. **Status:** blocked (pin must carry `3329f5eaff`: t006's production walk as the QA user, then t010; t001–t005 and t007–t009 done, and t006 passed on dev-4)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Datastore passwords and connection URLs are Admin-only in a protected environment — **DONE** | 50m | — |
| [t002](done/t002.md) | Datastore inbound IP rules are Admin-only in a protected environment — **DONE** | 45m | — |
| [t003](done/t003.md) | An environment group's variables, secret files and Delete are Admin-only in a protected environment — **DONE** | 1h15m | — |
| [t004](done/t004.md) | The datastore projections and the env-group view say so before the click — **DONE** | 45m | t001, t002, t003 |
| [t005](done/t005.md) | Dashboard: the reveal, IP-rule and env-group controls read Render's admin-only reason — **DONE** | 1h30m | t004 |
| [t006](t006.md) | Live check on dev-4 as an Admin, a Developer and a Viewer at 1440 and 390 | 45m | t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 25m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 30m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 7h10m**.

## Definition of done

Verified by the backend and dashboard suites, then on dev-4 with OpenFGA enforced (w4/m550 t005's recipe), then on production as far as its one QA identity (an Admin) allows:

- For a Developer on a member of a protected environment, each of these is refused with a `403` coded `PROTECTED_ENVIRONMENT_ADMIN_REQUIRED` on REST, GraphQL and MCP, and nothing is revealed or written:
  - Postgres and Key Value connection info (`GET /v1/postgres/{id}/connection-info`, `GET /v1/key-value/{id}/connection-info`, their GraphQL and MCP twins), and Postgres `CreateUser`'s one-time password.
  - A Postgres or Key Value `PATCH` that carries `ipAllowList`, and the dedicated IP allow-list writes over it (`SetIPAllowList`).
  - An environment group's per-key and per-file reveals, every variable and secret-file write, and `DeleteEnvGroup`. A REST read of the group answers without `envVars` and `secretFiles`, the shape Render documents for a caller without permission to read the group's secret values.
- An Admin there, and the same Developer in an unprotected environment, do each of them as before. A Viewer keeps today's role refusal.
- A Blueprint's own sync (`core.BlueprintSync`) still applies a pushed `render.yaml`'s datastore `ipAllowList` and env-group variables in a protected environment.
- `databaseActions` and `keyValueActions` report the connection-info reveal and the IP-rule write as `denied`/`protected_environment_admin_required` for a non-Admin there (decided by `core.RestrictedDecision`, the w4/m578 rule), and the dashboard can tell the same for an environment group.
- The dashboard shows, for that Developer at 1440 and 390: the Postgres and Key Value connection-info reveal and copy, the Postgres access-control and Key Value networking IP-rule editors, and an environment group's variable and secret-file editors and Delete, all disabled with _This action is limited to admins._, sending nothing. A raced `403` with the code shows the same reason.
- ADR032's residual line is rewritten (an environment group follows its own environment's protection), ADR018 and `docs/render-artifacts/render-changelog.md` (the 2024-04-16 and 2024-08-14 rows) say what shipped.

## Source + Goal linkage

- **Source:** inbox note w4/720 ([`source-720.md`](source-720.md)), filed by w4/m553 t002 on 2026-11-22 from Render's changelog entries of 2024-04-16 and 2024-08-14. w4/m578 t005 confirmed it covers these three verbs. Promoted by `/loopx w4` on 2026-11-23: the work spans three backend packages, two capability projections, five dashboard components across 8 locales and a live replay with three roles, about 7h over ten tasks, past the inbox's hour.
- **Premise checked 2026-11-23 on `26cfc53e3c`:**
  - **Render** ([render.com/docs/projects](https://render.com/docs/projects), read 2026-11-23): only Admins in a protected environment may modify access control IPs for any Postgres or Key Value instance, view or modify environment variables or secret files for any service or environment group, and view passwords or connection URLs for any Postgres or Key Value instance. Its env-groups paragraph adds that deleting a group and viewing or changing its variables or secret files are Admin-only there. Non-Admins can still change Blueprint-managed resources by publishing a `render.yaml` update.
  - **Passwords.** `PostgresConnectionInfo` (`lego/backend/internal/postgres/service.go:1145`) and `KeyValueConnectionInfo` (`keyvalue/service.go:1483`) check `can_view_sensitive` only, which a Developer holds (`deploy/gitops/authz/model.fga:88`). `CreateUser` (`postgres/access.go:133`) returns a fresh password and checks `can_create` (`:87`). Neither reads the environment's protection.
  - **IP rules.** `UpdatePostgres` (`postgres/service.go:1811`) and `UpdateKeyValue` (`keyvalue/service.go:1345`) ask `protectedDatabasePatchVerb` / `protectedKeyValuePatchVerb` (`postgres/protection.go:47`, `keyvalue/protection.go:35`), which never name `ipAllowList`, and the verbs they do name (`upgrade`, `rename`, …) are not in `core.ProtectedVerbRestricted` since w8/187. `SetIPAllowList` (`postgres/access.go:64`, `keyvalue/service.go:1105`) routes through those patches, so one gate covers both routes.
  - **Environment groups.** `authorizeGroup` (`envgroups/service.go:1829`) checks `can_view_sensitive` for reads and `can_create` for `SetEnvGroupVars`, `SetEnvGroupVarInput`, `DeleteEnvGroupVar`, `SetEnvGroupFile`, `DeleteEnvGroupFile` and `DeleteEnvGroup` (`:869-1065`). None reads the group's environment. REST's `retrieved` (`envgroups/render.go:136`) already omits values for a caller refused `can_view_sensitive`. A protected service's own env vars are Admin-only since w8/193 (`secrets/service.go:234-245`), with a bare `FORBIDDEN`.
  - **Shared gate (w4/m578).** `core.RequireEnvironmentAdmin`, `core.ProtectedVerbRestricted` and `core.RestrictedDecision` (`core/confirmation.go`), and `ReasonProtectedEnvironmentAdmin` (`core/decision.go`). The coded refusal is `core.ProtectedPlacementRefusal` (`PROTECTED_ENVIRONMENT_ADMIN_REQUIRED`), today used only for placement; suspend, resume and delete still answer a bare `FORBIDDEN`.
  - **Dashboard.** The connection-info reveals gate on the workspace `canViewSensitive` (`dashboard/src/features/databases/components/connection-info-panel.tsx:68`, `keyvalue/components/connection-info-panel.tsx:87`). The IP-rule editors (`databases/components/access-control-panel.tsx`, `keyvalue/components/key-value-networking-panel.tsx`) and the env-group editors (`env-groups/components/env-group-editors.tsx`, over `services/components/service-environment-editor.tsx`) read no protection; a refused env read shows "Not authorized — You don't have permission to view this service's environment variables." `useProtectedPlacement` (`environments/hooks/use-protected-placement.ts`) already gates an env group's Move on its environment's `protectedStatus` and the workspace `canManage`.
  - **ADR032** (`docs/ADR032-environments.md:109`) records "a linked environment group's content is workspace-scoped, not per protected service, and stays editable by anyone who may edit the group" as a residual with no sign-off.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). A divergence stands only with the user's sign-off in ADR018, and this one has none. The capability projections exist so a client never offers a verb the server refuses ([ADR087](../../../../docs/ADR087-mobile-role-views.md)).
- **Expected outcome:** a workspace that protects its production environment keeps its database passwords, its datastores' IP rules and its production env groups' secrets to Admins, as on Render. A Developer sees why before clicking.
- **Why now:** w4/m578 shipped the per-resource admin-only rule and copy for Suspend and Resume on 2026-11-23, and w4/724–726 cover Delete, maintenance mode and Shell. These three are the last items on Render's protected-environment list with no gate at all, so a Developer can read the very credentials protection exists to guard.
- **Render parity:** included (t007). The change faces tenants on REST, GraphQL, MCP and the dashboard.

## Live check (2026-11-23)

**dev-4 (t006).** bex-api built from this change, with authorization enforced by an OpenFGA seeded with `deploy/gitops/authz/model.json` and an OpenBao for the environment groups' values, both in `dev-4-auth` (the agent leg's templates, without its sandbox half); the dashboard from this tree on :50040. Three throwaway Kratos identities: an Admin on a Scale workspace, and a Developer and a Viewer it invited. Fixtures: project `m594-protected` (disposable) with `staging` and a protected `production`, each holding a Postgres with IP rules (free in staging, `basic_256mb` in production), a Key Value with external access, and an environment group with a variable and a secret file; production also held an image web service linked to its group.

**REST, GraphQL and MCP**, every verb as each role in each environment:

| Verb | Admin, both | Developer, production | Developer, staging | Viewer, both |
| --- | --- | --- | --- | --- |
| Postgres and Key Value connection info (REST, GraphQL) | 200 / ok | 403 (REST), `PROTECTED_ENVIRONMENT_ADMIN_REQUIRED` (GraphQL) | 200 / ok | 403 / `FORBIDDEN` |
| IP rules: REST `PATCH`, GraphQL `setDatabaseIpAllowList`, MCP `update_postgres` / `update_key_value` | applied | 403, the code on GraphQL and MCP | applied | 403 / `FORBIDDEN` |
| Group: per-key and per-file reveal, variable write (REST, GraphQL, MCP) | ok | 403, the code on GraphQL and MCP | ok | 403 / `FORBIDDEN` |
| Group: REST `GET` | values | 200 without `envVars` and `secretFiles` | values | 200 without them |
| Group: delete (REST, GraphQL, MCP) | — | 403, the code on GraphQL and MCP | 204 | 403 / `FORBIDDEN` |
| `databaseActions`, `keyValueActions`, `envGroupActions` rows | `allowed` | `denied` / `protected_environment_admin_required` | `allowed` | `denied` / `insufficient_permission` |

REST's `403` carries no code on these Render operations (w8/411 keeps Render's vocabulary there) and names the action in its message. No refusal body carried a value.

- **Fixed by the replay:** the Developer's REST read of the protected group first answered `403` instead of the group without values. The group's view spells its environment by the public `evm-` id, and `GetEnvironmentProtectedStatus` read that as an environment that does not exist, so the probe saw no protection and the read went on to the audited path, which refused. `valuesWithheld` now passes the storage id, and the store resolves either spelling; both have tests. Rerun after the fix: 200 without values.

**Dashboard**, headless Chromium as each role at 1440 and 390 (both widths gave the same results):

| Control | Admin, production | Developer, production | Developer, staging | Viewer, both |
| --- | --- | --- | --- | --- |
| Postgres and Key Value **Reveal connection info** | enabled | disabled, _This action is limited to admins._, nothing sent | enabled | disabled, "Your role can’t reveal secret values. Ask a workspace admin." |
| Postgres Access control and Key Value Networking IP rules | inputs enabled | rules listed, every input disabled, Save says _This action is limited to admins._, nothing sent | inputs enabled | inputs disabled, Save says "Your role can’t perform this action. Ask a workspace admin." |
| Group **Edit**, **Reveal** (variable and file) | enabled | disabled with the admin-only reason, nothing sent | enabled | disabled with the role reasons |
| Group **Unlink** and Manage → **Delete** | enabled | disabled with the admin-only reason, nothing opened | enabled | disabled with the role reason |

The pooled-string reveal was not shown (no pooler on these plans); its gate is covered by `admin-only-controls.test.tsx`. A raced coded `403` was not forced live; the API rows above show the code reaching GraphQL, and the component tests show each control reading it as the admin-only reason.

- **Cleanup:** the services, datastores, groups, project, the three workspaces and the three identities were deleted, the OpenFGA and OpenBao deployments removed, the dashboard stopped, and bex-api restarted with its defaults.
- **Production:** waits for a pin that carries this change. The QA user is an Admin in `bex-canary`, so the walk checks that an Admin keeps every control in a protected environment of a disposable project.
