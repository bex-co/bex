# w5 · m26 — Workspace-level Env Groups page

**Worker:** worker5 **Goal:** A user can create and manage environment groups as first-class **workspace** resources — view all groups, create/rename/delete, edit their env vars & secret files, and link/unlink services — without opening any individual service's Environment tab. **Status:** done (2026-07-14)

## Tasks (in order)

| id   | title                                                                                                                                                                                             | est | depends_on |            |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Data layer: promote `use-env-groups.ts` out of service scope into a `features/env-groups/` feature — hooks over `envGroups`/`envGroup` + `createEnvGroup`/`deleteEnvGroup` + `setEnvGroupVars`/`setEnvGroupSecretFile`/`deleteEnvGroupSecretFile` + `linkEnvGroup`/`unlinkEnvGroup` | 40m | —          | — **DONE** |
| t002 | Env Groups list page + route (`/env-groups`) + sidebar nav entry (`dashboard-sidebar.tsx`): groups with var/file + linked-service counts, empty state, "New Env Group" action                    | 45m | t001       | — **DONE** |
| t003 | Env Group detail page + route (`/env-groups/$groupId`): env-vars + secret-files editors (**reuse** the Environment tab's `env-groups-panel.tsx` editors), rename/delete, linked-services list     | 40m | t001       | — **DONE** |
| t004 | Create-group dialog (mirror `new-project-dialog.tsx`) + link/unlink-service action from the detail page                                                                                          | 40m | t001       | — **DONE** |
| t005 | Empty/loading/error states + `en`/`zh` locale strings                                                                                                                                            | 30m | t002, t003 | — **DONE** |
| t006 | Live verification against the mock cluster: create → add vars + secret file → link to a service → confirm the service's Environment tab reflects it → unlink → delete                             | 30m | t004, t005 | — **DONE** |
| t007 | Render parity — full-surface check (REST/GraphQL/MCP already ship env-group CRUD + link/unlink; confirm the new workspace-level UI matches their contract + Render's Env Groups page, flag drift) | 20m | t006       | — **DONE** |
| t008 | Simplify — `/simplify` over the code this milestone changed                                                                                                                                      | 20m | t007       | — **DONE** |
| t009 | Test coverage — meaningful tests for the behavior this milestone shipped                                                                                                                         | 30m | t007       | — **DONE** |
| t010 | Closeout — DoD met → move milestone to `done/`                                                                                                                                                   | 10m | t009       | — **DONE** |

## Definition of done

From a workspace-level Env Groups page, a user can list all env groups, create/rename/delete a group, edit its env vars and secret files, and link/unlink services — with no service Environment tab involved — verified against the mock cluster. A group created and left unlinked to any service is still fully manageable.

## Source + Goal linkage

- **Source:** `docs/ADR018-render-parity.md` "Environment groups" row (UI ✅ but _service-scoped only_: "dashboard Environment tab Env-Groups section"). Render exposes env groups as a dedicated workspace-level page (create group → add vars/files → link services). Proposed via `/pm-brainstorm more tasks for w5 to achieve feature parity` 2026-07-13.
- **Goal linkage:** closes the IA divergence in the Environment-groups parity cell — the `internal/envgroups` backend (full CRUD + link/unlink across REST/GraphQL/MCP) has no workspace-level home in the dashboard.
- **Expected outcome:** users manage env groups as workspace resources (Render's model), including groups not yet linked to any service.
- **Why now:** backend fully shipped and inert at the workspace level; one of only two genuine w5 parity gaps left after m25.
- **Render parity closing task: included** — the milestone adds a workspace-level dashboard UI over an existing REST/GraphQL/MCP contract (`internal/envgroups`).

## Closeout notes (2026-07-14)

- **Shipped:** `dashboard/src/features/env-groups/` now owns the shared GraphQL documents, normalized types, list/detail and mutation hooks, editors, linked-services card, create/rename/typed-delete dialogs, locales, and tests. `/env-groups` is a first-class sidebar destination with count cards and a create flow; `/env-groups/$groupId` edits an unlinked or linked group without service context. The service Environment tab imports the same feature hooks and preserves its existing service-side link view.
- **Contract completion:** the backend now exposes metadata rename and sibling-preserving single-variable set/delete consistently across Core, REST, GraphQL, and MCP, with per-value sensitive reads and full tests. Rename preserves the immutable `evg-…` id, contents, links, and running revision; content and link changes roll affected services.
- **Parity (t007):** the UI and all API adapters were checked against Render's public Environment Groups guide and OpenAPI. `docs/ADR018-render-parity.md` now records the shipped workspace IA and the remaining REST/object-shape, owner/workspace-attribution, environment-scope, create-payload, and rollout-policy divergences instead of overstating full REST parity; `w7/m12/t001` already records env groups as deliberately excluded until they gain tenant attribution. `docs/ADR006-bex-api.md` records the complete current bex contract.
- **Simplify (t008):** service and group screens reuse `EnvVarsEditor`/`SecretFilesEditor`; service-scoped duplicate env-group documents/hooks were removed; one shared feature maps the nullable wire shape; per-key writes avoid revealing and resubmitting sibling values; successful writes use best-effort refetch so a cache refresh cannot be misreported as a failed mutation.
- **Live verification (t006):** a CAPD mock cluster ran the real manager, bex-api, OpenBao Kubernetes-auth store, temporary control-plane Postgres, dashboard, and a running image-backed service. The browser completed create → unlinked var/file add + explicit reveal → link → verify keys/files and `Linked` state on the service Environment tab → rename → unlink → typed delete, with no console, request, or API errors. A separate API/operator proof verified both projected Kubernetes Secrets, `spec.envFromSecrets`/`spec.filesFromSecrets`, linked-content rollouts, unlink cleanup, and Secret deletion. Temporary groups, service, workspace, and secrets were removed; the pre-existing drill App remained Running.
- **Routing regression found live:** the first detail filename nested under the list page, so the URL changed without mounting the detail component. The detail route now uses TanStack's trailing-underscore escape (`env-groups_.$groupId.tsx`), preserving the public URL while making list/detail siblings; a route-tree test guards this exact topology.
- **Quality gates (t009):** dashboard `yarn test` passed 154 files / 950 tests; `yarn lint`, `yarn typecheck`, and the production `yarn build` passed. Backend `go test ./...`, `go build ./...`, and `make lint-backend` passed with zero lint issues. The focused live flow and route regression passed after the final fix.
