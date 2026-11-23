# w10 · m577 — A session told to merge once CI is green arms GitHub auto-merge or the merge queue and ends its turn, as Devin can

**Worker:** worker10 **Goal:** "merge once CI is green" no longer keeps a session in its turn watching the checks, and a repository whose base branch requires a merge queue no longer refuses the session's merge outright. `merge_pull_request` takes one option (t001 names it, for example `auto: true`, after `gh pr merge --auto`). With it, bex enables GitHub auto-merge on the session's own recorded pull request, pinned to the head bex recorded, and on a merge-queue branch the pull request goes into the queue. GitHub then lands it when its checks pass, and the session's `prState` reads `merged` once the PR-state poller or a `pull_request` delivery sees it. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: the option, auto-merge vs the queue vs a merge now, the head pin after arming, the undo, the refusals and the answer; ADR090 and ADR047 amended | 1h | — |
| [t002](t002.md) | Backend: the GitHub client enables and disables auto-merge and enqueues, pinned to the head | 1h15m | t001 |
| [t003](t003.md) | Backend: the merge verb and the gateway route take the option and answer an armed or queued pull request | 1h15m | t002 |
| [t004](t004.md) | Driver: the tool's option, its answer, the overlay's "arm it and end your turn" and the `gh pr merge --auto` refusal | 1h | t001, t003 |
| [t005](t005.md) | Live check on production: a session arms auto-merge and ends its turn, GitHub merges after the check, and `prState` reads merged | 45m | t003, t004 |
| [t006](t006.md) | Render parity (Devin) | 30m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 7h**.

## Definition of done

1. **Arm it and go.** In a repo session, `merge_pull_request` with the option on enables GitHub auto-merge on the session's recorded pull request (`enablePullRequestAutoMerge` with `expectedHeadOid` = the recorded `headSha`, the method rule of today's merge, and the commit text when given). It answers within the call that auto-merge is armed (for example `{merged: false, autoMerge: true, method, headSha}`), and the call returns without waiting for the checks. The tool still takes no pull request number, owner, repository or head.
2. **The merge queue.** Where the base branch requires a merge queue (GitHub's `isMergeQueueEnabled`, which `gh pr merge` reads), the pull request goes into the queue, pinned to the recorded head, and the answer says so (for example `{merged: false, enqueued: true}`). That holds both with the option and without it, as Devin's `m` and GitHub's merge button add it to the queue. The call no longer comes back as `merge_blocked`. A pull request that is already armed or queued answers the same and is left as it is.
3. **The edge cases are typed.**
   - A repository with "Allow auto-merge" off is refused under its own code (t001 names it, for example `auto_merge_not_allowed`). Its hint says to wait for the checks and merge without the option, or to tell the user.
   - A pull request that is already mergeable gets what t001 records gh 2.105.0 doing in that state (a merge now, or a refusal).
   - A draft stays `pull_request_draft`, and a head that is not the recorded one stays `head_changed`, before any GitHub call.
   - Every other refusal of GitHub's comes back with GitHub's message, as today.
4. **The head pin holds after arming.** t001 records what happens when the session pushes after arming. GitHub keeps auto-merge on after a push by a writer, and the session's push token is one. Either bex disarms auto-merge when a new head is pushed or recorded, or ADR090 D3 says that GitHub merges a later head only after that head's own required checks pass. Tests cover the chosen behavior.
5. **The undo.** If t001 keeps it, a person's "don't merge it after all" can be carried out through the tool (`disablePullRequestAutoMerge`, or `dequeuePullRequest` for a queued pull request), as Devin Review can disable auto-merge. `gh pr merge --auto` and `--disable-auto` stay refused in the `github-api` route, and the refusal names the tool and its option.
6. **The state.** The merge GitHub makes later reaches `prState: merged` on REST, GraphQL, MCP and the dashboard through the existing PR-state poller. It reads an open pull request every five minutes for a week (`lego/backend/internal/agentsessions/pr_state.go`), or sooner when a `pull_request` delivery arrives (ADR094 D7). An armed or queued answer never records `merged`, and neither does the driver's PR record, which the delivery check reads.
7. **Said everywhere.** The overlay's merge step says that when the task or a person asks to merge once the checks pass, the agent marks the PR ready, calls the tool with the option and ends its turn. That matters most where `gh` is off and the agent cannot watch the checks. ADR090 D3 and D4, ADR047 and `lego/agent-image/README.md` say the same.
8. **Tests.** They cover the auto-merge call (its exact body, with `expectedHeadOid` and the method), the queue call, a repository with auto-merge off, the head pin, an already-mergeable pull request, the gateway client's and the driver's acceptance of an armed answer, and the overlay sentence.
9. **Live.** Once a deploy carries the milestone, run the check on production in `tian-personal` with one short session (t005). The session arms auto-merge on a QA repository whose `main` requires a check that takes about a minute, and ends its turn before the check finishes. GitHub merges the pull request as `bex-co[bot]`, and REST, GraphQL and MCP read `prState: merged`. Then delete the session and the repository. Run the merge-queue leg where the QA account can own a queue repository; otherwise t005 records why the queue rests on t002's and t003's tests.

## Source + Goal linkage

- **Source:** inbox note [w10/039](../done/039.md), filed by w10/m538's Devin comparison (t006) on 2026-11-20 and promoted by `/loopx w10` on 2026-11-22.
- **Triage evidence (`7a2b45e6a8`):**
  - **No auto-merge.** `PullRequestMergeContent` takes only `method`, `commitTitle` and `commitMessage` (`lego/backend/internal/agentsession/pullrequest_merge.go:139`), and so does the driver's `validateMergeInput` (`lego/agent-image/driver/src/merge-pr.ts:141`).
  - **The merge is synchronous.** `github.MergePullRequest` sends only `PUT …/pulls/{n}/merge` (`lego/backend/internal/github/merge.go:210`). GitHub says that endpoint "does not support stacked pull requests or merging with a merge queue" ([Pulls](https://docs.github.com/en/rest/pulls/pulls), read 2026-11-22), so a queue branch answers `merge_blocked`.
  - **`gh` cannot do it either.** `enablePullRequestAutoMerge`, `disablePullRequestAutoMerge`, `enqueuePullRequest` and `dequeuePullRequest` are refused with `pullRequestStateRefusal` (`lego/backend/internal/sshgateway/agentcred/githubapi.go:577`).
  - **The overlay tells the agent to wait.** Its merge step says "mark it ready, wait until its required checks pass (`gh pr checks --watch` …), then call `merge_pull_request`" (`lego/agent-image/driver/src/overlay.ts:175`), and the watch is dropped when `gh` is off. w10/042 tracks that `gh pr checks --watch` fails without the pull request's number.
  - **The answer must change on both hops.** The gateway client accepts only `out.Merged && out.SHA != ""` (`pullrequest_merge.go:461`), and the driver accepts only `merged === true` (`merge-pr.ts:277`). So an armed answer needs both changed, and the driver's `recordMerge` must not mark an armed pull request merged.
  - **The state half already exists.** The PR-state poller reads an open pull request every five minutes for its first week (`lego/backend/internal/agentsessions/pr_state.go:52`) and records `merged`, and a `pull_request` delivery updates it at once (ADR094 D7).
- **Reference product (Devin), read 2026-11-22:**
  - [Devin Review](https://docs.devin.ai/work-with-devin/devin-review): "Enable or disable GitHub auto-merge from the merge button dropdown. When enabled, the PR will merge automatically once all required checks pass." "The merge bar shows the current auto-merge status, including who enabled it."
  - The `m` shortcut will "Merge the PR, add or remove it from the merge queue, or enable auto-merge when offered". On the list, `m` "merges or adds the PR to the queue". Its Autoapprover will "let an existing auto-merge or merge queue configuration proceed".
  - Devin's own `gh` token runs `gh pr merge --auto`.
- **GitHub's calls, read 2026-11-22:**
  - `EnablePullRequestAutoMergeInput` takes `pullRequestId`, `mergeMethod`, `commitHeadline`, `commitBody` and `expectedHeadOid`. Its notes say that "when merging with a merge queue any input value for merge method is ignored".
  - `EnqueuePullRequestInput` takes `pullRequestId`, `expectedHeadOid` and `jump`. `DisablePullRequestAutoMergeInput` takes `pullRequestId`. The PullRequest object has `autoMergeRequest`, `isInMergeQueue`, `mergeStateStatus` and `viewerCanEnableAutoMerge` ([GraphQL pulls](https://docs.github.com/en/graphql/reference/pulls)).
  - `PUT …/pulls/{n}/merge-async` takes `merge_action` `default`, `direct_merge` or `merge_queue`, plus `sha`. It answers 202 with a UUID, or 200 when the pull request is already merged or queued, and "An enqueued result is final for the merge queue requests" ([Pulls](https://docs.github.com/en/rest/pulls/pulls)).
  - The recorded `gh pr merge 15108 --squash --auto` (gh 2.105.0, `lego/backend/internal/sshgateway/agentcred/testdata/gh-2.105.0-requests.json`) reads `mergeStateStatus`, `isInMergeQueue` and `isMergeQueueEnabled`, then sends `enablePullRequestAutoMerge`.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../docs/ADR090-agent-owned-delivery.md) D3). It continues [w10/m538](../done/m538/README.md), which shipped the synchronous merge and filed this variant.
- **Expected outcome:**
  - An automation such as `beancount-rt-flaky-test-fixer`, told to merge once green, ends its turn as soon as its pull request is open. It no longer spends sandbox minutes watching CI, and GitHub merges the pull request when the checks pass.
  - A repository that requires a merge queue gets its pull request queued instead of refused.
- **Why now:**
  - The synchronous merge is live (w10/m538, pin `eaaa61506165`), so this is the remaining gap in "merge once CI is green".
  - The prod sandbox node is near capacity (w9/m349). A session that waits on CI holds a sandbox through the whole check run, and one with `gh` off cannot wait at all.
- **Render parity included (reference product: Devin):** the `bex` MCP tool, its refusals and the session's `prState` are cloud coding-agent surfaces. t006 walks them against Devin's auto-merge and merge-queue behavior above. Two candidate drifts are filed as notes, not built here: showing the auto-merge status and who enabled it on the session (Devin's merge bar), and Devin's stacked-PR auto-merge (`includeStack`).
