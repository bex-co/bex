# w10 · m538 — An agent session marks its own pull request ready and merges it when its task asks, as Devin does

**Worker:** worker10 **Goal:** a task such as "fix the flaky test and merge once CI is green" ends with the session's pull request merged, not with a draft a person has to undraft and merge. Two `bex` MCP tools mark the session's own recorded pull request ready and merge it, using the merge method the repository allows. Each call goes through the Pod-bound gateway hop and a bex-api verb, with no token in the sandbox. Branch protection still decides whether a merge is allowed, and an App's approval stays refused, so a session cannot satisfy required reviews by itself. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: the tools' shape, merge method and head pin, the token, GitHub's refusals, a taken-over PR; ADR090 and ADR047 amended | 1h | — |
| [t002](t002.md) | Backend: GitHub client and bex-api verbs that mark the session's recorded PR ready and merge it, recording `prState: merged` at once | 2h | t001 |
| [t003](t003.md) | Gateway: the `pull-request/ready` and `pull-request/merge` routes on the session's Git proxy path | 1h | t002 |
| [t004](t004.md) | Driver: the `bex` MCP tools, the overlay's "only when asked" rule, and the `gh pr ready`/`gh pr merge` refusal naming them | 1h15m | t001, t003 |
| [t005](t005.md) | Live check on production: a session told to merge once CI is green merges its own PR, and GitHub's refusals reach the agent | 45m | t003, t004 |
| [t006](t006.md) | Render parity (Devin) | 30m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 7h45m**.

## Definition of done

1. **The tools.** In a repo session, the `bex` MCP server lists a ready tool and a merge tool (t001 names them, for example `mark_pull_request_ready` and `merge_pull_request`). Ready undrafts the session's recorded pull request (`prNumber`). Merge merges it with a method the repository allows: the agent may name one, and otherwise the repository's first allowed method is used. The merge is pinned to the head bex recorded for the session, so a commit pushed later is never merged unchecked. Neither tool takes a pull request number, owner or repository: all of them come from the session record.
2. **GitHub decides, and says why.** With no pull request yet the tool answers `no_pull_request`, and with a closed or merged one `pull_request_closed`. GitHub's own refusal reaches the agent as GitHub states it, under a typed code with a hint that says what to wait for or fix, and the hint tells the agent not to retry in a loop. That covers required checks pending or failing, required reviews, a conflict, a head that moved and a method the repository disallows. An `APPROVE` review stays refused on every path (ADR090 D4), so a required review is still a person's.
3. **The state shows at once.** A merge records `prState: merged` on the session immediately, so REST, GraphQL, MCP and the dashboard read it without waiting for the PR-state poller or a webhook.
4. **Accountable.** Each call is audited, allowed or denied, with the repository. It runs through the PR verb's chain: Pod source-IP resolution, the live-generation check, the limiter and the budgets. Its token is minted per call, narrowed to the one repository with the permissions t001 records (`pull_requests` and `contents` write, plus `workflows` when the PR changes workflow files, falling back as w10/014 does), and never enters the sandbox.
5. **Said everywhere.** `gh pr ready` and `gh pr merge` stay refused in the `github-api` route, and the refusal now names the new tools. The overlay drops "never mark it ready". In its place, it tells the agent to mark the pull request ready or merge it only when the task or a person's message asks, and otherwise to leave the draft for the requester. ADR090 D2, D3 and D4, ADR047 and `lego/agent-image/README.md` say the same.
6. **Live.** Once a deploy carries the milestone, run the check on production in `tian-personal`, on a QA repository whose default branch requires one check. Give a session the task "open the PR and merge it once CI is green". It undrafts and merges its own pull request after the check passes, and REST, GraphQL and MCP read `prState: merged`. A merge tried while the check is pending is refused with GitHub's reason, and `gh pr merge` is refused naming the tool. Afterwards, delete the session and the repository.

## Source + Goal linkage

- **Source:** inbox note [w10/034](../done/034.md), filed by w10/m522's Devin comparison (t006) on 2026-11-19 and promoted by `/loopx w10` on 2026-11-20. w10/033 (`80b275f87`) already edited ADR090 D4's refused list and the overlay's `gh` sentence, so this milestone starts from that text.
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
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../docs/ADR090-agent-owned-delivery.md)). It continues [w10/m522](../done/m522/README.md) and [w10/033](../done/033.md), which gave `gh` every other write Devin's App makes.
- **Expected outcome:** an automation such as `beancount-rt-flaky-test-fixer` can be told to merge once its fix is green and finish the job unattended. A person who asks a session to merge sees it merged, or sees GitHub's reason it could not be.
- **Why now:** since w10/m522 and w10/033, opening the pull request, marking it ready and merging it are the only parts of Devin's PR workflow a session cannot do. Every other GitHub write is already admitted, and the PR state the merge records already shows on every surface.
- **Render parity included (reference product: Devin):** the `bex` MCP tools and the session's `prState` are cloud coding-agent surfaces. t006 walks the tools and their refusals against Devin's behavior above. It also checks two adjacent differences and files each as its own item if it matters: Devin opens a regular pull request by default (6 of 200 Devin-authored pull requests created since 2026-11-13 were drafts on 2026-11-20), while ADR090 D3 always opens a draft; and GitHub auto-merge (`gh pr merge --auto`).

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
