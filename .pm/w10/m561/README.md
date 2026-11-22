# w10 · m561 — An automation or a session can turn `gh` off for itself, and an admin can make the workspace's `gh` off mandatory, as Devin's security profiles bind per automation and per session

**Worker:** worker10 **Goal:** `gh` access resolves through Devin's binding chain instead of one workspace switch. A session create and an automation template carry `agentConfig.githubCli`. The agent environment gains an automations default and a mandatory flag. The mints, the dispatch and the overlay use the most specific binding, unless the workspace's `gh` off is mandatory. A refusal names the level that turned `gh` off. REST, GraphQL, MCP, the CLI and the dashboard read and write each level. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: names, the binding chain and its precedence, who may set each level, when a change applies, the refusal and driver contract; ADR047 and ADR090 amended | 45m | — |
| [t002](t002.md) | Backend: the agent environment gains the automations default and the mandatory flag on REST, GraphQL and MCP | 1h30m | t001 |
| [t003](t003.md) | Backend: a session create and an automation template carry `agentConfig.githubCli`, refused where it would loosen a mandatory or admin-set binding, and child sessions inherit it | 1h30m | t001, t002 |
| [t004](t004.md) | Backend and gateway: one resolver feeds the mints and `driverEnv`, and the route's refusal names the level | 1h30m | t002, t003 |
| [t005](t005.md) | Driver: the overlay says which level turned `gh` off | 30m | t004 |
| [t006](t006.md) | CLI: `--github-cli` on `bex agent run`/`chat`/`handoff` and `bex agent automations`, and the two new `bex agent env set` flags | 1h | t002, t003 |
| [t007](t007.md) | Dashboard: the session composer, the automation form and Settings → Agents set each level, with matching skeletons | 2h | t002, t003 |
| [t008](t008.md) | Live check on production: a session's and an automation's own `gh` off, and a mandatory workspace off that no create can loosen | 45m | t004, t005, t006, t007 |
| [t009](t009.md) | Render parity (Devin) | 30m | t008 |
| [t010](t010.md) | Simplify | 20m | t009 |
| [t011](t011.md) | Test coverage | 45m | t009, t010 |
| [t012](t012.md) | Closeout | 10m | t011 |

Total: **12 tasks, about 11h15m**.

## Definition of done

1. **Each level carries a binding.**
   - A session create carries `agentConfig.githubCli` (true, false, or absent for inherit) on REST `POST /v1/agent-sessions`, GraphQL, MCP, `bex agent run`/`chat`/`handoff --github-cli on|off` and the dashboard composer. An automation template carries the same field on REST, GraphQL, MCP, `bex agent automations create`/`update`/`apply` and the automation form.
   - The agent environment gains an automations default (inherit, on or off) and a mandatory flag on REST `GET`/`PATCH /v1/agent-environment`, GraphQL, MCP, `bex agent env`/`env set` and Settings → Agents.
   - Every read shows what was stored, the session and automation reads included. Final names are t001's.
2. **The most specific binding wins, unless mandatory.**
   - Precedence from broadest to most specific: workspace (`githubCliEnabled`), the automations default for automation-started sessions, the automation, the session.
   - A child session is governed by its parent's chain.
   - With the workspace's `gh` off and mandatory, no lower level turns it on.
   - Lower levels may always turn `gh` off.
   - A test pins each pair of levels in each direction, and the mandatory floor.
3. **Loosening is refused, naming why.**
   - A create, an automation save or an environment save that would turn `gh` on below a mandatory off is refused with a named code, on every surface.
   - So is a member's attempt to turn `gh` on where an admin's binding has it off. t001 records who may set which direction against Devin's "Manage security profiles" permission.
   - Turning `gh` off for one's own session or automation is never refused.
4. **Enforcement resolves the chain.**
   - The `gh` mints (`Minter.githubCLIAllowed`, `lego/backend/internal/agentsession/mint.go`) and `driverEnv` (`lego/backend/internal/agentsessions/service.go`) use one resolver. The session's web Shell, SSH and Zed `gh`, which use the same route, follow it too.
   - The route's 403 names the level that turned `gh` off, and how that level turns it back on.
   - A turn dispatched with `gh` off at any level gets no `gh` configuration, and its overlay names the level.
   - An older gateway or driver still answers or behaves as for the workspace level, never with `gh` on.
5. **Said everywhere.** ADR047 D2's `gh` amendment and ADR090 D4 describe the chain, the mandatory flag, who may set each level and when a change applies. The MCP tool descriptions, the GraphQL field descriptions and the CLI help name the fields.
6. **Live.**
   - Setup: on production, once a deploy carries the milestone, in the QA account's `tian-personal`, on a fixture repository the check deletes, with at most three short sessions.
   - A session created with `githubCli: false` while the workspace has `gh` on has no `GH_*` and its overlay names the session.
   - An automation whose template turns `gh` off starts a session with no `gh` when run once. Its overlay names the automation.
   - With the workspace's `gh` off and mandatory, a create naming `githubCli: true` is refused with the named code, on REST and in the composer.
   - The workspace setting is left on and not mandatory, the automations default inherits, and the fixtures are deleted.

## Source + Goal linkage

- **Source:** inbox note [w10/035](../done/035.md), filed by w10/m524's Devin comparison (t008) on 2026-11-19 and promoted by `/loopx w10` on 2026-11-22.
- **Triage evidence (2026-11-22, `d417a1e70`):**
  - **The workspace switch is the only control.** The mints read only the workspace row (`Minter.githubCLIAllowed`, `lego/backend/internal/agentsession/mint.go:259`). `driverEnv` sets `BEX_AGENT_GITHUB_CLI=0` only from `workspaceEnv.GitHubCLIDisabled` (`lego/backend/internal/agentsessions/service.go:1908`).
  - **No level below the workspace has a field.** `AgentConfig` has Agent, Model, ModelEndpoint, Task and Template (`lego/backend/internal/agentsessions/models.go:110`). `AutomationAgentConfig` has Agent, Model and Template (`automations.go:74`). The automation runs (`automation_scheduler.go:273`, `automations.go:666`) and spawns (`spawn.go:108`) copy only those.
  - **No schema change for the lower levels.** A session's `agent_config` and an automation's `template` are already stored as JSON (`lego/backend/internal/store/agentsessions.go:211`, `agentautomations.go:30`). The workspace's automations default and mandatory flag need columns on `agent_environments`. The highest migration on `origin/main` is `0326` at triage; re-check before pushing.
  - **The wire already has a named refusal.** `RefusalGitHubCLIOff` (`lego/backend/internal/agentsession/contract.go:63`) maps to `ErrGitHubCLIOff` in `agentsession/http.go:207`, and the gateway's text is the workspace one (`githubCLIOffRefusal`, `lego/backend/internal/sshgateway/agentcred/githubapi.go:79`). The overlay's `gh`-off line says "this workspace's admins" (`lego/agent-image/driver/src/overlay.ts:187`).
  - **It is more than one sitting.** It touches a migration and the store, three API surfaces, the mint, the gateway, the driver and its overlay, the CLI, three dashboard surfaces, two ADRs and a live check.
- **Reference product (Devin):** [Security profiles](https://docs.devin.ai/product-guides/security-profiles), re-read 2026-11-22.
  - **Levels.** Profiles bind at the enterprise default, the organization default, the automations default, an automation and a session. "The most specific binding wins, unless a mandatory profile higher up locks things down". "A mandatory profile is a floor that lower levels cannot escape", and under one "Lower-level selections can only tighten, never loosen".
  - **Child sessions.** "Sessions spawned by other sessions (child sessions) are governed by the same chain as their parent".
  - **Who may select.** "Members without the permission can't enumerate, select, or change profiles". The "Manage security profiles" permission is held by org admins by default. Under a recommended (not mandatory) default, "Anyone (with the appropriate permission) at a lower level can pin a different profile or opt out entirely".
  - **Where it is chosen.** A session's profile is chosen "when it's created (via the options menu in the session start box)". An automation's is pinned in its editor or with `security_profile` on create or update.
  - **`gh`.** A profile "can remove the GitHub CLI token from the machine", and with a mandatory profile "The GitHub CLI token is removed if either profile removes it".
  - **When a change applies.** Changes reach "new sessions immediately, and for existing sessions the next time they resume".
  - **Scope here.** bex has no enterprise level above the workspace, so the chain here starts at the workspace (Devin's organization default).
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../docs/ADR090-agent-owned-delivery.md)). It completes [w10/m524](../done/m524/README.md)'s `gh` setting with Devin's narrower bindings.
- **Expected outcome:**
  - An admin keeps `gh` for interactive sessions but turns it off for a scheduled automation, or for every automation through the automations default. The reverse works too.
  - A member turns `gh` off for one session.
  - A workspace that must never call the GitHub API from a sandbox locks the setting, so nothing below turns it back on.
- **Why now:** since w10/m522, `gh` writes on the session's repository, and since w10/m538 the `bex` tools merge. A workspace that wants those writes for people at the keyboard but not for unattended automations has no choice short of turning `gh` off everywhere.
- **Render parity included (reference product: Devin):** the bindings are tenant-facing on REST, GraphQL, MCP, the CLI and the dashboard. t009 checks those surfaces against each other and the chain against Devin's security profiles.
