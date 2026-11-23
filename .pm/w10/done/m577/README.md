# w10 · m577 — A session told to merge once CI is green arms GitHub auto-merge or the merge queue and ends its turn, as Devin can

**Worker:** worker10 **Goal:** "merge once CI is green" no longer keeps a session in its turn watching the checks, and a repository whose base branch requires a merge queue no longer refuses the session's merge outright. `merge_pull_request` takes one option (t001 names it, for example `auto: true`, after `gh pr merge --auto`). With it, bex enables GitHub auto-merge on the session's own recorded pull request, pinned to the head bex recorded, and on a merge-queue branch the pull request goes into the queue. GitHub then lands it when its checks pass, and the session's `prState` reads `merged` once the PR-state poller or a `pull_request` delivery sees it. **Status:** done (t001–t010; live on pin `9fdf7e0a1171`, 2026-11-23: told to merge once CI is green, a session armed auto-merge, got the answer's `next` and ended its turn 89 s before the check finished, without `gh`; GitHub merged as `bex-co[bot]` and REST, GraphQL and MCP read `prState: merged`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: the option, auto-merge vs the queue vs a merge now, the head pin after arming, the undo, the refusals and the answer; ADR090 and ADR047 amended — **DONE** | 1h | — |
| [t002](done/t002.md) | Backend: the GitHub client enables and disables auto-merge and enqueues, pinned to the head — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | Backend: the merge verb and the gateway route take the option and answer an armed or queued pull request — **DONE** | 1h15m | t002 |
| [t004](done/t004.md) | Driver: the tool's option, its answer, the overlay's "arm it and end your turn" and the `gh pr merge --auto` refusal — **DONE** | 1h | t001, t003 |
| [t005](done/t005.md) | Live check on production: a session arms auto-merge and ends its turn, GitHub merges after the check, and `prState` reads merged — **DONE** | 45m | t003, t004, t010 |
| [t006](done/t006.md) | Render parity (Devin) — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006, t007 |
| [t010](done/t010.md) | The armed answer, the overlay and the tool description make the agent end its turn instead of watching the checks (from t005's first run) — **DONE** | 45m | t004 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008, t010 |

Total: **10 tasks, about 7h45m**.

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

- **Source:** inbox note [w10/039](../../done/039.md), filed by w10/m538's Devin comparison (t006) on 2026-11-20 and promoted by `/loopx w10` on 2026-11-22.
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
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../../docs/ADR090-agent-owned-delivery.md) D3). It continues [w10/m538](../../done/m538/README.md), which shipped the synchronous merge and filed this variant.
- **Expected outcome:**
  - An automation such as `beancount-rt-flaky-test-fixer`, told to merge once green, ends its turn as soon as its pull request is open. It no longer spends sandbox minutes watching CI, and GitHub merges the pull request when the checks pass.
  - A repository that requires a merge queue gets its pull request queued instead of refused.
- **Why now:**
  - The synchronous merge is live (w10/m538, pin `eaaa61506165`), so this is the remaining gap in "merge once CI is green".
  - The prod sandbox node is near capacity (w9/m349). A session that waits on CI holds a sandbox through the whole check run, and one with `gh` off cannot wait at all.
- **Render parity included (reference product: Devin):** the `bex` MCP tool, its refusals and the session's `prState` are cloud coding-agent surfaces. t006 walks them against Devin's auto-merge and merge-queue behavior above. Two candidate drifts are filed as notes, not built here: showing the auto-merge status and who enabled it on the session (Devin's merge bar), and Devin's stacked-PR auto-merge (`includeStack`).

## Design record (t001, 2026-11-22)

Settled on `79633aaba2` and implemented in the same change; [ADR090 D3](../../../../docs/ADR090-agent-owned-delivery.md#d3--the-pull-request-verb) (the w10/m577 amendment) is the contract. gh's behavior is read from `pkg/cmd/pr/merge/merge.go` and `http.go` at `cli/cli` tag `v2.105.0` (the pinned gh).

- **The option** is `auto: true`, after `gh pr merge --auto`; the undo is `disableAuto: true`, after `--disable-auto`, and takes no other field (`invalid_request` otherwise, as is `auto` with `disableAuto`). Without a merge queue, `method`, `commitTitle` and `commitMessage` keep today's meaning and go to `enablePullRequestAutoMerge`'s `mergeMethod`, `commitHeadline` and `commitBody` under today's method rule. On a merge-queue branch GitHub ignores all three ("when merging with a merge queue any input value for merge method is ignored", the same for the headline and body, [GraphQL pulls](https://docs.github.com/en/graphql/reference/pulls), read 2026-11-22), so bex sends none, skips the method pre-check and answers no `method`. They are ignored rather than refused, as gh only warns ("The merge strategy for main is set by the merge queue") and proceeds.
- **The state read** is GraphQL `node(id) { ... on PullRequest { isMergeQueueEnabled isInMergeQueue mergeStateStatus autoMergeRequest { mergeMethod } } }`, the fields gh's `NewMergeContext` reads, with a Pull requests read token. `isMergeQueueEnabled` covers rulesets and classic protection, so the `merge_queue` branch rule is not read.
- **The decision table.** `mergeableNow` is gh's `isImmediatelyMergeable`: `CLEAN`, `HAS_HOOKS` or `UNSTABLE`.

  | State | Plain call | `auto: true` |
  | --- | --- | --- |
  | Already in the queue | `{merged: false, mergeQueue: true}`, no call | same |
  | Queue branch, not queued | queue call, `{merged: false, mergeQueue: true}` | same |
  | No queue, auto-merge armed | the synchronous merge, as today | `{merged: false, autoMerge: true, method: <armed>}`, no call |
  | No queue, mergeable now | the synchronous merge | the synchronous merge (gh's `autoMerge: enable && !isImmediatelyMergeable`) |
  | No queue, not mergeable yet (`BLOCKED`, `BEHIND`, `UNKNOWN`, `DIRTY`) | the synchronous merge, which GitHub refuses (`merge_blocked`) | `enablePullRequestAutoMerge` pinned, `{merged: false, autoMerge: true, method}` |
  | GitHub refuses the arm with "Pull request is in clean status" (the state read lags behind checks that just settled) | — | the synchronous merge, pinned |

  The plain call on a queue branch queues the pull request (DoD 2), as gh does (its `shouldAddToMergeQueue` sets `payload.auto = true` with or without `--auto`), as GitHub's button does, and as Devin's `m` "merges or adds the PR to the queue" ([Devin Review](https://docs.devin.ai/work-with-devin/devin-review), read 2026-11-22). A plain call whose state read fails merges as today, so the read can never break the m538 merge. An `auto` call whose read fails is refused (`upstream_error`).

- **The queue call** is `enablePullRequestAutoMerge({pullRequestId, expectedHeadOid})`, gh's call on a queue branch and GitHub's "Merge when ready": it adds a ready pull request to the queue and one whose checks are pending once they pass. `enqueuePullRequest` adds only a ready one, and `merge-async` answers 202 with a UUID to poll, so neither is the first call. GitHub's docs say the repository's auto-merge setting does not apply on a queue branch ([Automatically merging a pull request](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/incorporating-changes-from-a-pull-request/automatically-merging-a-pull-request), read 2026-11-22), but public reports show the mutation refused there with "Auto merge is not allowed for this repository". When GitHub refuses the arm that way, or with "is in clean status", bex sends `enqueuePullRequest({pullRequestId, expectedHeadOid})` with the same pin.
- **The head pin after arming: GitHub's rule, no disarm.** The arm is pinned: `head_changed` before any call when the pull request's head is not the recorded one, and `expectedHeadOid` = the recorded head on the arm and the queue call. GitHub disables auto-merge only when "someone without write permissions pushes new changes to the head branch" (same page), so a later push by the session, which has write access, keeps it armed. GitHub then merges the new head only once that head's own required checks pass. Devin arms with `gh pr merge --auto`, which sends no expected head, and keeps pushing to its branch, so a later head merging after its checks is Devin's observable behavior. bex does not disarm on a push, which would make "fix the failing check" turns re-arm by hand. ADR090 D3's "a commit pushed later is never merged unchecked" still holds for the merge now. For an armed pull request it reads "a later head merges only after its own required checks pass". The overlay says so and names `disableAuto` for a person's "don't merge it after all".
- **The undo is kept.** `disableAuto: true` sends `dequeuePullRequest({id})` for a queued pull request, `disablePullRequestAutoMerge({pullRequestId})` for an armed one, and nothing for one with neither. The answer is `{merged: false, autoMergeDisabled: true}` either way. It is never pinned and runs on a draft too: leaving a pull request unmerged is safe at any head. Devin Review can "Enable or disable GitHub auto-merge", and its `m` can "remove it from the merge queue".
- **Refusals.**
  - `auto_merge_not_allowed` (409): GET `/repos/{owner}/{repo}` reads `allow_auto_merge: false` beside the merge-method settings, with the same Contents-write token. Checked only on a branch without a queue; GitHub's own "Auto merge is not allowed for this repository" maps to the same code. Hint: the setting is off, so wait until the checks pass and call without `auto`, or tell the user a person has to turn it on or merge it.
  - A GraphQL `expectedHeadOid` mismatch ("… does not match") is `head_changed`.
  - FORBIDDEN is `not_permitted`, and RATE_LIMITED is `rate_limited`, since GraphQL errors now carry it.
  - Every other GraphQL refusal of the arm or the queue is `merge_blocked` with GitHub's message.
  - The disarm's refusals go through the shared `gitHubStateRefusal`.
  - Every hint keeps `mergeNoOtherRoute`. `merge_blocked`'s hint now offers `auto: true` first for pending checks, and `head_changed`'s offers it for the retry.
- **The answer.** `{url, number, merged, sha?, method?, headSha}` plus at most one of `autoMerge`, `mergeQueue` or `autoMergeDisabled`. The gateway client (`PullRequestMergeResponse.wellFormed`) and the driver (`mergeAnswer`) accept an answer that names the pull request, carries exactly one outcome, and gives a merge its commit. `recordMerged` runs only on a merge, and so does the driver's `recordMerge`, so the delivery check's `mergedHead` stays empty for an armed pull request.
- **The audit.** It keeps the one verb, `agentsessions.PullRequestMerge`, allowed or denied with the repository, on both hops: arming, queueing and disarming are decisions of the merge verb. Audit detail fields are typed columns per verb (`core.AuditEvent`), and GitHub's own timeline already shows `auto_merge_enabled` by `bex-co[bot]`.
- **Tokens.**
  - The arm and the queue call use the merge's token (Contents, Pull requests and Workflows write, with the w10/014 fallback). GitHub's Dependabot docs run `gh pr merge --auto` with `GITHUB_TOKEN`, an installation token, given `contents: write` and `pull-requests: write` ([Automating Dependabot with GitHub Actions](https://docs.github.com/en/code-security/dependabot/working-with-dependabot/automating-dependabot-with-github-actions), read 2026-11-22). The same page says a merge-queue branch needs "a GitHub App token that has permission to merge" instead of the built-in token.
  - GitHub makes the later merge as the actor who armed it, so Workflows write matters for a pull request that changes a workflow file. [stateful-y/python-package-copier#489](https://github.com/stateful-y/python-package-copier/pull/489) moved its arm to an App token for exactly that.
  - The undo uses the mark-ready token (Contents and Pull requests write), and the state read Pull requests read.
  - t005 confirms the arm live.
- **Docs.** ADR090 D3 (merge call, head pin, tokens, the state, the delivery check) and D4 (the `gh pr merge --auto` refusal), ADR047's PR-delivery amendment, ADR018's m538 row, `lego/agent-image/README.md` and the backend env inventory.

## Render parity (Devin) record (t006, 2026-11-22)

Walked against [Devin Review](https://docs.devin.ai/work-with-devin/devin-review) (re-read 2026-11-22) and gh 2.105.0's `pr merge`. The live half of the `prState` row rests on t005.

| Surface | bex (w10/m577) | Devin | Verdict |
| --- | --- | --- | --- |
| Arm auto-merge | `merge_pull_request({auto: true})` arms GitHub auto-merge pinned to the recorded head and answers `{merged: false, autoMerge: true, method, headSha}` within the call | "Enable … GitHub auto-merge from the merge button dropdown. When enabled, the PR will merge automatically once all required checks pass"; Devin's `gh` runs `gh pr merge --auto` | match |
| Merge queue | a queue branch takes the pull request into the queue with or without `auto` (`mergeQueue: true`); already queued answers the same | `m` "merges or adds the PR to the queue"; GitHub's button on a queue branch is "Merge when ready" | match (live leg: t005) |
| Undo | `{disableAuto: true}` disables auto-merge or dequeues, answering `autoMergeDisabled: true` | "Enable or disable GitHub auto-merge"; `m` can "remove it from the merge queue" | match |
| A pull request mergeable now, with `auto` | merged now, as `gh pr merge --auto` does | "enable auto-merge when offered"; GitHub offers it "only on pull requests that cannot be merged immediately" | match |
| Auto-merge off in the repository | `auto_merge_not_allowed` with a hint to wait and merge without `auto`, or tell the user | Devin Review would not offer the option; gh prints GitHub's "Auto merge is not allowed for this repository" | match (typed) |
| `gh pr merge --auto` / `--disable-auto` in the sandbox | refused in the `github-api` route, the refusal naming `merge_pull_request` with `auto` / `disableAuto` | Devin's `gh` token runs them | same outcome through the tool; the refusal names it (ADR090 D4, w10/m538) |
| The overlay and the tool description | asked to merge once the checks pass: mark ready, call with `auto: true`, report it, end the turn; `disableAuto` when a person says not to merge | Devin arms auto-merge and does not hold a session on CI | match |
| `prState` after GitHub's later merge | the PR-state poller (5 minutes, first week) or a `pull_request` delivery records `merged` on REST, GraphQL, MCP and the dashboard; an armed answer records nothing | the session's PR shows merged | match (live: t005) |
| Auto-merge status and who enabled it | not shown: `prState` stays `open` while armed | "The merge bar shows the current auto-merge status, including who enabled it" | **drift**, filed as [w10/047](../../done/047.md) |
| Stacked pull requests | a taken-over stacked pull request is armed alone (no `includeStack`) | "You can enable GitHub auto-merge on a stacked PR … GitHub lands the PR and every unmerged PR below it" | **drift** (takeover only), filed as [w10/048](../../done/048.md) |

The overlay's merge step and the tool's description no longer name `gh pr checks --watch`, because `auto` replaces the wait. The `merge_blocked` hint still names it as the alternative, so [w10/042](../../done/042.md) keeps that part.

## Simplify record (t007, 2026-11-22)

`/simplify` over the changed backend and driver code:

- The arm branch in `PullRequester.Merge` became a flat `switch` on the arm's error: armed, refused, or mergeable now and merged.
- The GraphQL error mapping `MarkPullRequestReady` had inline is now one helper, `gitHubGraphQL`, shared by the five GraphQL calls. It also maps `RATE_LIMITED` to a rate-limited `*APIError`.
- The token mint with the Workflows fallback is now `mergeToken`, shared by the merge, the arm and the queue call. The method rule is `repositoryMergeSettings.method`, shared by the merge and the arm.
- Kept: the merge state read on a plain merge. It is one more GitHub call per merge, but it is what tells a queue branch apart (DoD 2), and a failed read falls back to the plain merge.

The suites t002–t004 name pass after it.

## Test coverage record (t008, 2026-11-22)

| DoD 8 case | Test |
| --- | --- |
| The auto-merge call's exact body, `expectedHeadOid`, the method in GraphQL's enum, the commit text, the token | `github.TestEnablePullRequestAutoMergePinsTheHeadWithTheMethodRule`; through the verb, `agentsession.TestPullRequestMergeAutoArmsAutoMergeAtTheRecordedHead` |
| The queue call (pin, no method, no settings read, the `enqueuePullRequest` fallback), plain and with `auto`, already queued | `github.TestEnqueuePullRequestPinsTheHeadAndLeavesTheMethodToTheQueue`, `agentsession.TestPullRequestMergeJoinsTheMergeQueue` |
| A repository with auto-merge off (pre-check and GitHub's words) | `github.TestEnablePullRequestAutoMergeNamesGitHubsRefusals`, `agentsession.TestPullRequestMergeAutoTypedRefusals` |
| The head pin before arming, and the unpinned undo | `agentsession.TestPullRequestMergeAutoTypedRefusals` ("a head bex has not recorded", "a draft": no state read, no arm), `TestPullRequestMergeDisableAutoUndoesTheArm` |
| An already-mergeable pull request (state `CLEAN`/`UNSTABLE`, GitHub's "clean status") | `agentsession.TestPullRequestMergeAutoMergesAPullRequestMergeableNow`, `github.TestPullRequestMergeStateReadsWhatGhReads` |
| The gateway client's and the driver's acceptance of an armed answer | `agentsession.TestPullRequestMergeResponseWellFormed`, `agentcred.TestPullRequestMergeRouteArmsAndDisarmsAutoMerge` (end to end), driver `merge-pr.test.ts` ("auto arms auto-merge …", "an answer with no outcome …") |
| The overlay sentence, with `gh` on and off | driver `overlay.test.ts` ("the repo overlay names …"), `github-cli.test.ts` ("… gh off …") |
| The `gh pr merge --auto` / `--disable-auto` refusal | `agentcred.TestGitHubAPIRouteRefusesAutoMergeNamingTheMergeToolsOption` |

Revert proofs (each change reverted, the named test failed, the change restored):

- Dropping `expectedHeadOid` from `EnablePullRequestAutoMerge`'s input fails `TestEnablePullRequestAutoMergePinsTheHeadWithTheMethodRule` ("input = map[… no expectedHeadOid …]").
- Letting the driver call `recordMerge` for every answer fails "merge_pull_request with auto arms auto-merge, returns the armed answer and records nothing merged" (the PR record gained `merged: true`).
- The head-pin-after-arming rule has two code-bearing halves. Skipping the head check for `auto` fails `TestPullRequestMergeAutoTypedRefusals/a_head_bex_has_not_recorded`. Routing `disableAuto` through the draft and head checks fails `TestPullRequestMergeDisableAutoUndoesTheArm/armed` and `/neither`. The rule's third half, that a later head merges only after its own checks, is GitHub's and has no bex code to revert.

## Unblocked (2026-11-23)

- **Finding (verified evidence):** production's pin `ac882b5f557f` (`87ea4b0ee5`, `chore(deploy): pin platform images to ac882b5f557f`, the newest pin on `origin/main`) carries `98092bb49e` and `ba7e6c069e`: `git merge-base --is-ancestor` holds for both, and for w10/040 (`4fef6f30b0`), w10/041 (`ec4d7eb13d`) and w10/042 (`e09869973a`) after them. The running pods match the pin: both `bex-api` and both `bex-ssh-gateway` replicas run `ghcr.io/bex-co/bex-operator@sha256:f5d75a7c48d1…` (the pin's `controller` image, ready, started 11:05Z), and `bex-api`'s `BEX_AGENT_SESSION_IMAGE` is `ghcr.io/bex-co/bex-agent-sandbox@sha256:ae1e4b1b21d2…`, the pin's sandbox image.
- **Gate:** none left. The pin gate cleared by itself; no user action was involved.
- **Next:** t005's live check in `tian-personal` with one short session, then t009. The run (below) found the session watching the checks after arming, so t010 was added and the milestone parked again on a pin that carries t010.

## Live check (t005, production, first run, 2026-11-23)

Workspace `tian-personal`, 11:45–12:01Z, on pin `ac882b5f557f`, which carries `98092bb49e` and `ba7e6c069e`. The workspace's GitHub connection is the `puncsky` installation of the `bex-co` App.

- **Fixture.** A new public repository, `puncsky/qa-20261123-m577-automerge` (disposable), whose `main` required the status check `test` (classic branch protection, no reviews, not strict), from a workflow `ci` whose one job sleeps 90 s (about 94 s end to end), on `pull_request` and pushes to `main`. Settings: "Allow auto-merge" on, merge, squash and rebase allowed.
- **Session.** `bex agent run --repo puncsky/qa-20261123-m577-automerge --max-spend 1` (the CLI built from `main`, `claude` on the default `deepseek/deepseek-v4.1-flash-1017`, template `agent`) started `ags-dc22fn6hv52s73bqmme0` with the task "Make a one-line change to README.md (add a line saying 'Checked by bex QA w10/m577'), open the PR, and merge it once CI is green." The sandbox node had no other session. Three turns cost $0.06 in all.
- **Turn 1: armed and pinned, merged by GitHub.**
  - `open_or_update_draft_pr` opened #1 ready for review (w10/040) at 11:49:20Z with head `3085722256e5`.
  - `merge_pull_request({auto: true})` at 11:49:22Z answered `{merged: false, autoMerge: true, method: "merge", headSha: "3085722256e5…"}` within the call: the repository's first allowed method, pinned to the recorded head. GitHub's timeline shows `auto_merge_enabled` by `bex-co[bot]` at 11:49:25Z, with `autoMergeRequest.enabledBy` `app/bex-co` and `mergeMethod` `MERGE`.
  - `test` ran 11:49:26–11:50:59Z. GitHub merged #1 as `bex-co[bot]` at 11:51:02Z, merge commit `3ab53e1cbcd9`.
- **Turn 1: the session did not end its turn (DoD 9 not met).** Right after the armed answer, the agent's reasoning read "The instruction says end turn, but let me verify it actually merges? … I'll follow the guidance and end turn". It then ran `gh pr checks 1 --watch --interval 15` from 11:49:28Z and ended the turn at 11:51:11Z, after GitHub's merge, reporting the merge commit. So the sandbox was held through the whole check run.
- **The state.** At turn end all three surfaces read `prState: open`, and no `pull_request` delivery changed that. The PR-state poller recorded `merged` at 11:54:45.1Z, 3 min 43 s after GitHub's merge and inside its five minutes. After that, REST `GET /v1/agent-sessions/<id>` and the `prState=merged` list filter, GraphQL `agentSession { prState prStateAt }` and MCP `get_agent_session` all read `prState: merged`. They were read with a QA key minted for the workspace, revoked afterwards.
- **Turn 2, "Allow auto-merge" off: no arm at all.** The agent was asked to make another change, "open a new pull request, and merge it once CI is green". It opened #2 at 11:55:49Z (head `994449ac3e2b`). Its reasoning was "Now wait for CI", so it ran `gh pr checks 2 --watch` (11:55:51–11:57:31Z), then called `merge_pull_request({})`, which merged #2 at once (`{merged: true, sha: "4eff803e0438…", method: "merge"}`). The default model therefore watched the checks in both turns it was asked to merge once CI was green.
- **Turn 3: `auto_merge_not_allowed` (DoD 3, step 2).** With the setting still off, the agent was told to open #3 (head `1ec5b319c12a`) and call `merge_pull_request({auto: true})` at once. It got `auto_merge_not_allowed`, "the repository does not allow auto-merge", with the hint naming the setting, `gh pr checks <number> --watch` (w10/042), a call without `auto`, and telling the user. #3's timeline shows no `auto_merge_enabled`.
- **Audited (step 4).** `audit_events` (read-only, the `bex-db-1` replica) has `agentsessions.PullRequestMerge` rows for target `agent-session:ags-dc22fn6hv52s73bqmme0`:
  - allowed at 11:49:25.885Z and .893Z, the arm of `puncsky/qa-20261123-m577-automerge#1`;
  - allowed at 11:57:37.270Z and .273Z, the merge of #2;
  - denied at 11:59:29.650Z and .653Z, the refusal on #3. Its gateway-side row names the repository without `#3`.

  Each is on both hops.
- **The queue leg (step 3) rests on t002's and t003's tests.** No QA-owned organization exists: the QA account's organizations are `bex-co` and organizations of real projects. `tian-personal`'s GitHub connection is the `puncsky` user installation, and GitHub offers merge queues only on organization-owned repositories, so no queue repository was created. `TestEnqueuePullRequestPinsTheHeadAndLeavesTheMethodToTheQueue` and `TestPullRequestMergeJoinsTheMergeQueue` cover the queue call, the pin and the `enqueuePullRequest` fallback.
- **Found, outside this milestone.** In turn 2 the overlay still said "Earlier turns of this session already opened pull request #1 … there is never a second one", though #1 was merged and `open_or_update_draft_pr` opened #2. The agent hesitated over it before calling the tool. Filed as [w10/052](../../done/052.md).
- **Cleanup.** #3 closed, the session deleted (`404 AGENT_SESSION_NOT_FOUND`), the repository deleted (`404`), the QA key revoked and the QA sign-in logged out.

**Verdict.** DoD 1–3 and 6 hold live, and the queue leg rests on tests. DoD 9 does not: the session armed auto-merge and then waited for the checks anyway. t010 makes the armed answer, the merge step, the `gh` line and the tool description say to end the turn. t005 re-runs step 1 once a pin carries t010.


## Unblocked again (2026-11-23)

- **Finding (verified evidence):** production's pin `9fdf7e0a1171` (`cded300699`, `chore(deploy): pin platform images to 9fdf7e0a1171`, the newest pin on `origin/main`) carries `1c868643c6` (t010): `git merge-base --is-ancestor 1c868643c6 9fdf7e0a1171` holds. The running pods match the pin: both `bex-api` and both `bex-ssh-gateway` replicas run `ghcr.io/bex-co/bex-operator@sha256:df6e69bd31bf…` (the pin's `controller` image, ready, started 17:38Z), and `bex-api`'s `BEX_AGENT_SESSION_IMAGE` is `ghcr.io/bex-co/bex-agent-sandbox@sha256:b04181dd2f88…`, the pin's sandbox image, which carries t010's driver change.
- **Gate:** none left. The pin gate cleared by itself; no user action was involved.
- **Next:** t005 re-runs step 1 in `tian-personal` with one short session, then t009.

## Live check (t005, production, re-run, 2026-11-23)

Workspace `tian-personal`, 19:10–19:19Z, on pin `9fdf7e0a1171`, which carries `1c868643c6` (t010) and, after it, w10/047 and w10/048. Step 1 alone, as t005's re-run section says; steps 2–4 held in the first run.

- **Fixture.** A new public repository, `puncsky/qa-20261123-m577-rerun` (disposable), shaped as the first run's: `main` required the status check `test` (classic branch protection, no reviews, not strict), from a workflow `ci` whose one job sleeps 90 s, on `pull_request` and pushes to `main`. Settings: "Allow auto-merge" on, merge, squash and rebase allowed.
- **Session.** `bex agent run --repo puncsky/qa-20261123-m577-rerun --max-spend 1 --follow --reasoning` (the CLI built from `main` at `a46ff006cd`, `claude` on the default `deepseek/deepseek-v4.1-flash-1017`, template `agent`, `gh` on) started `ags-dc28vnmurl5c7380qua0` at 19:11:24Z with the first run's task, word for word: "Make a one-line change to README.md (add a line saying 'Checked by bex QA w10/m577'), open the PR, and merge it once CI is green." The sandbox node had no other session. One turn cost $0.006.
- **Armed, and the turn ended before the check finished (DoD 9).**
  - The agent read the README and the workflow, committed and pushed (19:11:45–47Z), and `open_or_update_draft_pr` opened #1 ready for review at 19:11:52.9Z with head `5237bfd28557`.
  - Its reasoning then read "Now merge once CI green — use auto: true and end turn." `merge_pull_request({auto: true})` (19:11:54.7–58.9Z) answered `{merged: false, autoMerge: true, method: "merge", headSha: "5237bfd28557…", next: "Auto-merge is armed: GitHub merges this pull request once its required checks pass. End your turn now, without waiting for or watching the checks. Your final reply says auto-merge is armed, not that the pull request merged."}`. GitHub's timeline shows `auto_merge_enabled` by `bex-co[bot]` at 19:11:57Z (`autoMergeRequest.enabledBy` `app/bex-co`, `mergeMethod` `MERGE`).
  - The final reply at 19:12:00.6Z said "Auto-merge is armed, so GitHub will merge it as soon as the required `ci` check passes", and the turn finished at 19:12:02.3Z (session `succeeded`). The turn's three shell commands were `git status`, `cat` and the commit and push: no `gh` at all, so no `gh pr checks --watch`.
  - `test` ran 19:11:59–19:13:31Z, so the turn ended 89 s before the check finished. At 19:12:16Z the pull request was open, `mergeStateStatus` `BLOCKED`, `test` in progress.
- **Merged by GitHub.** GitHub merged #1 as `bex-co[bot]` at 19:13:36Z, 5 s after `test` passed: merge commit `da8b8d2e78d8`, parents `6060c6fa7557` and `5237bfd28557`.
- **Audited.** `audit_events` (read-only, the `bex-db-1` replica) has `agentsessions.PullRequestMerge` allowed at 19:11:58.905Z and .910Z for target `agent-session:ags-dc28vnmurl5c7380qua0`, target name `puncsky/qa-20261123-m577-rerun#1`, on both hops, after the two `agentsessions.PullRequest` rows of the open at 19:11:52.9Z.
- **The state.** While armed, REST and GraphQL read `prState: open` with w10/047's `prAutoMerge {method: "merge", enabledBy: "bex-co", enabledAt: 19:11:57Z}` and `prMergeQueued: false`. The PR-state poller recorded `merged` at 19:17:26.3Z, 3 min 50 s after GitHub's merge and inside its five minutes, and cleared `prAutoMerge`. Then REST `GET /v1/agent-sessions/<id>` and the `prState=merged` list filter, GraphQL `agentSession { prState prStateAt prAutoMerge prMergeQueued }` and MCP `get_agent_session` all read `prState: merged`. They were read with a QA key minted for the workspace, revoked afterwards.
- **Cleanup.** The session deleted (`404 AGENT_SESSION_NOT_FOUND`), the repository deleted (`404`), the QA key revoked and the QA sign-in logged out.

**Verdict.** DoD 9 holds: told to merge once CI is green, the session armed auto-merge while the check ran, got `next`, and ended its turn 89 s before the check finished, without `gh pr checks --watch`; GitHub merged as `bex-co[bot]` and REST, GraphQL and MCP read `prState: merged`. With the first run's DoD 1–3 and 6 and the shipped DoD 4, 5, 7 and 8, the definition of done holds, and t009 closed the milestone on 2026-11-23.
