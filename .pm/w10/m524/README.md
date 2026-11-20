# w10 · m524 — A workspace admin can turn off its sessions' `gh` API access, as a Devin security profile can remove the `gh` token

**Worker:** worker10 **Goal:** the workspace agent environment gains a GitHub CLI setting, on by default. When an admin turns it off, the workspace's sessions get no `gh` configuration, and the gateway's `github-api` route refuses their calls with a message that names the setting. Git pushes to the session branch and the `bex` MCP tools keep working. REST, GraphQL, MCP, the CLI and Settings → Agents all read and write the setting. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: the setting's name, default and scope, where it is enforced and when a change applies; ADR047 and ADR090 amended | 45m | — |
| [t002](t002.md) | Backend: the agent environment stores the setting and serves it on REST, GraphQL and MCP | 1h30m | t001 |
| [t003](t003.md) | Backend and gateway: a session's `gh` mints are refused while the setting is off, and the route's message names it | 1h30m | t002 |
| [t004](t004.md) | Driver: a session dispatched with the setting off gets no `gh` socket or `GH_*` environment, and the overlay says why | 45m | t001 |
| [t005](t005.md) | CLI: `bex agent env set --github-cli on\|off`, shown by `bex agent env` | 45m | t002 |
| [t006](t006.md) | Dashboard: a GitHub CLI switch in Settings → Agents, with its skeleton matching | 1h30m | t002 |
| [t007](t007.md) | Live check on production: with the setting off a session's `gh` is refused naming it, and turned back on it works | 45m | t003, t004, t005, t006 |
| [t008](t008.md) | Render parity (Devin) | 30m | t007 |
| [t009](t009.md) | Simplify | 20m | t008 |
| [t010](t010.md) | Test coverage | 45m | t008, t009 |
| [t011](t011.md) | Closeout | 10m | t010 |

Total: **11 tasks, about 9h15m**.

## Definition of done

1. **A setting, on by default.** The workspace agent environment carries `githubCliEnabled` (the final name is t001's), `true` for a workspace with no row and after the migration. A workspace admin sets it through REST `PATCH /v1/agent-environment`, GraphQL, MCP `update_agent_environment`, `bex agent env set` and Settings → Agents. Every read (REST `GET /v1/agent-environment`, GraphQL, MCP `get_agent_environment`, `bex agent env`, Settings → Agents) shows it. A non-admin cannot change it, exactly as for the other environment fields. The change is audited as the other environment changes are.
2. **Off means no `gh` in the sandbox.** A repo session whose turn is dispatched while the setting is off gets no `gh` socket, no `gh` config and no `GH_*` variables (`lego/agent-image/driver/src/github-cli.ts`). Its overlay says that `gh` is turned off for the workspace and that pull requests still go through the `bex` tools.
3. **Off means the route refuses.** While the setting is off, every call on the session's `github-api` route, read or write, answers `403` in GitHub's error shape before any token is minted. The message names the setting and how an admin turns it back on. The refusal holds for a session already running when the setting was turned off, from its next `gh` call. The denied mint is audited. The Git proxy, pushes to the session branch, `open_or_update_draft_pr` and `reply_on_pull_request` are unaffected.
4. **Said everywhere.** ADR047 and ADR090 D4 describe the setting, its default, where it is enforced and when a change applies. The MCP tool descriptions, the GraphQL field descriptions and the CLI help name it.
5. **Live.** On production, once a deploy carries the milestone: in the QA user's workspace, with the setting off, one short repo session's `gh run list` is refused with the message naming the setting and its overlay says `gh` is off. With the setting back on, the next session's `gh run list` answers. The setting is left on.

## Source + Goal linkage

- **Source:** inbox note [w10/026](../done/026.md), filed by w10/m513's worker from its Devin comparison (t007) on 2026-11-19, promoted by `/loopx w10` on 2026-11-19.
- **Triage evidence (2026-11-19, `1ac45e591`):**
  - **No switch exists.** The workspace agent environment row holds the network tier, custom hosts, default model and template, spend cap, model key flag, disabled agents and data-use acknowledgements, and nothing about `gh` (`lego/backend/internal/store/agent_environments.go:30`). The driver configures `gh` for every repo session (`githubCliEnabled`, `lego/agent-image/driver/src/github-cli.ts:67`, which reads only `deliver`, `repoUrl` and `repository`), and `driverEnv` passes nothing that could turn it off (`lego/backend/internal/agentsessions/service.go:1846`).
  - **The route has no workspace check.** `serveGitHubAPI` (`lego/backend/internal/sshgateway/agentcred/githubapi.go:96`) checks the path, the body and the admission budgets, then mints (`:163`). bex-api's `Minter.mint` (`lego/backend/internal/agentsession/mint.go:179`) checks the session's lifecycle, Pod and branch and never reads the workspace's agent environment. The internal mint hop already carries a named refusal in `RefusalHeader` (`lego/backend/internal/agentsession/contract.go:58`, used for the missing model key), so a new refusal can reach the gateway without a wire break.
  - **It is more than one sitting.** It touches a migration and the store, three API surfaces, the mint and the gateway, the driver and its overlay, the CLI, the dashboard, two ADRs and a live check.
- **Reference product (Devin):** a Devin security profile can "remove the GitHub CLI token from the machine", and sessions under it "keep working with your repositories through Devin's git integration" but "cannot make direct GitHub API calls". Managing profiles needs a "Manage security profiles" permission that org admins hold by default. Profiles bind, from broadest to most specific, at the enterprise default, the organization default, the automations default, an automation and a session ([Security profiles](https://docs.devin.ai/product-guides/security-profiles), read 2026-11-19). The token is on the machine unless a profile removes it, so on by default matches. This milestone ships the workspace-wide switch, which is Devin's organization default and is enough to reach the note's end state. A per-automation or per-session override is a stricter variant, and t008 files it as its own item instead of holding this milestone for it.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../docs/ADR090-agent-owned-delivery.md)). It follows [w10/m513](../done/m513/README.md), which gave every repo session a read-only `gh`, and [w10/m522](../blocked/m522/README.md), which lets that `gh` write comments, reviews, labels, issues and CI re-runs.
- **Expected outcome:** an admin whose policy is that agents reach GitHub only through Git and the `bex` tools turns `gh` off once in Settings → Agents, and no session in the workspace can call the GitHub API from then on.
- **Why now:** since w10/m522, `gh` writes on the session's repository: it comments, labels, opens issues and re-runs CI. A workspace that does not want that has no way to opt out, while Devin's admins do.
- **Render parity included (reference product: Devin):** the setting is a tenant-facing surface on REST, GraphQL, MCP, the CLI and the dashboard. t008 checks those surfaces against each other and the setting against Devin's security profiles.
- **Related:** [w10/027](../027.md) gives the session's web shell and SSH terminal the same `gh`. Whichever of the two ships second makes the terminal's `gh` obey this setting too.
