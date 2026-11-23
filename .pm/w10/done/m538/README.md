# w10 · m538 — An agent session marks its own pull request ready and merges it when its task asks, as Devin does

**Worker:** worker10 **Goal:** a task such as "fix the flaky test and merge once CI is green" ends with the session's pull request merged, not with a draft a person has to undraft and merge. Two `bex` MCP tools mark the session's own recorded pull request ready and merge it, using the merge method the repository allows. Each call goes through the Pod-bound gateway hop and a bex-api verb, with no token in the sandbox. Branch protection still decides whether a merge is allowed, and an App's approval stays refused, so a session cannot satisfy required reviews by itself. **Status:** done (2026-11-22)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: the tools' shape, merge method and head pin, the token, GitHub's refusals, a taken-over PR; ADR090 and ADR047 amended — **DONE** | 1h | — |
| [t002](done/t002.md) | Backend: GitHub client and bex-api verbs that mark the session's recorded PR ready and merge it, recording `prState: merged` at once — **DONE** | 2h | t001 |
| [t003](done/t003.md) | Gateway: the `pull-request/ready` and `pull-request/merge` routes on the session's Git proxy path — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Driver: the `bex` MCP tools, the overlay's "only when asked" rule, and the `gh pr ready`/`gh pr merge` refusal naming them — **DONE** | 1h15m | t001, t003 |
| [t005](done/t005.md) | Live check on production: a session told to merge once CI is green merges its own PR, and GitHub's refusals reach the agent — **DONE** | 45m | t003, t004 |
| [t006](done/t006.md) | Render parity (Devin) — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006, t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

Total: **9 tasks, about 7h45m**.

## Definition of done

1. **The tools.** In a repo session, the `bex` MCP server lists a ready tool and a merge tool (t001 names them, for example `mark_pull_request_ready` and `merge_pull_request`). Ready undrafts the session's recorded pull request (`prNumber`). Merge merges it with a method the repository allows: the agent may name one, and otherwise the repository's first allowed method is used. The merge is pinned to the head bex recorded for the session, so a commit pushed later is never merged unchecked. Neither tool takes a pull request number, owner or repository: all of them come from the session record.
2. **GitHub decides, and says why.** With no pull request yet the tool answers `no_pull_request`, and with a closed or merged one `pull_request_closed`. GitHub's own refusal reaches the agent as GitHub states it, under a typed code with a hint that says what to wait for or fix, and the hint tells the agent not to retry in a loop. That covers required checks pending or failing, required reviews, a conflict, a head that moved and a method the repository disallows. An `APPROVE` review stays refused on every path (ADR090 D4), so a required review is still a person's.
3. **The state shows at once.** A merge records `prState: merged` on the session immediately, so REST, GraphQL, MCP and the dashboard read it without waiting for the PR-state poller or a webhook.
4. **Accountable.** Each call is audited, allowed or denied, with the repository. It runs through the PR verb's chain: Pod source-IP resolution, the live-generation check, the limiter and the budgets. Its token is minted per call, narrowed to the one repository with the permissions t001 records (`pull_requests` and `contents` write, plus `workflows` when the PR changes workflow files, falling back as w10/014 does), and never enters the sandbox.
5. **Said everywhere.** `gh pr ready` and `gh pr merge` stay refused in the `github-api` route, and the refusal now names the new tools. The overlay drops "never mark it ready". In its place, it tells the agent to mark the pull request ready or merge it only when the task or a person's message asks, and otherwise to leave the draft for the requester. ADR090 D2, D3 and D4, ADR047 and `lego/agent-image/README.md` say the same.
6. **Live.** Once a deploy carries the milestone, run the check on production in `tian-personal`, on a QA repository whose default branch requires one check. Give a session the task "open the PR and merge it once CI is green". It undrafts and merges its own pull request after the check passes, and REST, GraphQL and MCP read `prState: merged`. A merge tried while the check is pending is refused with GitHub's reason, and `gh pr merge` is refused naming the tool. Afterwards, delete the session and the repository.

## Source + Goal linkage

- **Source:** inbox note [w10/034](../034.md), filed by w10/m522's Devin comparison (t006) on 2026-11-19 and promoted by `/loopx w10` on 2026-11-20. w10/033 (`80b275f87`) already edited ADR090 D4's refused list and the overlay's `gh` sentence, so this milestone starts from that text.
- **Triage evidence (2026-11-20, `80b275f87`):**
  - **Nothing undrafts or merges.** The D3 verb only opens a draft or updates its title and body (`lego/backend/internal/github/pulls.go:79`, `SyncDraftPullRequest`, token `pull_requests` write and `contents` read). ADR090 D3 says "It never undrafts" (`docs/ADR090-agent-owned-delivery.md:86`), and D2's overlay rule is "never mark it ready" (`:63`). The overlay tells the agent "Leave the pull request as a draft. Marking it ready for review is the requester's decision." (`lego/agent-image/driver/src/overlay.ts:198`).
  - **`gh` refuses both.** The `github-api` route refuses `markPullRequestReadyForReview`, `mergePullRequest` and the auto-merge mutations (ADR090 D4, `:111`). Its write token has no `contents` write, and merging needs it, so admitting `gh pr merge` there would also mean widening that token.
  - **The merged state is already on every surface.** `prState` and `prStateAt` are on REST, GraphQL and MCP, and REST and GraphQL take a `prState` filter (`lego/backend/internal/agentsessions/graphql.go:413`, `mcp.go:77`, `pr_state.go`). The dashboard shows it (`session-pr-state-filter.tsx`). `store.RecordAgentSessionPRState` (`lego/backend/internal/store/agentsession_pr_state.go:75`) can record a merge immediately.
  - **The template exists.** `reply_on_pull_request` acts on the session's own recorded pull request through a kind-tagged HMAC verb. It is built from `lego/backend/internal/agentsession/pullrequest_comment.go`, the gateway's `servePullRequestComment` (`lego/backend/internal/sshgateway/agentcred/pullrequest.go:67`), the wiring in `cmd/api/main.go:1385` and `cmd/ssh-gateway/main.go:372`, and `lego/agent-image/driver/src/reply-on-pr.ts`. The new verbs follow it.
- **Reference product (Devin), verified 2026-11-20:**
  - **Docs.** Devin's App holds pull requests write ("Allow Devin to create new PRs") and contents write. Its setup tip recommends branch protection "to ensure all required checks pass before Devin can merge changes" ([GitHub integration](https://docs.devin.ai/integrations/gh)). Its SDLC guide says Devin "is subject to the exact same branch protections and SDLC policies as any human engineer" ([SDLC integration](https://docs.devin.ai/essential-guidelines/sdlc-integration)).
  - **Observed: Devin merges its own pull requests.** In a sample of the 60 most recently updated merged pull requests authored by `app/devin-ai-integration`, 35 were merged by `devin-ai-integration[bot]` itself, for example [gateweavehq/track#365](https://github.com/gateweavehq/track/pull/365) and [carlosmazzei/Kurn#447](https://github.com/carlosmazzei/Kurn/pull/447).
  - **Observed: Devin undrafts before merging.** On [TeddyJubu/cal-scheduling#1](https://github.com/TeddyJubu/cal-scheduling/pull/1), a Devin draft, the timeline shows `ready_for_review` by `devin-ai-integration[bot]` at 22:33:11Z and `merged` by the same bot at 22:33:18Z on 2026-11-20.
  - **Merging waits to be asked.** A Devin-authored pull request that Devin merged itself, [Al-does/alex-rl-experiments#149](https://github.com/Al-does/alex-rl-experiments/pull/149), says a session otherwise returns "the PR link for manual merging". It says the session's "GitHub credentials already have merge permission (verified against the GitHub merge API)", and that agents "merge only on explicit request — nothing here makes merging autonomous".
  - **The parity-sufficient behavior** is therefore: the session marks its pull request ready and merges it when asked, and branch protection decides. That is the note's "allowed" branch, so no ADR018 divergence or user sign-off is needed.
- **Open question in the note, settled under the Devin rule:** the note asked whether a session may undraft and merge "only the pull request it opened, … only when its task or a person's message asked for it, or never". Devin allows it, so the answer is not "never". "Only when asked" is how Devin behaves, and it lives in the overlay as an instruction, not as a mechanism, because no gate can read intent.
  - **One hardening Devin does not document.** Devin's App credentials can merge any pull request in the repository. bex keeps the session confined to its own recorded pull request, as its Git proxy confines pushes to the session branch, and ADR090 records why. A user who wants the stricter variant (agents never merge) already has the workspace's `gh` switch for `gh`. A per-workspace switch for these tools is a variant t006 may file; it does not gate this milestone.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../../docs/ADR090-agent-owned-delivery.md)). It continues [w10/m522](../m522/README.md) and [w10/033](../033.md), which gave `gh` every other write Devin's App makes.
- **Expected outcome:** an automation such as `beancount-rt-flaky-test-fixer` can be told to merge once its fix is green and finish the job unattended. A person who asks a session to merge sees it merged, or sees GitHub's reason it could not be.
- **Why now:** since w10/m522 and w10/033, opening the pull request, marking it ready and merging it are the only parts of Devin's PR workflow a session cannot do. Every other GitHub write is already admitted, and the PR state the merge records already shows on every surface.
- **Render parity included (reference product: Devin):** the `bex` MCP tools and the session's `prState` are cloud coding-agent surfaces. t006 walks the tools and their refusals against Devin's behavior above. It also checks two adjacent differences and files each as its own item if it matters: Devin opens a regular pull request by default (6 of 200 Devin-authored pull requests created since 2026-11-13 were drafts on 2026-11-20), while ADR090 D3 always opens a draft; and GitHub auto-merge (`gh pr merge --auto`).

## Design record (t001, 2026-11-20)

Settled on `e6191e804` and implemented in the same change; [ADR090 D3](../../../../docs/ADR090-agent-owned-delivery.md#d3--the-pull-request-verb) (the w10/m538 amendment) is the contract.

- **Tools and paths.** `mark_pull_request_ready({})` and `merge_pull_request({method?, commitTitle?, commitMessage?})` in the `bex` MCP server, listed only when the driver has `BEX_PR_READY_ENDPOINT` / `BEX_PR_MERGE_ENDPOINT` (repo sessions: `<Git proxy URL>/pull-request/ready` and `…/pull-request/merge`). The gateway serves both through `admitPullRequestVerb` (POST, JSON, no query, Pod source IP, label digests, limiter, Git budget, 64 KiB body) and strict-decodes the body: ready takes `{}`, merge at most the three fields. bex-api's `:8091` verbs are `/v1/agent-session-pull-request-ready` and `/v1/agent-session-pull-request-merges`, kinds `pull-request-ready` and `pull-request-merge` (each its own MAC domain, nonce required), audit verbs `agentsessions.PullRequestReady` and `agentsessions.PullRequestMerge`. The gateway URLs override with `BEX_AGENT_PULL_REQUEST_READY_API_URL` and `BEX_AGENT_PULL_REQUEST_MERGE_API_URL`.
- **Undrafting is GraphQL-only.** GitHub's REST reference lists only `title`, `body`, `state`, `base` and `maintainer_can_modify` for `PATCH …/pulls/{n}` ([Pulls](https://docs.github.com/en/rest/pulls/pulls), read 2026-11-20), and the recorded gh 2.105.0 `gh pr ready` sends `markPullRequestReadyForReview(input: {pullRequestId})` (`testdata/gh-2.105.0-requests.json`). bex-api reads the node id from `GET …/pulls/{n}` (`node_id`).
- **Merge call: the synchronous REST merge.** `PUT …/pulls/{n}/merge` answers 200 merged, 405 not mergeable, 409 when `sha` does not match the head, 422 invalid ([Pulls](https://docs.github.com/en/rest/pulls/pulls), read 2026-11-20). GitHub now recommends `PUT …/pulls/{n}/merge-async`, which answers 202 with a UUID to poll (`GET …/merge-async/{uuid}`, kept 24 h) and is required for merge queues and stacked pull requests. The sync call answers within the tool call, so the agent hears GitHub's reason at once; a repository that requires a merge queue gets GitHub's refusal, and the queue and auto-merge are filed as [w10/039](../039.md).
- **Head pin.** `sha` = the session's recorded `headSha`. bex-api refuses `head_changed` before calling GitHub when the pull request's head differs from it (or nothing is recorded), and maps GitHub's 409 to the same code.
- **Method.** Named, or the first of merge, squash, rebase allowed by the repository settings (`allow_merge_commit`, `allow_squash_merge`, `allow_rebase_merge`; "To view merge-related settings, you must have the contents:read and contents:write permissions", [Repositories](https://docs.github.com/en/rest/repos/repos#get-a-repository), read 2026-11-20) and by the base branch's active rules (`GET …/rules/branches/{branch}`: `pull_request.parameters.allowed_merge_methods`, and `required_linear_history` drops merge commits; [Rules](https://docs.github.com/en/rest/repos/rules#get-rules-for-a-branch), read 2026-11-20). Unreadable rules leave the choice to the settings; GitHub then reports a rule's refusal.
- **Token permissions.** Pull requests read to read the PR back; Contents and Pull requests write to undraft (GitHub's reference names none for the mutation, and third-party reports show installations with Pull requests write alone refused it with "Resource not accessible by integration" until Contents write was added); Contents, Pull requests and Workflows write to merge (GitHub lists Contents write for both merge endpoints, [permissions for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps), read 2026-11-20; Workflows write for a pull request that changes `.github/workflows/`, falling back to the set without it on 422 as w10/014's push token does). A mint the installation cannot grant is `not_permitted`. Production's App already holds Contents and Pull requests write (the session push token and the D3 verb use them).
- **Refusal table.**

  | Code | Status | When | Hint |
  | --- | --- | --- | --- |
  | `no_pull_request` | 409 | no `prNumber` yet | push and call `open_or_update_draft_pr` first |
  | `pull_request_closed` | 409 | closed, or merged already (recorded merged) | nothing to do; tell the user |
  | `pull_request_draft` | 409 | merge of a draft | call `mark_pull_request_ready` first |
  | `head_changed` | 409 | recorded head ≠ PR head, or GitHub 409 | push, call `open_or_update_draft_pr`, wait for checks, try once |
  | `merge_blocked` | 409 | GitHub 405 (checks pending or failing, required review, conflict, behind, a rule), or a 422 not about the method | per cause: `gh pr checks --watch`; fix and push; stop and tell the user (you cannot approve); merge the base; never loop |
  | `merge_method_not_allowed` | 422 | named method the settings or rules refuse, or GitHub's "… are not allowed" words | retry with a listed method or none |
  | `not_permitted` | 403 | GitHub 403, or a token the installation cannot mint | do not retry; a person has to do it |
  | `rate_limited`, `upstream_error`, `forbidden`, `invalid_request`, `too_large`, `unavailable` | as the PR verb | | as the PR verb |

  Every message carries GitHub's own words (`message` plus `errors[].message`, one line, ≤1000 characters, prefixed `GitHub: `), and every hint ends naming the two tools as the only route.

- **Takeover decision.** A takeover session may mark ready or merge the person's pull request it took over, when the `/bex` comment or a later message asks, at the head its fast-forward recorded. The commenter has write access and could merge it themselves, and Devin's `/devin` session on a pull request acts on that pull request with the same App credentials. The overlay says the pull request's state belongs to the person who opened it unless asked.
- **The state.** A merge calls `RecordAgentSessionPRState(merged, next: nil)` in the request; a failed write is logged and the poller records it later.
- **Delivery check.** The merge tool marks the driver's `pull-request.json` merged at the merged head, so a branch GitHub deletes after the merge does not make the post-turn check call the merged commits unpushed.

## Render parity (Devin) record (t006, 2026-11-20)

Walked against Devin's docs ([GitHub integration](https://docs.devin.ai/integrations/gh), [SDLC integration](https://docs.devin.ai/essential-guidelines/sdlc-integration), [Devin Review](https://docs.devin.ai/work-with-devin/devin-review), all read 2026-11-20) and the observed pull requests in § Reference product. The live run (t005) can still add findings.

| Behavior | Devin | bex (w10/m538) | Verdict |
| --- | --- | --- | --- |
| Undraft its pull request | `ready_for_review` by `devin-ai-integration[bot]` before the merge | `mark_pull_request_ready`, as the App's bot | match |
| Merge its pull request when asked | merged by `devin-ai-integration[bot]`; "merge only on explicit request" | `merge_pull_request`; overlay and descriptions say only when asked | match |
| Branch protection decides | "subject to the exact same branch protections … as any human engineer" | GitHub decides; `bypass_rules` never sent; `APPROVE` refused on every path | match |
| Why a merge was refused | GitHub's answer to its `gh` / App call | GitHub's message under a typed code with a what-to-do hint | match (bex adds the hint) |
| Merge method | the repository's settings | named, or the first the settings and the base branch's rules allow | match |
| Merged state on the session | `pr_state` | `prState: merged` at once on REST, GraphQL, MCP, dashboard | match |
| GitHub auto-merge, merge queue | Devin Review toggles auto-merge and the queue; `gh pr merge --auto` | refused naming the tool; sync merge only | gap → [w10/039](../039.md) |
| Draft by default | 6 of 200 recent Devin pull requests were drafts | always a draft (ADR090 D3) | gap → [w10/040](../040.md) |
| Act on another pull request of the repository | its App credentials can | the session's recorded pull request only | divergence without sign-off → [w10/041](../../041.md) |
| A switch that stops agents merging | none documented; branch protection is the control | none (the `gh` switch does not reach these tools) | not filed: no Devin counterpart, and branch protection already gates every merge |

The ADR018 bex-native agent rows gained this milestone's row with the three open gaps.

## Test coverage record (t008, 2026-11-20)

Tests: `lego/backend/internal/github/merge_test.go` (token scopes and the Workflows fallback, the method rule from settings and rules, the pinned `sha`, GitHub's message extraction, the GraphQL undraft and its errors), `lego/backend/internal/agentsession/pullrequest_merge_test.go` (both verbs' success, every refusal code, another session's or a stale Pod's call, a closed, merged, draft or foreign pull request, a takeover, a failed state record, the signed hop's MAC domains, replay and a numbered envelope), `pullrequest_pg_test.go` (the merged state on the real row and the `prState` filter), `lego/backend/internal/sshgateway/agentcred/pullrequest_merge_test.go` (Pod binding, body validation, relayed refusals, the shared budget), `githubapi_test.go` (`gh pr ready`/`gh pr merge` replayed from the gh 2.105.0 fixture are refused naming the tools), and the driver's `test/merge-pr.test.ts`, `overlay.test.ts`, `github-cli.test.ts` and `delivery-check.test.ts`.

| Mutant | Killed by |
| --- | --- |
| Drop the head pre-check (`session.HeadSHA` vs the PR's head) | `TestPullRequestMergeTypedRefusals/head_moved_past_the_record` |
| Drop `SHA` from the merge options | `TestPullRequestMergeMergesAtTheRecordedHeadAndRecordsMerged`, `TestPullRequestMergeOfATakenOverPR` |
| Drop `sha` from GitHub's merge body | `TestMergePullRequestPinsTheHeadWithTheFirstAllowedMethod` |
| Accept unknown fields (a PR number) on the signed hop | `TestPullRequestReadyAndMergeHandlersAndClientsRoundTrip` |
| Accept unknown fields (a PR number) at the gateway | `TestPullRequestReadyAndMergeRoutesValidateTheBody/merge_names_a_PR` |
| Skip the merged-state record | `TestPullRequestMergeMergesAtTheRecordedHeadAndRecordsMerged` (and `TestPGPullRequestVerbRecordsThePRMidTurn`) |
| List `merge_pull_request` without its endpoint (a repo-less session) | `only repo sessions list the ready and merge tools, saying to use them only when asked` |
| Drop "only when … asks" from the tool description | the same driver test |
| Replace the overlay's "Leave the pull request as a draft … unless … asks" | `the repo overlay names the branch, base, tool and authorization` |
| Ignore the merged head in the delivery check | `work merged by merge_pull_request counts as delivered after GitHub deletes the branch` |

## Live check (t005, production, 2026-11-22)

Workspace `tian-personal` (`tea-da2isimlm39c739m4ofg`), 20:06–20:17Z, on pin `eaaa61506165`, which carries `9163cfe3e`. The workspace's GitHub connection is the `puncsky` installation of the `bex-co` App.

- **Fixture.** A new public repository, `puncsky/qa-20261122-m538-merge`, whose `main` required the status check `test` (classic branch protection, no reviews), from a workflow `ci` whose one job sleeps 150 s, on `pull_request` and pushes to `main`. Settings allowed merge, squash and rebase.
- **Session.** `bex agent run --repo puncsky/qa-20261122-m538-merge --max-spend 1` (the CLI built from `main`, `claude` on `deepseek/deepseek-v4.1-flash-1017`, template `agent`) started `ags-dc1kmbu0usec73d5it3g` with the task "open the pull request, mark it ready, try the merge while `test` runs, run `gh pr merge --squash`, wait for the checks, then merge".
  - `open_or_update_draft_pr` opened draft #1 at 20:06:30Z with head `5fae03cc0816`; `mark_pull_request_ready` returned `{url, number: 1}`, and GitHub's timeline shows `ready_for_review` by `bex-co[bot]` at 20:06:33Z.
  - `merge_pull_request` at 20:06:38Z, with `test` running (20:06:36–20:09:09Z), was refused `merge_blocked` with "GitHub: Required status check \"test\" is in progress." and the hint naming `gh pr checks --watch`.
  - After `test` passed, `merge_pull_request` at 20:09:16Z returned `{merged: true, sha: ff71713bb9d0, method: merge, headSha: 5fae03cc0816}`: the repository's first allowed method, pinned to the recorded head. GitHub shows #1 `merged` by `bex-co[bot]` at 20:09:19Z, merge commit `ff71713bb9d0` with two parents on `main`.
- **The state at once.** `prStateAt` reads 20:09:20.164Z, 36 ms before the tool's answer reached the sandbox and within the turn (the turn ended 20:09:25Z), so the verb recorded it, not the poller. REST `GET /v1/agent-sessions/<id>` and the `prState=merged` list filter, GraphQL `agentSession { prState prStateAt }` and MCP `get_agent_session` all read `prState: merged` (read with a QA key minted for the workspace, revoked afterwards).
- **Audited.** `audit_events` (read-only, the `bex-db-1` replica) has `agentsessions.PullRequestReady` allowed at 20:06:33.86Z, `agentsessions.PullRequestMerge` denied at 20:06:38.58Z and allowed at 20:09:20.17Z, each on both hops (gateway and bex-api), with target `agent-session:ags-dc1kmbu0usec73d5it3g` and target name `puncsky/qa-20261122-m538-merge`.
- **`gh` stays refused.** A second turn opened draft #2 and ran `gh pr ready 2`, which the `github-api` route refused `403`: "bex: gh pr ready and gh pr merge do not work in this session; call the mark_pull_request_ready and merge_pull_request tools of the bex MCP server instead, only when your task or a person asks. gh in this session reads …". `gh pr merge 2 --squash` stopped at gh's own check ("the base branch policy prohibits the merge") before it sent a mutation, as `gh pr create` did in w10/m522's run, and the agent declined a third turn's `gh pr merge 2 --squash --admin` as a bypass of the branch policy. The route's refusal of that mutation is the one `TestGitHubAPIRouteReplaysTheRecordedGhCommands` asserts on the recorded gh 2.105.0 `gh pr merge --squash` and `--admin` requests.
- **Found.** Without a number, `gh pr merge --squash` and `gh pr checks --watch` failed with "could not resolve remote \"origin\"": gh maps the current branch's pull request through the git remote, which is the Git proxy. The agent recovered with `--repo`; the overlay and the `merge_blocked` hint should name the number. Filed as [w10/042](../../042.md).
- **Cleanup.** #2 closed, the session deleted (`404 AGENT_SESSION_NOT_FOUND`), the repository deleted (`404`), the QA key revoked and the QA sign-in logged out.

The Definition of done holds: items 1–5 in the shipped code, tests and ADR amendments (`9163cfe3e`), item 6 here. t009 closed the milestone on 2026-11-22.

## Direction for t001

- **Shape.** Add two tools, or one tool with an action, beside `open_or_update_draft_pr`. Each is a kind-tagged signed verb on bex-api's `:8091`, reached at `<Git proxy URL>/pull-request/ready` and `…/pull-request/merge`, as the comment verb is. The merge tool takes an optional `method` (`merge`, `squash` or `rebase`) and optional commit title and message, and nothing that names a pull request. With no `method`, it uses the repository's first allowed method, read from GitHub.
- **Head pin.** Send `sha` as the session's recorded `headSha`. The agent pushes and calls `open_or_update_draft_pr` first, which records the head. GitHub's 409 for a moved head becomes a typed refusal that says to call `open_or_update_draft_pr` again.
- **Verify, don't assume:**
  - Which installation permissions a merge needs (`contents` write, and `workflows` when the PR changes `.github/workflows/`).
  - Whether GitHub's REST merge endpoint or the asynchronous merge API it now recommends (which supports merge queues) is the right call.
  - That undrafting is GraphQL-only (`markPullRequestReadyForReview` with the PR's node id).
  - Record each answer in ADR090.
- **A taken-over PR (w9/m348).** Its recorded `prNumber` is the person's pull request, whose draft state ADR090 D3 leaves alone. t001 decides whether a takeover session may mark it ready or merge it when the person who started it asks, as Devin's `/devin` on a PR would, and records the reason.
</content>
</invoke>
