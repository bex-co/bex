# w8 · m149 — REST lists scope by `ownerId` the way Render's do

**Worker:** worker8 **Goal:** every Render list route bex serves returns exactly the workspaces the request names. `ownerId` narrows to those workspaces, and an omitted `ownerId` spans every workspace the caller belongs to, as Render's lists and the pinned CLI's name resolvers expect. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `GET /v1/disks` honors `ownerId` | 30m | — |
| t002 | An unfiltered `GET /v1/projects` spans the caller's workspaces, so the CLI's `--project`/`--environment` name lookups resolve | 45m | — |
| t003 | Every other unfiltered REST list spans the caller's workspaces; ADR006/ADR018 amended | 60m | t002 |
| t004 | Render parity | 20m | t001, t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout (live CLI + REST DoD) | 20m | t006 |

## Definition of done

On production, for a caller who belongs to several workspaces (the active CLI workspace not being the caller's default):

1. `GET /v1/disks?ownerId=<workspace A>` lists only A's disks, including a disk just attached to a service in A. `GET /v1/disks?ownerId=<A>&diskId=<that disk>` returns it. `?ownerId=<a workspace the caller is not in>` answers 403, as `GET /v1/services` does.
2. `bex pg create --plan free --project <project name in A> --environment <env name> --confirm -o json` and `bex kv create --plan free --environment <env name only in A> --confirm -o json` create the store in A, with no `--workspace` flag. An environment name that exists in two of the caller's workspaces is refused as ambiguous by the pinned CLI, not resolved silently into one of them.
3. Each unfiltered Render list route (`/services`, `/postgres`, `/key-value`, `/redis`, `/projects`, `/env-groups`, `/blueprints`, `/disks`, `/registrycredentials`, `/webhooks`, and any other list whose pinned spec makes `ownerId` optional; `/sandboxes` requires it) returns the union of the caller's workspaces in one stable cursor order. With `ownerId`, it returns only the named workspaces. ADR006 and ADR018 say so, and nothing still describes "omitted `ownerId` = default workspace" for a list.

## Findings

### F1 — `GET /v1/disks` ignores `ownerId` and answers from the caller's default workspace

- **Severity:** major. A filtered list silently returns another workspace's disks and omits the requested workspace's own. A script that acts on "the disks of workspace A" acts on workspace B's.
- **Versions:** production pin `1672bb379` (2026-11-10). Read with `curl` and the caller's own token; the pinned CLI v2.35.0 has no disks command.
- **Context:** 2026-11-10 07:27–07:41Z, `https://api.bex.co/v1/`, caller in three workspaces (default `bex` `tea-d98210cbbpdc73dcrkvg`, `tian-personal`, `bex-canary` `tea-daif693dqjvc73e7as3g`). `/qa-find-bugs-cli` sweep `54a1`.
- **Repro and evidence** (owned fixture: starter service `srv-dbpcja9pjd2c73f0qqjg` in bex-canary; `POST /v1/disks` → 201 `dsk-dbpcknv5g1is7398ksc0`, 1 GB at `/data`; both deleted afterwards):

  ```text
  GET /v1/disks?ownerId=tea-daif693dqjvc73e7as3g                    → 200, 6 disks, all of bex's services, ours missing   Rndr-Id b5154af9-dd94-46a0-8753-b08efa4fab05
  GET /v1/disks?ownerId=tea-d98210cbbpdc73dcrkvg                    → 200, the same 6
  GET /v1/disks                                                     → 200, the same 6
  GET /v1/disks?ownerId=tea-daif693dqjvc73e7as3g&diskId=dsk-dbpcknv5g1is7398ksc0  → 200 []                                  Rndr-Id 80829902-f373-4df0-96ea-a866e334e8d0
  GET /v1/disks?ownerId=tea-daif693dqjvc73e7as3g&serviceId=srv-dbpcja9pjd2c73f0qqjg → 200, ours
  GET /v1/disks?ownerId=tea-00000000000000000000                    → 200, bex's 6   (GET /v1/services with the same ownerId → 403 FORBIDDEN)
  ```

  The same three-way comparison over every pinned list route that takes `ownerId` showed only `/disks` ignoring it.

- **Root cause:** `lego/backend/internal/apps/disks.go:376-450` reads `serviceId`, `diskId`, `name` and the time windows, but never `ownerId`. Its no-`serviceId` branch calls `ListDisks(ctx, "")` (`:142-168`), which lists `s.WorkspaceOrDefault(ctx)`. The handler's own comment says "a filter this handler does not implement is … silently ignored … a wrong answer delivered confidently is worse than a refusal" (w1/m88/t003). The pinned spec declares `ownerId` on `list-disks` (`lego/backend/internal/api/openapi/render-public-api-1.json`, `/disks` `get`, first parameter), so the OpenAPI gate admits it. GraphQL `disks` (`disks.go:544-552`) has no `ownerId` argument. MCP `list_disks` takes the session's `workspaceId` through `mcp_workspace.go`. The dashboard always passes `serviceId` (`dashboard/src/features/services/api/disks.graphql`), so it is unaffected.

### F2 — Unfiltered lists answer only the default workspace, so the pinned CLI's project and environment name lookups miss the active workspace

- **Severity:** major. `bex pg create`/`kv create` (and every pinned command that takes `--project`/`--environment` by name) fails, or resolves into the wrong workspace, for any caller whose project is outside their default workspace.
- **Versions:** released `bex v0.5.0` (Render CLI v2.35.0 `24ac2d60fdb2`), production pin `1672bb379`.
- **Repro** (owned fixtures in bex-canary: project `prj-dbpd4caj0hgs73eavn60` `qa-20261110-54a1-proj` with environments Production `evm-dbpd4caj0hgs73eavn6g` and Staging `evm-dbpd4caj0hgs73eavn70`; all deleted afterwards). The active CLI workspace is bex-canary (`BEX_WORKSPACE`):

  ```text
  $ bex pg create --name qa-20261110-54a1-pg --plan free --project qa-20261110-54a1-proj --environment Staging -o json --confirm
  Error: no project found with name "qa-20261110-54a1-proj"                                   exit 1, 0.29 s
  $ bex kv create --name qa-20261110-54a1-kv2 --plan free --environment Staging -o json --confirm
  Error: no environment found with name "Staging"                                             exit 1, 0.60 s
  controls:
  $ bex pg create … --workspace tea-daif693dqjvc73e7as3g --project qa-20261110-54a1-proj --environment Staging   → exit 0, dpg-dbpd50a0mi5c73c5phg0 in evm-…n70
  $ bex kv create … --project prj-dbpd4caj0hgs73eavn60 --environment Staging                                   → exit 0 (ID path)
  GET /v1/projects?name=qa-20261110-54a1-proj                              → 200 []        Rndr-Id 68758219-0843-4c81-b445-9c547e97a7d1
  GET /v1/projects?ownerId=tea-daif693dqjvc73e7as3g&name=qa-20261110-54a1-proj → 200 [prj-dbpd4caj0hgs73eavn60]
  ```

  The default workspace has a project whose environment is named `Production`. So `bex kv create --environment Production` from bex-canary would resolve into the **default** workspace and create the store there. Render would see both `Production` environments and refuse the name as ambiguous. That command was not run, because it would have created a resource outside the authorized workspace.

- **Consumer contract (pinned):** `pkg/resolve/resolve.go` `ResolveScope` "The configured active workspace is only used when no scope input is provided". `ResolveProject` says "if workspaceID is empty the search spans every workspace the caller can access". It calls `pkg/project/repo.go` `ListProjectsByName`, which sends `ownerId` only when `--workspace` is given. An environment name without a project goes through `ListAllAccessibleProjects` ("every project the caller can see across all workspaces"), an unfiltered `GET /projects`. Render's overrides list documents the same rule ("all notification overrides for all workspaces the user belongs to", w8/191).
- **Root cause:** `lego/backend/internal/projects/rest.go:120-128` passes an empty `ownerId` to `listBare`, which authorizes and lists `WorkspaceOrDefault`. w4/475 made the omitted `ownerId` legal by copying "as every sibling list does". It did not check what Render returns. The same default-workspace rule is in every list adapter (AGENTS.md: "Empty → the default workspace for `List`"). Only the notification-overrides list spans member workspaces (`apps/rest.go:1815-1828` `listOverrideApps`, w8/191), and that is the pattern to reuse.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` w8 loop sweep `54a1`, 2026-11-10, production pin `1672bb379`, released `bex v0.5.0`.
- **Goal linkage:** Render parity first (ADR008). The pinned Render CLI is the wire-contract oracle, and its name resolvers depend on the unfiltered list semantics. Render-conformant clients (CLI, SDK, Terraform) read `ownerId` as a filter, not as a hint.
- **Expected outcome:** a multi-workspace customer can name projects and environments in any of their workspaces from the CLI, and a filtered disks list returns the named workspace's disks.
- **Why now:** both failures are silent wrong answers rather than errors. F2 can also place a new database in the wrong workspace. The disks list is the only `ownerId` filter still ignored after w1/m88, and v0.5.0 was just released to customers.
- **Render parity task:** included. The change touches REST lists, and GraphQL `disks` lacks `ownerId`. Check GraphQL/MCP list defaults against the REST change, and the dashboard's REST callers (it reads GraphQL).
- **Dedupe:** w8/191 (done: overrides only, plus default paging), w4/475 (done: made `ownerId` optional on projects and chose the default workspace), w4/m172 (done: by-id verbs across workspaces, not lists), w1/m88/t003 (done: disks filters other than `ownerId`). `rg 'spans every workspace|ListAllAccessibleProjects|disks.*ownerId' .pm docs` finds no open item.
