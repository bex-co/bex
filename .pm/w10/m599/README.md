# w10 · m599 — A workspace admin changes a session's `gh` binding from its settings after it starts, as Devin changes a session's security profile

**Worker:** worker10 **Goal:** an admin who started a session with `gh` off, or on, changes it without starting a new session. A workspace admin sets a session's `agentConfig.githubCli` to inherit, on or off after create on REST, GraphQL, MCP, the CLI and the session's settings in the dashboard. A member is refused, and nobody turns `gh` on under a mandatory off. The session's next `gh` mint and next dispatch follow the change, and a child still only tightens its tree. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: the verb's shape on each surface, which sessions take a change, who may make one, when it applies, what the session level's refusal and overlay say, and what the session's settings show; ADR047 D2, ADR094 and ADR018 amended | 45m | — |
| [t002](t002.md) | Backend: the change verb on REST, GraphQL and MCP over the store, with the refusals, the audit, and the session level's refusal text | 1h45m | t001 |
| [t003](t003.md) | CLI: change a session's binding with `--github-cli on\|off\|inherit`, and `bex agent get` shows it | 45m | t002 |
| [t004](t004.md) | Dashboard: the session's settings offer the GitHub CLI binding, admin-only, and the GraphQL clients are regenerated | 1h15m | t002 |
| [t005](t005.md) | Live check: an admin turns a running repo session's `gh` off and back to inherit, and a member is refused | 45m | t002, t003, t004 |
| [t006](t006.md) | Render parity (Devin) | 30m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 7h**.

## Definition of done

1. **A change verb on every surface.**
   - A workspace admin changes a session's `agentConfig.githubCli` to `true`, `false` or inherit after create, on REST, GraphQL and MCP, under the names t001 settles.
   - The CLI does the same with `--github-cli on|off|inherit`, and the session's settings in the dashboard do it too.
   - Every session read (REST, GraphQL `AgentSession.agentConfig.githubCli`, MCP `get_agent_session`, `bex agent get`) returns the changed binding.
2. **The same refusals as at create.**
   - A member's change is refused `403 AGENT_SESSION_GITHUB_CLI_LOCKED` with `params.reason: admin_required`, whichever direction it goes, inherit included.
   - Anyone's `true` under the workspace's mandatory off is refused with `reason: mandatory`.
   - A change that keeps the stored binding is never refused.
   - Sessions t001 rules out (for example archived or deleted) get the refusal t001 names, and the session stays as it was.
3. **The chain follows the change.**
   - The session's next `gh` mint resolves the changed binding: an off refuses the next call naming the session level, and inherit or on serves it.
   - The next dispatch's driver gets `BEX_AGENT_GITHUB_CLI` and `BEX_AGENT_GITHUB_CLI_LEVEL` as the changed chain resolves.
   - A child's own `true` still never turns `gh` on. A change on a root reaches its whole tree.
4. **Said everywhere.**
   - The session level's `gh` refusal (`lego/backend/internal/sshgateway/agentcred/githubapi.go`) and the driver overlay's session line (`lego/agent-image/driver/src/overlay.ts`) say how a workspace admin changes it, not "start a new session".
   - ADR047 D2 ("set at create only", "A session's own binding is fixed at create") and the ADR018 row drop the gap and state the verb.
   - ADR094 lists the MCP tool. The REST, GraphQL and MCP descriptions and the CLI help say who may change it and when the change applies.
5. **Tested.**
   - Backend tests cover the verb on all three surfaces: an admin's change in each direction, a member's refusal, the mandatory refusal, an unchanged save, the next mint and the next dispatch env after a change, and a child under a changed root.
   - CLI and dashboard tests cover the change and its refusals. The dashboard's and `mobile/src/generated-graphql/index.ts` GraphQL clients are regenerated.
6. **Live.**
   - On production, once the pin carries the milestone, a workspace admin in `tian-personal` turns one short repo session's `gh` off and its next `gh` call is refused naming the session level, then back to inherit and the next turn has `gh`.
   - A member's change is refused there or on `dev-10`.
   - Every session the check creates is deleted.

## Source + Goal linkage

- **Source:** inbox note [w10/050](../done/050.md), filed by w10/044's Devin check on 2026-11-23 and promoted by `/loopx w10` on 2026-11-23.
- **Triage evidence (2026-11-23, `cf088e38f6`):**
  - **No verb takes it after create.** `refuseGitHubCLIBinding` (`lego/backend/internal/agentsessions/githubcli.go:80`) guards a session only in `Create` (`service.go:699`, `stored` nil). The store writes `agent_config` after create only for `model` (`lego/backend/internal/store/agentsessions.go:696`). The `/v1/agent-sessions/{id}` routes are `cancel`, `tags`, `archive`, `unarchive`, `DELETE`, `steer`, `interrupt`, `attach-ticket` and `queued-messages`: there is no `PATCH` (`rest.go`, `queue_rest.go`). GraphQL's `githubCli` input exists only on `AgentSessionConfigInput` (`graphql.go:37`). MCP's only session update is `set_agent_session_tags`.
  - **The surfaces say so.** The session level's `gh` refusal reads "A session keeps that choice: start a new session without `--github-cli off` to have gh" (`sshgateway/agentcred/githubapi.go:85-86`). The overlay says "`gh` is turned off for this session, which was started with it off" (`lego/agent-image/driver/src/overlay.ts:185`). ADR047 D2 says "set at create only" and links this note.
  - **No settings surface yet.** The session header's menu (`dashboard/src/features/agent-sessions/components/session-detail-header.tsx:393-430`) offers tags, archive and delete; `GithubCliPicker` (`github-cli-picker.tsx`) and `githubCliLock` serve only the composer and the automation form. The CLI's only session-edit verb is `bex agent tags` (`lego/cli/internal/agent/lifecycle.go:100`).
  - **The resolver needs nothing new.** `agentsession.ResolveGitHubCLI` and `LoadGitHubCLIChain` (`lego/backend/internal/agentsession/githubcli.go:116`, `:145`) read the session's `agent_config` live, with no cache, at every mint and dispatch. A stored change is what the next mint and dispatch see.
- **Devin (re-read 2026-11-23, [Security profiles](https://docs.devin.ai/product-guides/security-profiles)):** a session's profile is "chosen for an individual session when it's created" and "changed later from the session's settings". "A session resolves its governing profile at the moments it boots": edits "do not automatically propagate to sessions that are already running", and changes take effect "for existing sessions the next time they resume". "Members without the permission can't enumerate, select, or change profiles." The v3 API pins a profile to a session only "when you … create a session": the page documents no API to change one later.
- **Goal linkage:** cloud coding-agent parity with Devin ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)), the security-profile chain w10/m561 built and w10/044 gated to admins.
- **Expected outcome:** an admin fixes a session's `gh` in place, from the session or any API surface, instead of losing the session's context to a new one.
- **Why now:** the chain and its admin gate shipped this week (w10/m561, w10/044), and the session level is the one Devin binding bex cannot change after it is set. The refusal text sends users to start over.
- **Render parity task:** included, against Devin, because the milestone adds REST, GraphQL, MCP, CLI and dashboard surface.
