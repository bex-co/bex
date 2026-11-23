# w10 · m561 — An automation or a session can turn `gh` off for itself, and an admin can make the workspace's `gh` off mandatory, as Devin's security profiles bind per automation and per session

**Worker:** worker10 **Goal:** `gh` access resolves through Devin's binding chain instead of one workspace switch. A session create and an automation template carry `agentConfig.githubCli`. The agent environment gains an automations default and a mandatory flag. The mints, the dispatch and the overlay use the most specific binding, unless the workspace's `gh` off is mandatory. A refusal names the level that turned `gh` off. REST, GraphQL, MCP, the CLI and the dashboard read and write each level. **Status:** done (t001–t012 done; t008's live check held on production pin `1a5a8e872398` on 2026-11-23)

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
| [t008](done/t008.md) | Live check on production: a session's and an automation's own `gh` off, and a mandatory workspace off that no create can loosen — **DONE** | 45m | t004, t005, t006, t007 |
| [t009](done/t009.md) | Render parity (Devin) — **DONE** | 30m | t008 |
| [t010](done/t010.md) | Simplify — **DONE** | 20m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009, t010 |
| [t012](done/t012.md) | Closeout — **DONE** | 10m | t011 |

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

Each choice matches Devin's [Security profiles](https://docs.devin.ai/product-guides/security-profiles) (quoted under Source + Goal linkage) unless it says otherwise. The full text is in [ADR047 D2's w10/m561 amendment](../../../../docs/ADR047-cloud-coding-agent-sessions.md#automations-and-sessions-bind-their-own-gh-over-a-mandatory-workspace-floor-amendment-w10m561-2026-11-22) and [ADR090 D4](../../../../docs/ADR090-agent-owned-delivery.md#d4--agent-facing-tool-the-bex-stdio-mcp-server).

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
  - **Divergence:** Devin gates every selection, an opt-out included. bex lets members turn `gh` off, which only removes access. The stricter variant is filed as [w10/044](../../044.md).
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
  - **Who may select:** "Manage security profiles", and members "can't … select". **Partial**: `true` needs `can_manage`, as Devin's selection does, but a member may set `false`. That only removes access. The stricter variant is filed as [w10/044](../../044.md).
  - **Where it is chosen:** the session start box's options menu, and the automation's editor or `security_profile`. **Match**: the composer's options, the automation form and the template field.
  - **When a change applies:** "new sessions immediately, and for existing sessions the next time they resume". **Match for an on, and sooner for an off**: a running session's next `gh` call is refused, as w10/m524 already decided for the workspace switch.
- **Filed:** [w10/044](../../044.md). No other gap users would miss: Devin's other profile controls (network policy, MCP allowlist, Git access level) and its enterprise level are out of scope.

## Live check (t008, production, 2026-11-23)

Workspace `tian-personal` (`tea-da2isimlm39c739m4ofg`, the QA account's, whose GitHub connection is the `puncsky` installation), 04:56–05:08Z, pin `1a5a8e872398`: both `bex-api` Pods and both `bex-ssh-gateway` Pods ran `bex-operator@sha256:65ef4f0d…`, sessions the sandbox `bex-agent-sandbox@sha256:e374484c…` and the dashboard `bex-dashboard@sha256:5829a2c3…`, the digests that pin set (read-only `kubectl`). The CLI was built from `main` (`ba7e6c069e`), because the released v0.2.1 has no `--github-cli`; REST, GraphQL and MCP were called with the same account's OAuth token, and the dashboard was signed in through `scripts/qa-login.sh`. Before the check the workspace had `gh` on, not mandatory, and the automations default inheriting. Two sessions on `claude` with `deepseek/deepseek-v4.1-flash-1017`, $0.027 in all.

- **Fixture.** A new private repository, `puncsky/qa-20261122-m561-gh-off`, with a workflow `qa-pass` on pushes to `main`. Its run 45114753216 ended `success`.
- **Session level (DoD 6, step 2).** `bex agent run --github-cli off --repo puncsky/qa-20261122-m561-gh-off --max-spend 1` started `ags-dc1sf2vqbg1c739j177g` with the workspace's `gh` on. `env | grep '^GH_'` printed nothing (exit 1), `gh auth status` said "You are not logged into any GitHub hosts" (exit 1), and `gh run list --limit 1` asked for `gh auth login` (exit 4). The agent quoted its overlay: "There is no GitHub token in the sandbox, and `gh` is turned off for this session, which was started with it off, so `gh` and direct GitHub API calls do not work here; do not try to authenticate it." REST `GET /v1/agent-sessions/<id>`, GraphQL `agentSession` and MCP `get_agent_session` read `agentConfig.githubCli: false`, and `bex agent get` printed `GitHub CLI off`.
- **Automation level (step 3).** `bex agent automations create --github-cli off` (a one-off fire on 2030-01-01) made `aut-dc1sgggoodks73aud17g`, whose `template.agentConfig.githubCli` read `false` on REST, GraphQL and MCP. `bex agent automations run --wait` started `ags-dc1sgmnqbg1c739j17dg`, whose own `agentConfig` has no `githubCli` (the automation level is read live, not copied). Its turn had no `GH_*`, `gh auth status` exit 1 and `gh run list` exit 4, and the overlay said "the automation that started this session turned `gh` off for its sessions".
- **An automation edited mid-turn (step 5).** `bex agent automations update --github-cli inherit`, then a follow-up turn in the same session: its first `gh run list --limit 1` answered the fixture's run (exit 0). At 05:02:15Z, while the turn slept, `--github-cli off` saved, and the turn's next `gh run list` exited 1 with "HTTP 403: bex: gh is turned off for the sessions of the automation that started this session (its template's agentConfig.githubCli). Its creator turns it back on by setting it to inherit in the automation's settings or with `bex agent automations update <automation> --github-cli inherit`; turning it on outright needs a workspace admin. Pushes to your session branch and the bex MCP tools … still work." The gateway logged `agent github api: refused (… status=403 result=github_cli_off)` and bex-api `credential mint denied … gh is turned off for this agent session (the automation binding)` at 05:03:20Z.
- **Mandatory floor (step 4).** From 05:06:12Z to 05:07:03Z the workspace had `gh` off and mandatory (`bex agent env set --github-cli off --github-cli-mandatory on`).
  - REST `POST /v1/agent-sessions` with `agentConfig.githubCli: true` answered `403` `AGENT_SESSION_GITHUB_CLI_LOCKED`, `params` `{"field": "agentConfig.githubCli", "reason": "mandatory"}`, message "this workspace's admins made its gh off mandatory, so nothing turns gh on for an automation or a session; set it to false or leave it out to inherit". GraphQL `createAgentSession` (`extensions.code`), MCP `create_agent_session` and `bex agent run --github-cli on` were refused with the same code and text.
  - The composer, opened before the save with **On** chosen, was refused on **Start session**: GraphQL answered `AGENT_SESSION_GITHUB_CLI_LOCKED` (`mandatory`) and the composer showed "Couldn't start the session. This workspace's GitHub CLI off is mandatory, so a session can't turn gh on. Choose Off or Inherit." Reloaded, the control read "Inherit (off)" and its **On** was disabled with "Unavailable while this workspace's GitHub CLI off is mandatory."
  - Under the floor, `bex agent automations update --github-cli on` was refused `AGENT_AUTOMATION_GITHUB_CLI_LOCKED` and `bex agent env set --automations-github-cli on` `AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED`.
- **Cleanup.** `bex agent env set --github-cli on --github-cli-mandatory off` restored the workspace at 05:07:03Z: REST and GraphQL read `githubCliEnabled: true`, `automationsGithubCliEnabled: null`, `githubCliMandatory: false`, and `bex agent env` prints `GitHub CLI on` and `Automations GitHub CLI inherit`. The automation and both sessions are deleted (`404 AGENT_AUTOMATION_NOT_FOUND`, `404 AGENT_SESSION_NOT_FOUND`; no sandbox Pod is left); neither session pushed a branch or opened a pull request. The repository is deleted (`Could not resolve to a Repository`). The dashboard's workspace selection is back on `bex`, and the QA browser session is logged out.

The Definition of done holds. Items 1–5 hold in t002–t007 and t011's tests, t009's surface check and the ADR047 and ADR090 amendments, and item 1's reads, item 3's refusals, item 4's resolver, refusal text and overlay, and item 6 as written held live above. t012 closed the milestone on 2026-11-23.

## Unblock record (2026-11-23)

- **Affected:** w10/m561 t008 and t012.
- **Source:** verified evidence. The newest `chore(deploy): pin platform images to <sha>` on `origin/main` is `e6c65f5c62` (pin `1a5a8e872398`, 04:25Z on 2026-11-23), and `git merge-base --is-ancestor ba013e3a75 1a5a8e872398` holds. Both `bex-api` Pods and both `bex-ssh-gateway` Pods run `bex-operator@sha256:65ef4f0d…`, bex-api's `BEX_AGENT_SESSION_IMAGE` is `bex-agent-sandbox@sha256:e374484c…` and the dashboard runs `bex-dashboard@sha256:5829a2c3…`, the digests that pin set (read-only `kubectl`). The pin gate was the only one.
- **Next:** t008's live check as written, in the QA account's `tian-personal` (`bex-canary` has no GitHub connection), on a fixture repository it deletes, with at most three short sessions on the cheapest model. The workspace is left with `gh` on, not mandatory, and the automations default inheriting. Then t012 closes the milestone if DoD 6 holds.

## Source + Goal linkage

- **Source:** inbox note [w10/035](../../done/035.md), filed by w10/m524's Devin comparison (t008) on 2026-11-19 and promoted by `/loopx w10` on 2026-11-22.
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
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../../docs/ADR090-agent-owned-delivery.md)). It completes [w10/m524](../../done/m524/README.md)'s `gh` setting with Devin's narrower bindings.
- **Expected outcome:**
  - An admin keeps `gh` for interactive sessions but turns it off for a scheduled automation, or for every automation through the automations default. The reverse works too.
  - A member turns `gh` off for one session.
  - A workspace that must never call the GitHub API from a sandbox locks the setting, so nothing below turns it back on.
- **Why now:** since w10/m522, `gh` writes on the session's repository, and since w10/m538 the `bex` tools merge. A workspace that wants those writes for people at the keyboard but not for unattended automations has no choice short of turning `gh` off everywhere.
- **Render parity included (reference product: Devin):** the bindings are tenant-facing on REST, GraphQL, MCP, the CLI and the dashboard. t009 checks those surfaces against each other and the chain against Devin's security profiles.
