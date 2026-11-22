# w10 · m561 — An automation or a session can turn `gh` off for itself, and an admin can make the workspace's `gh` off mandatory, as Devin's security profiles bind per automation and per session

**Worker:** worker10 **Goal:** `gh` access resolves through Devin's binding chain instead of one workspace switch. A session create and an automation template carry `agentConfig.githubCli`. The agent environment gains an automations default and a mandatory flag. The mints, the dispatch and the overlay use the most specific binding, unless the workspace's `gh` off is mandatory. A refusal names the level that turned `gh` off. REST, GraphQL, MCP, the CLI and the dashboard read and write each level. **Status:** todo (t001–t007 and t009–t011 done; t008's live check and t012 remain)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: names, the binding chain and its precedence, who may set each level, when a change applies, the refusal and driver contract; ADR047 and ADR090 amended — **DONE** | 45m | — |
| [t002](done/t002.md) | Backend: the agent environment gains the automations default and the mandatory flag on REST, GraphQL and MCP — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Backend: a session create and an automation template carry `agentConfig.githubCli`, refused where it would loosen a mandatory or admin-set binding, and child sessions inherit it — **DONE** | 1h30m | t001, t002 |
| [t004](done/t004.md) | Backend and gateway: one resolver feeds the mints and `driverEnv`, and the route's refusal names the level — **DONE** | 1h30m | t002, t003 |
| [t005](done/t005.md) | Driver: the overlay says which level turned `gh` off — **DONE** | 30m | t004 |
| [t006](done/t006.md) | CLI: `--github-cli` on `bex agent run`/`chat`/`handoff` and `bex agent automations`, and the two new `bex agent env set` flags — **DONE** | 1h | t002, t003 |
| [t007](done/t007.md) | Dashboard: the session composer, the automation form and Settings → Agents set each level, with matching skeletons — **DONE** | 2h | t002, t003 |
| [t008](t008.md) | Live check on production: a session's and an automation's own `gh` off, and a mandatory workspace off that no create can loosen | 45m | t004, t005, t006, t007 |
| [t009](done/t009.md) | Render parity (Devin) — **DONE** | 30m | t008 |
| [t010](done/t010.md) | Simplify — **DONE** | 20m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009, t010 |
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

## Design record (t001, 2026-11-22)

Each choice matches Devin's [Security profiles](https://docs.devin.ai/product-guides/security-profiles) (quoted under Source + Goal linkage) unless it says otherwise. The full text is in [ADR047 D2's w10/m561 amendment](../../../docs/ADR047-cloud-coding-agent-sessions.md#automations-and-sessions-bind-their-own-gh-over-a-mandatory-workspace-floor-amendment-w10m561-2026-11-22) and [ADR090 D4](../../../docs/ADR090-agent-owned-delivery.md#d4--agent-facing-tool-the-bex-stdio-mcp-server).

- **Names.** As proposed, plus one flag:
  - `agentConfig.githubCli` (optional boolean) on a session create and an automation template, and on their reads. GraphQL `AgentSessionConfig(Input).githubCli` and `AgentAutomationAgentConfig(Input).githubCli`.
  - On the environment, `automationsGithubCliEnabled` (nullable, `null` inherits) and `githubCliMandatory` (boolean, default `false`). Columns `automations_github_cli_enabled boolean` (nullable) and `github_cli_mandatory boolean NOT NULL DEFAULT false`, migration 0328. Their zero value inherits and is not mandatory.
  - **Added:** `resetAutomationsGithubCli: true` returns the automations default to inherit. graphql-go cannot carry an explicit `null`, the same reason `resetAgentModelMonthlySpendCap` exists. REST and MCP also take the `null`. Setting and resetting it in one update is `400 AGENT_ENVIRONMENT_GITHUB_CLI_INVALID`.
  - CLI: `--github-cli on|off|inherit` on `bex agent run`/`chat`/`handoff` (`inherit` sends nothing) and on `bex agent automations create`/`update` (where `inherit` removes an automation's binding), `bex agent env set --automations-github-cli on|off|inherit` and `--github-cli-mandatory on|off`.
  - Codes, `403` with `params.field` and `params.reason` (`mandatory` or `admin_required`): `AGENT_SESSION_GITHUB_CLI_LOCKED`, `AGENT_AUTOMATION_GITHUB_CLI_LOCKED`, `AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED`.
  - Wire: `X-Bex-Agent-Credential-Refusal: github-cli-off` unchanged, plus `X-Bex-Agent-Credential-Refusal-Level: workspace|automations-default|automation|session`. Driver: `BEX_AGENT_GITHUB_CLI_LEVEL` beside `BEX_AGENT_GITHUB_CLI=0`.
- **Precedence** (Devin: "The most specific binding wins, unless a mandatory profile higher up locks things down"). The chain runs workspace, then for an automation's session the automations default and the automation, then the session. The most specific non-absent binding wins. ADR047 has the table of every pair in both directions.
  - **Mandatory** is the workspace's off as a floor, matching "A mandatory profile is a floor that lower levels cannot escape". It has no effect while the workspace is on, because a lower level may always turn `gh` off. It locks the automations default too: a `true` there is refused on save, and one stored earlier has no effect.
  - Saving the workspace on neither clears nor refuses the flag. It just has no effect until the workspace is off again, as a mandatory Devin profile that keeps the token removes nothing.
- **Child sessions** ("governed by the same chain as their parent"). A child resolves its tree's chain: the root's binding and the root's automation, then any session below the root that binds `false`. A child's own `true` is stored but never loosens. A spawn names nothing. A child whose parent was deleted is top-level, as ADR102 already makes it.
- **Who may set which direction.** Anyone who may create the session or save the automation may set `false`. `true` needs a workspace admin (`can_manage`, bex's "Manage security profiles"), and nobody may set it under a mandatory off.
  - `true` needs an admin even when the inherited value is on. A pinned on outlives a later workspace off that is not mandatory, and that is a selection Devin reserves for the permission.
  - An automation update is checked only when the value changes to `true`, so its creator's save keeps an admin's on.
  - The environment's two fields are admin-only, like every environment field.
  - **Divergence:** Devin gates every selection, an opt-out included. bex lets members turn `gh` off, which only removes access. The stricter variant is filed as [w10/044](../044.md).
- **Where it resolves, and when a change applies.** One resolver serves both `gh` mints and every dispatch: `agentsession.LoadGitHubCLIChain` and `GitHubCLIChain.Resolve` in `lego/backend/internal/agentsession/githubcli.go`. It reads the environment row, the session's tree (at most three ancestors) and the root's automation's current template, live. So an automation or environment edit refuses a running session's next `gh` call and reaches the driver at its next turn.
  - An automation's session does not copy the template's value at fire time. The automation level therefore stays nameable, and an edit applies to its running sessions.
  - A deleted automation reads as inherit, and its sessions still take the automations default.
  - A session's own binding is set at create only, as Devin's is chosen "when it's created".
- **Refusal and driver contract.** Keeping `github-cli-off` means an older gateway answers with the workspace's words, never `gh` on. A newer gateway reads an absent or unknown level as the workspace's.
  - The route's message for each level says how that level turns `gh` back on (texts in ADR047), and the workspace's text is unchanged.
  - The metric label stays `result="github_cli_off"`, and bex-api's denied-mint log line carries the level. The audit row has no place for it.
  - The overlay names the level, and an absent level reads exactly as before.

## Render parity (Devin) record (t009, 2026-11-22)

- **The surfaces agree.** Each field reads and writes the same way on every surface, with the same refusals:
  - **Session (`agentConfig.githubCli`).** REST `POST`/`GET /v1/agent-sessions`, GraphQL `createAgentSession`/`agentSession` and MCP `create_agent_session`/`get_agent_session` store the binding and read it back. A member's `true` is refused `AGENT_SESSION_GITHUB_CLI_LOCKED` (`admin_required`) on all three, an admin's is accepted, and under a mandatory off an admin's `true` is refused `mandatory` while a member's `false` is accepted. Pinned by `TestSessionGitHubCLIBindingOnEverySurface`.
  - **Automation (`template.agentConfig.githubCli`).** REST, GraphQL and MCP create and read it the same way. A member's `true` is refused `AGENT_AUTOMATION_GITHUB_CLI_LOCKED` on all three. A creator's save that leaves an admin's `true` alone keeps it, and a change to `true` is refused. Pinned by `TestAutomationGitHubCLIBindingOnEverySurface`.
  - **Environment (`automationsGithubCliEnabled`, `githubCliMandatory`).** REST, GraphQL and MCP read `null`/`false` with no row, and an admin's write on any surface reads back on the others. The explicit `null` (REST, MCP) and `resetAutomationsGithubCli` (all three) return the default to inherit. A member's write is refused on each, and an automations `true` under a mandatory off is refused `AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED` on each. Pinned by `TestEnvironmentGitHubCLIBindingsOnEverySurface`.
  - **CLI.** `bex agent run`/`chat`/`handoff --github-cli on|off|inherit`, `bex agent automations create`/`update --github-cli on|off|inherit` and `export`/`apply` speak REST. So do `bex agent env set --automations-github-cli on|off|inherit` and `--github-cli-mandatory on|off`. `bex agent env`, `bex agent get` and `bex agent automations get` print each binding, reading a field missing from an older bex-api as inherit and not mandatory.
  - **Dashboard.** The composer's toolbar, the automation form and Settings → Agents read and write each level through GraphQL. A shared inherit/on/off picker shows what inherit resolves to. Its "on" is disabled, with the reason, under a mandatory off and for a member, while "off" is open to anyone, as the API allows. Settings → Agents adds the mandatory switch and the automations select, whose inherit sends `resetAutomationsGithubCli`. Each refusal code shows as that control's error, and the automation settings card shows the binding.
- **Devin** ([Security profiles](https://docs.devin.ai/product-guides/security-profiles), re-read 2026-11-22):
  - **Levels:** organization default, automations default, automation and session. **Match**, with no enterprise level above the workspace.
  - **Precedence:** "the most specific binding wins". **Match**: `TestGitHubCLIChainResolve` pins each pair in both directions.
  - **Mandatory floor:** "a floor that lower levels cannot escape", where they "can only tighten". **Match**: no lower level turns `gh` on, any lower level may turn it off, and a save that would loosen is refused.
  - **Child sessions:** "governed by the same chain as their parent". **Match**: a child resolves its tree's chain and can only turn `gh` off.
  - **Who may select:** "Manage security profiles", and members "can't … select". **Partial**: `true` needs `can_manage`, as Devin's selection does, but a member may set `false`. That only removes access. The stricter variant is filed as [w10/044](../044.md).
  - **Where it is chosen:** the session start box's options menu, and the automation's editor or `security_profile`. **Match**: the composer's options, the automation form and the template field.
  - **When a change applies:** "new sessions immediately, and for existing sessions the next time they resume". **Match for an on, and sooner for an off**: a running session's next `gh` call is refused, as w10/m524 already decided for the workspace switch.
- **Filed:** [w10/044](../044.md). No other gap users would miss: Devin's other profile controls (network policy, MCP allowlist, Git access level) and its enterprise level are out of scope.

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
