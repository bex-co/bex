# w10 · m599 — A workspace admin changes a session's `gh` binding from its settings after it starts, as Devin changes a session's security profile

**Worker:** worker10 **Goal:** an admin who started a session with `gh` off, or on, changes it without starting a new session. A workspace admin sets a session's `agentConfig.githubCli` to inherit, on or off after create on REST, GraphQL, MCP, the CLI and the session's settings in the dashboard. A member is refused, and nobody turns `gh` on under a mandatory off. The session's next `gh` mint and next dispatch follow the change, and a child still only tightens its tree. **Status:** done (2026-11-24: t001–t004, t006–t008 done 2026-11-23; t005's live check held on production's pin `f46c44d9ddb1` and on `dev-10`; t009 closed it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: the verb's shape on each surface, which sessions take a change, who may make one, when it applies, what the session level's refusal and overlay say, and what the session's settings show; ADR047 D2, ADR094 and ADR018 amended — **DONE** | 45m | — |
| [t002](done/t002.md) | Backend: the change verb on REST, GraphQL and MCP over the store, with the refusals, the audit, and the session level's refusal text — **DONE** | 1h45m | t001 |
| [t003](done/t003.md) | CLI: change a session's binding with `--github-cli on\|off\|inherit`, and `bex agent get` shows it — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Dashboard: the session's settings offer the GitHub CLI binding, admin-only, and the GraphQL clients are regenerated — **DONE** | 1h15m | t002 |
| [t005](done/t005.md) | Live check: an admin turns a running repo session's `gh` off and back to inherit, and a member is refused — **DONE** | 45m | t002, t003, t004 |
| [t006](done/t006.md) | Render parity (Devin) — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006, t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

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

## Design record (t001, 2026-11-23)

Checked against Devin's [Security profiles](https://docs.devin.ai/product-guides/security-profiles), re-read 2026-11-23: a session's profile is "chosen for an individual session when it's created" or "changed later from the session's settings"; "Changes take effect for new sessions immediately, and for existing sessions the next time they resume"; "Members without the permission can't enumerate, select, or change profiles", but "anyone can see whether their session is governed by a profile". Devin's API pins a profile only at create, so the settings are its one place to change it. bex also serves the change on its API surfaces, as every bex dashboard verb is.

- **One general session update, not a `gh`-only verb.** Later session fields can join it without a new route, as Devin's session settings hold more than the profile.
  - **REST** `PATCH /v1/agent-sessions/{id}` with `{"agentConfig": {"githubCli": true|false|null}}`, decoded strictly. An explicit `null`, or `"resetGithubCli": true` beside `agentConfig`, returns the session to inherit; an omitted field keeps the binding. Any other `agentConfig` key is `400` (an unknown field); naming a binding and `resetGithubCli` together is `400 AGENT_SESSION_INPUT_INVALID` (`field: resetGithubCli`). An empty update returns the session unchanged.
  - **GraphQL** `updateAgentSession(id: String!, agentConfig: AgentSessionConfigUpdateInput, resetGithubCli: Boolean): AgentSession`, with `AgentSessionConfigUpdateInput { githubCli: Boolean }`. `resetGithubCli` stands for the null, as `resetAutomationsGithubCli` does.
  - **MCP** `update_agent_session` with `id`, `agentConfig.githubCli` (an explicit `null` inherits) and `resetGithubCli`, on `/mcp` only: it is an admin verb, and `/mcp/cloud-agents` keeps admin tools off.
  - **CLI** `bex agent update <ags-id> --github-cli on|off|inherit`, where `inherit` sends the explicit `null`. With no flag it is a usage error. `bex agent get` shows `GitHub CLI` for every repo session: `on`, `off`, or `inherit`.
  - **Audit.** The verb authorizes `can_operate` on the session (`AuthorizeOn`), which records it as every session verb is. The admin probe in `refuseGitHubCLIBinding` records nothing, as at create.
  - **Scopes.** Each surface is a write needing `agents:manage` on a bex API key, as tags are.
- **Which sessions take a change.** Every session that is not archived, in any phase, the binding being stored config that the next mint and dispatch read:
  - A completed or failed session takes it: its next turn, and an editor sandbox `resume_agent_session` starts, mint `gh` under it, as Devin's change reaches a session "the next time [it] resume[s]". A canceled session takes it too, as its tags do, though it runs no more turns.
  - A session still `creating` takes it: the mint reads it at once, and its first turn's driver gets what the chain resolved at create.
  - A repo-less session takes it, as create already stores a binding on one. It has no `gh`, so nothing reads it.
  - A child session takes it. Its own `true` is stored and read back but never turns `gh` on, as at create.
  - An archived session is refused `409 AGENT_SESSION_ARCHIVED`, as tags and every other mutation verb are (ADR065 D1), and keeps its binding. An unknown or deleted session is `404`.
- **Who.** The caller needs `can_operate` on the session (contributor and up, so a viewer is refused `403` before anything is read) and, for any change, `can_manage` on the workspace: `refuseGitHubCLIBinding(stored, new)` refuses a member's change `403 AGENT_SESSION_GITHUB_CLI_LOCKED` with `reason: admin_required` in either direction, inherit included, and anyone's `true` under a mandatory off with `reason: mandatory`. A save that keeps the stored binding is never refused and writes nothing, so a member's unchanged save cannot overwrite an admin's concurrent change. A workspace admin holds `can_operate` on every session of the workspace (`agent_session#can_operate` is `can_operate from workspace`), so no other rule is needed.
- **When it applies.** The resolver reads `agent_config` live at every mint and dispatch (`agentsession.LoadGitHubCLIChain`), so no new code is needed past the write.
  - An off refuses the session's next `gh` call at once, naming the session level, whether the call comes from the agent, the web Shell, SSH or Zed: all four reach the same `github-api` route. That is sooner than Devin's next resume, the safe direction ADR047 already records for an automation or environment edit.
  - An on, or an inherit that resolves on, serves the next mint at once, but a turn dispatched with `gh` off has no `gh` configured in its sandbox, so `gh` returns at the session's next turn (the next dispatch's driver env). Nothing else is needed for a running turn.
  - A change on a root reaches its whole tree, whose children resolve through it at their next mint and turn.
- **The words.**
  - The session level's `gh` refusal: "bex: gh is turned off for this agent session (its agentConfig.githubCli, or that of the session that started it). A workspace admin turns it back on by setting that session's GitHub CLI to inherit or on in its settings in the dashboard or with `bex agent update <session> --github-cli inherit`." followed by the sentence on pushes and the `bex` tools, as today.
  - The overlay's session line: "this session's GitHub CLI setting, or that of the session that started it, turned `gh` off". It ships in the agent image that `deploy.yml` builds from the same commit, so no separate pin is needed. An older driver's "which was started with it off" is wrong only about when, and still says `gh` is off.
  - The create field descriptions and help stop saying a binding is fixed at create, and the update verb's descriptions and help say only a workspace admin changes it and when it applies.
- **The dashboard.** A **Settings** item in the session header's "…" menu opens a **Session settings** dialog holding the GitHub CLI field (`GithubCliPicker`, field variant), shown for every repo session that is not archived (a repo-less session has no `gh`, and an archived one is read-only, as its tags are).
  - Every member sees it, so "anyone can see whether their session is governed by a profile" holds. A member's picker keeps every choice but the stored one disabled with "Only a workspace admin can change this.", as the composer does since w10/044; `on` under a mandatory off is disabled with its reason for everyone.
  - Inherit names what it resolves to, walking the chain as `agentsession.LoadGitHubCLIChain` does: a mandatory off wins; then the workspace's switch, or for a tree an automation started that automation's binding, else the automations default, else the switch; then the root's own binding; then any off below it. A child reads the sessions above it from the session page's tree query (cache-first), at most three, and an ancestor missing from the loaded tree ends the walk, as a deleted parent does. Session reads gain no resolved field: the dialog derives it, and the route's refusal and the overlay already name the level that turned `gh` off.
  - Save is enabled only for a change. A refusal is shown in the dialog with its mapped message, and a success writes the server's view into the cache, as tags do.
- **ADRs.** ADR047 D2's binding bullet, "Who may set a binding", "Where it resolves" and the session refusal text are amended; ADR094 lists `update_agent_session`; the ADR018 row drops the gap. ADR106 and `docs/bex-cli.md` list `bex agent update`.

## Implementation record (t002–t004, 2026-11-23)

- **Backend (t002).** `store.SetAgentSessionGitHubCLI` sets `agent_config.githubCli` with `jsonb_set` or removes the key, in one statement that leaves an archived row unchanged. `Service.UpdateSession` (`agentsessions/githubcli.go`) authorizes `can_operate` on the session, refuses an archived session, reads the stored binding, runs `refuseGitHubCLIBinding(stored, new)`, writes only a change, and logs `agent-session gh binding changed (… from=… to=…)`. REST `PATCH /v1/agent-sessions/{id}` decodes strictly (`SessionConfigUpdate.UnmarshalJSON` records whether `githubCli` was named and refuses any other key), GraphQL `updateAgentSession` takes `agentConfig: AgentSessionConfigUpdateInput` and `resetGithubCli` (graphql-go drops a null, so a null variable keeps the binding), and MCP `update_agent_session` is on `/mcp` only. Scope-matrix and key-permission rows (`agents:manage`), the swept-verb count (503), the MCP parity inventory (289, 261 `Extension`) and both goldens are updated. The session level's refusal and the overlay say what the design record says.
- **CLI (t003).** `bex agent update <ags-id> --github-cli on|off|inherit` sends `PATCH` with the explicit `null` for inherit and prints the session; with no flag it is a usage error. `bex agent get` shows `GitHub CLI inherit` for a repo session that names none. `run`'s help, `docs/bex-cli.md` and ADR106 name the verb.
- **Dashboard (t004).** A **Settings** item in the session header's "…" menu, for repo sessions that are not archived, opens `SessionSettingsDialog` (`components/session-settings-dialog.tsx`) with the field variant of `GithubCliPicker`, the composer's lock (`githubCliLock` with the stored choice) and `inheritedSessionGithubCli` over `githubCliChainOf`. Save sends `setGithubCli` (`resetGithubCli` for inherit), which writes the server's view into the cache. The fragment reads `agentConfig.githubCli`. Six strings in all eight languages. The dashboard's `definitions.ts` and `schema-types.ts` (this feature's hunks only) and `mobile/src/generated-graphql/index.ts` are regenerated from the schema dump. The dialog is a plain `Dialog` opened from a loaded page, so it adds no route skeleton. Its look at 1280 px and 390 px is checked on production in t005: the offline `local-bex` stub no longer serves a session page (its sessions lack a dozen fields the fragment reads, and it has no `agentSessionTree`).

## Render parity (Devin) record (t006, 2026-11-23)

Walked on the code at the working tree of this milestone against [Security profiles](https://docs.devin.ai/product-guides/security-profiles) (re-read 2026-11-23). t005's production run re-checks the dashboard and the refusal text live.

| Surface | bex | Devin | Verdict |
| --- | --- | --- | --- |
| Session settings in the dashboard | **Settings** in the session's "…" menu, GitHub CLI inherit, on or off | "changed later from the session's settings" | match |
| Who changes it | workspace admin (`can_manage`); a member's change is `AGENT_SESSION_GITHUB_CLI_LOCKED`, `admin_required`, in every direction | "Members without the permission can't … change profiles" | match |
| What a member sees | the stored binding and what inherit resolves to, every other choice disabled with "Only a workspace admin can change this." | "anyone can see whether their session is governed by a profile" | match |
| Mandatory floor | `true` under a mandatory off is refused `mandatory` for everyone | "Lower-level selections can only tighten, never loosen" | match |
| When it applies | an off refuses the next `gh` call; an on reaches the next turn | "for existing sessions the next time they resume" | sooner for an off, recorded in ADR047 D2 as the safe direction; same for an on |
| Child sessions | a change on a root reaches its tree; a child's own `true` never turns `gh` on | "child sessions are governed by the same chain as their parent" | match |
| REST `PATCH /v1/agent-sessions/{id}`, GraphQL `updateAgentSession`, MCP `update_agent_session` | the same verb, refusals and fields on all three | v3 pins a profile only at create, no change verb | bex-only API extension of the settings change; consistent across the three |
| CLI `bex agent update --github-cli`, `bex agent get` | change and read the binding | no CLI | bex-only, consistent with the API |
| Session reads (REST, GraphQL, MCP, `bex agent get`) | return the changed binding | the session shows its profile | match |
| `gh` refusal (session level) and overlay | name the setting and how a workspace admin changes it, no "start a new session" | — | consistent with the verb |

No drift left to file: the one gap the design first left (a child's inherit unnamed) was closed in t004 by walking the tree.

## Simplify record (t007, 2026-11-23)

`/simplify` over the backend, CLI and dashboard changes: the GraphQL resolver reads the binding with `gqlutil.BoolPtr` (graphql-go drops a null, so named means non-nil) instead of a hand-rolled presence check. Nothing else needed changing: no existing helper names a binding in a log line, and the dialog reuses `GithubCliPicker`, `githubCliLock` and the tags dialog's shape.

## Test coverage record (t008, 2026-11-23)

| DoD 5 case | Test |
| --- | --- |
| an admin's change in each direction on REST, GraphQL and MCP, read back on all three | `TestSessionGitHubCLIUpdateOnEverySurface` (`agentsessions/githubcli_test.go`) |
| a member refused `admin_required`, inherit included; a member's unchanged save passes | same |
| `mandatory` refusing a `true`; a stored `true` kept by a save that leaves it | same |
| an archived session refused and unchanged; another workspace never reaches it; REST's `resetGithubCli`, both-named `400`, unknown `agentConfig` keys and an empty `PATCH` | same |
| the store write: set, remove, rest of `agent_config` kept, archived unchanged, missing session | `TestPGSetAgentSessionGitHubCLI` (`store`, real Postgres) |
| the next mint after a change, a child under a changed root, a child's own `true` | `TestPGMintFollowsAChangedSessionBinding` (`agentsession`, real Postgres) |
| the next dispatch's driver env after off and inherit, and a child's under its root | `TestDispatchFollowsAChangedSessionBinding` |
| the session level's refusal text | `TestGitHubAPIRouteRefusalNamesTheBindingLevel` (no "start a new session") |
| the overlay line | `github-cli.test.ts` (driver) |
| the CLI change, inherit's `null`, `get`, usage errors and a member's refusal | `TestUpdateChangesTheSessionGitHubCLI`, `TestSessionTextShowsItsGitHubCLIBinding` (`lego/cli`) |
| the dashboard dialog: inherit named, an admin's off and inherit, a member's lock, the mandatory lock, both refusals, an automation's and a child's chain | `session-settings-dialog.test.tsx`, `github-cli.test.ts`, `use-agent-session-mutations.test.ts`, `session-detail-header.test.tsx` |

Reverts, each watched failing and restored: dropping `UpdateSession`'s `refuseGitHubCLIBinding` call fails the surface test at "REST: a member's off"; disabling the mandatory check fails it at "an admin's on under a mandatory off"; letting a child's own `true` count in `GitHubCLIChain.Resolve` fails `TestPGMintFollowsAChangedSessionBinding` at "a child's own on under its root's off". The next-mint test fails by construction if the write does nothing (its first refusal is the off).

## Unblocked (2026-11-24)

- **Source:** verified evidence, `/loopx w10`. The gate was ``pin must carry `cd74ac106e` ``.
- **Finding:** the newest pin on `origin/main` is `eb52a59622` (`chore(deploy): pin platform images to f46c44d9ddb1`), and `git merge-base --is-ancestor` shows `f46c44d9ddb1` carries `cd74ac106e` and `c3a7d4f372`. Production (`hetzner-prod`, read 2026-11-24T02:57Z) runs that pin: both `bex-api` and both `bex-ssh-gateway` Pods on `bex-operator@sha256:3c16403a…`, the dashboard on `bex-dashboard@sha256:a37ac43b…`, and bex-api's `BEX_AGENT_SESSION_IMAGE` is `bex-agent-sandbox@sha256:d0f86b91…`. These match the pin's `deploy/gitops/base/bex.yaml`, `dashboard.yaml` and `lego/operator/config/api/deployment.yaml`, and every Pod's `imageID` matches too.
- **Remaining gates:** none. Next: t005's live check as written, then t009's closeout if the definition of done holds.

## Live check (t005, 2026-11-24)

**Production, `tian-personal`, 03:00–03:12Z, pin `f46c44d9ddb1`** (digests as the Unblocked section reads them). Before the check the workspace had `gh` on, not mandatory, and the automations default inheriting. The check changed none of them, and a read afterwards gave the same. Signed in with `bash scripts/qa-login.sh --serve`. REST, GraphQL and MCP used an API key minted for the check. The CLI was built from `main`, because the released v0.2.1 has no `bex agent update`. One session ran on `claude` with `deepseek/deepseek-v4.1-flash-1017`, $0.073 for four turns.

- **Fixture.** A new private repository, `puncsky/qa-20261124-m599-gh-update` (disposable), with only its README.
- **Session.** `bex agent run --repo puncsky/qa-20261124-m599-gh-update --max-spend 1` started `ags-dc2frk9npn2c73ckvag0` (disposable) with no binding, so `gh` was inherited (on). Turn 1 ran `gh api repos/<fixture> --jq .full_name` seven times, 15 s apart, and every call printed the repository (`exit=0`).
- **Off from the session's settings (DoD 1, 3, 6).**
  - On the session page, "…" → **Settings** opened **Session settings** with "GitHub CLI (gh)" reading **Inherit (on)** and **Save settings** disabled.
  - Turn 2 ran the same loop twelve times. The admin chose **Off** and saved at 03:04:40Z, and the dialog closed with "Settings saved". bex-api logged `agent-session gh binding changed (… from=inherit to=off)`.
  - The loop's call at 03:04:32Z had printed the repository. The next one, at 03:04:48Z, got `HTTP 403` with "bex: gh is turned off for this agent session (its agentConfig.githubCli, or that of the session that started it). A workspace admin turns it back on by setting that session's GitHub CLI to inherit or on in its settings in the dashboard or with `bex agent update <session> --github-cli inherit`. Pushes to your session branch and the bex MCP tools … still work." (`exit=1`). So did every later call in the loop.
  - The gateway logged `agent github api: refused (… status=403 result=github_cli_off)`, and bex-api logged `credential mint denied … gh is turned off for this agent session (the session binding)`.
  - REST `GET /v1/agent-sessions/{id}`, GraphQL `agentSession { agentConfig { githubCli } }` and MCP `get_agent_session` read `githubCli: false`, and `bex agent get` printed `GitHub CLI  off`.
- **A turn dispatched under the off.** Turn 3's sandbox had no `GH_*` variable, and `gh api` asked for `gh auth login` (`exit=4`). The agent quoted its overlay: "There is no GitHub token in the sandbox, and this session's GitHub CLI setting, or that of the session that started it, turned `gh` off, so `gh` and direct GitHub API calls do not work here; do not try to authenticate it."
- **Back to inherit from the CLI.** `bex agent update ags-dc2frk9npn2c73ckvag0 --github-cli inherit` printed `GitHub CLI  inherit` at 03:08:43Z, and bex-api logged `from=off to=inherit`. Afterwards REST and MCP had no `githubCli` key, GraphQL read `null`, and `bex agent get` printed `inherit`. Turn 4's sandbox had `GH_TOKEN`, `GH_REPO` and the other `GH_*` variables back, and `gh api` printed the repository (`exit=0`).
- **The dialog at 1280 px and 390 px.** Read on the same session after inherit, with "Inherit (on)" stored.
  - At 1280 px the dialog is 492 × 267 px, centred. It holds the title, "What applies to this session while it runs. Only a workspace admin changes it.", the picker (444 px), the help line "Off turns gh off for this session and the sessions it starts from their next gh call. On or Inherit gives gh back from the session's next turn." and **Save settings** on the right, disabled until a choice changes.
  - At 390 px it is 358 px wide with 16 px margins. The picker and **Save settings** are full width (308 px), and its list ("Inherit (on)", "On", "Off") opens inside the viewport. Neither width scrolls sideways.
  - The dialog is a plain `Dialog` opened on a loaded page, so there is no skeleton to compare.
- **Not a regression.** Each follow-up turn (2, 3 and 4) logged one bare `credential mint denied … agent credential request forbidden` about 3 s after it was sent, with `gh` off or inherited. This is the sandbox-id mint race at a turn's start, which the driver retries (`gitRemote`, `lego/agent-image/driver/src/git-setup.ts`). Every turn then ran normally.
- **Cleanup.** No branch other than `main` was pushed, and no pull request was opened. The session was canceled and deleted (`404 AGENT_SESSION_NOT_FOUND`), and no sandbox Pod is left. The repository is deleted. The QA API key was revoked, and the QA sign-in was logged out (`ok logged-out`). No OAuth grant or consent session was touched.

**`dev-10`, 03:15–03:19Z: the member's refusal (DoD 2, 6).** Production has no member fixture: the QA account is an admin of all three of its workspaces. bex-api was built from `main` (`f029257800`) by `bash scripts/dev-env.sh 10 up`. Authz was enforced by a dedicated in-memory OpenFGA seeded with `scripts/authz-model.sh` and passed as `DEV_OPENFGA_URL`. OpenSandbox, the model proxy and the gateway were unreachable stand-ins, as in m568, m573 and m592, because `dev-9` holds the agent-session leg. Two throwaway Kratos identities, with emails verified through the admin API, were used. The admin moved their workspace `tea-dc2g3b9jg4r3iomrtl9g` (disposable) to Pro and invited the other as a **Developer**, which OpenFGA recorded as `developer`. Each made one repo-less session, `ags-dc2g421jg4r3iomrtlf0` and `ags-dc2g42pjg4r3iomrtlg0` (both disposable). Both failed at the unreachable sandbox. A failed session still takes a change.

- The admin's `PATCH {"agentConfig":{"githubCli":false}}` on their own session answered `200`.
- **The member on the admin's session (stored off).**
  - Inherit (`null`), `true` and `{"resetGithubCli":true}` each answered `403 AGENT_SESSION_GITHUB_CLI_LOCKED`, `params` `{"field":"agentConfig.githubCli","reason":"admin_required"}`, "only a workspace admin chooses or changes gh for an automation or a session; leave it out to inherit, or unchanged on an update".
  - An unchanged `false` answered `200`.
- **The member on their own session (inherit).** `false` and `true` got the same refusal. An unchanged `null` answered `200`.
- **The other surfaces.** As the member, GraphQL `updateAgentSession` (with `resetGithubCli`, and with `agentConfig: {githubCli: false}`) answered with `extensions` `{code: AGENT_SESSION_GITHUB_CLI_LOCKED, field: agentConfig.githubCli, reason: admin_required}`. MCP `update_agent_session` returned `isError: true` with the same code and text. `bex agent update … --github-cli off` and `--github-cli inherit` printed `403 AGENT_SESSION_GITHUB_CLI_LOCKED: …` and exited 1.
- **Nothing moved.** After the refusals the admin's session still read off and the member's inherit. The admin's `null` then returned theirs to inherit.
- **An archived session.** It answered the admin's change with `409 AGENT_SESSION_ARCHIVED`.
- **Torn down.** Both sessions are deleted (`404`). Both keys were revoked, both users logged out and both identities deleted. `bash scripts/dev-env.sh 10 down` ran, and the OpenFGA container was removed.

## Closeout (t009, 2026-11-24)

The definition of done holds. Items 1–5 hold in t002–t004's code, t006's surface walk and t008's tests and reverts. Item 6 held live above: production's off refused the very next `gh` call naming the session level, and the turn after the return to inherit had `gh`. A member's change was refused on `dev-10` on all four surfaces, and every session the check created is deleted. Items 1, 3 and 4 were also seen live: the reads on every surface, the next mint and next dispatch, the refusal text and the overlay line.

## Source + Goal linkage

- **Source:** inbox note [w10/050](../../done/050.md), filed by w10/044's Devin check on 2026-11-23 and promoted by `/loopx w10` on 2026-11-23.
- **Triage evidence (2026-11-23, `cf088e38f6`):**
  - **No verb takes it after create.** `refuseGitHubCLIBinding` (`lego/backend/internal/agentsessions/githubcli.go:80`) guards a session only in `Create` (`service.go:699`, `stored` nil). The store writes `agent_config` after create only for `model` (`lego/backend/internal/store/agentsessions.go:696`). The `/v1/agent-sessions/{id}` routes are `cancel`, `tags`, `archive`, `unarchive`, `DELETE`, `steer`, `interrupt`, `attach-ticket` and `queued-messages`: there is no `PATCH` (`rest.go`, `queue_rest.go`). GraphQL's `githubCli` input exists only on `AgentSessionConfigInput` (`graphql.go:37`). MCP's only session update is `set_agent_session_tags`.
  - **The surfaces say so.** The session level's `gh` refusal reads "A session keeps that choice: start a new session without `--github-cli off` to have gh" (`sshgateway/agentcred/githubapi.go:85-86`). The overlay says "`gh` is turned off for this session, which was started with it off" (`lego/agent-image/driver/src/overlay.ts:185`). ADR047 D2 says "set at create only" and links this note.
  - **No settings surface yet.** The session header's menu (`dashboard/src/features/agent-sessions/components/session-detail-header.tsx:393-430`) offers tags, archive and delete; `GithubCliPicker` (`github-cli-picker.tsx`) and `githubCliLock` serve only the composer and the automation form. The CLI's only session-edit verb is `bex agent tags` (`lego/cli/internal/agent/lifecycle.go:100`).
  - **The resolver needs nothing new.** `agentsession.ResolveGitHubCLI` and `LoadGitHubCLIChain` (`lego/backend/internal/agentsession/githubcli.go:116`, `:145`) read the session's `agent_config` live, with no cache, at every mint and dispatch. A stored change is what the next mint and dispatch see.
- **Devin (re-read 2026-11-23, [Security profiles](https://docs.devin.ai/product-guides/security-profiles)):** a session's profile is "chosen for an individual session when it's created" and "changed later from the session's settings". "A session resolves its governing profile at the moments it boots": edits "do not automatically propagate to sessions that are already running", and changes take effect "for existing sessions the next time they resume". "Members without the permission can't enumerate, select, or change profiles." The v3 API pins a profile to a session only "when you … create a session": the page documents no API to change one later.
- **Goal linkage:** cloud coding-agent parity with Devin ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)), the security-profile chain w10/m561 built and w10/044 gated to admins.
- **Expected outcome:** an admin fixes a session's `gh` in place, from the session or any API surface, instead of losing the session's context to a new one.
- **Why now:** the chain and its admin gate shipped this week (w10/m561, w10/044), and the session level is the one Devin binding bex cannot change after it is set. The refusal text sends users to start over.
- **Render parity task:** included, against Devin, because the milestone adds REST, GraphQL, MCP, CLI and dashboard surface.
