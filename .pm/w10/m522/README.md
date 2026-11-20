# w10 · m522 — An agent session's `gh` comments, reviews, labels and re-runs CI on its repository, as Devin's can, while pull requests stay with the `bex` tools

**Worker:** worker10 **Goal:** in a repo session, `gh` writes what Devin's `gh` token lets Devin write on the repository it works on: comments and review replies on pull requests, reviews, labels and requested reviewers, issues, and re-running or cancelling Actions runs. Each write goes through the gateway `github-api` route with a token minted for that call and narrowed to the repository, with no token in the sandbox. Opening or changing a pull request itself, approving one, and anything that changes code, refs or settings stay refused, naming the `bex` tools, so the session's one pull-request path and its branch confinement hold. **Status:** in progress (t001–t004 and t006–t008 done 2026-11-19; t005, the live check, waits for a deploy that carries the milestone)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: record what each `gh` write sends, decide the admitted writes, the refusals and the write token; ADR090, ADR047 and ADR026 amended — **DONE** | 1h | — |
| [t002](done/t002.md) | Backend: a per-call write token for the session's repository, minted through its own internal verb and audited — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | Gateway: the `github-api` route admits the decided writes with the write token and still refuses every other write naming the `bex` tools — **DONE** | 2h | t001, t002 |
| [t004](done/t004.md) | Driver: the overlay says what `gh` can write and what stays with the `bex` tools — **DONE** | 30m | t001 |
| [t005](t005.md) | Live check on production: the admitted writes land on the session's repository, the rest are refused | 45m | t003, t004 |
| [t006](done/t006.md) | Render parity (Devin) — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 7h15m**.

## Definition of done

1. **The writes land.** In a repo session, with the pinned `gh` 2.105.0 and no argument naming the repository: `gh pr comment <n> --body …` (and `--edit-last`), a reply in a review thread (`gh api -X POST repos/{owner}/{repo}/pulls/<n>/comments/<id>/replies`), `gh pr review <n> --comment` and `--request-changes`, `gh pr edit <n> --add-label/--remove-label/--add-reviewer`, `gh issue create`, `gh issue comment`, `gh run rerun <id>`, `gh run rerun <id> --failed`, `gh run rerun --job <id>` and `gh run cancel <id>` succeed on the session's repository, with the exact set as t001 records it.
2. **The token.** Each admitted write reaches GitHub with a token minted for that call, narrowed to the session's repository, carrying `pull_requests`, `issues` and `actions` write and no `contents`, `workflows`, `statuses`, `checks`, `administration` or other write. Reads keep the read-only token of w10/m513. An installation that has not granted one of the write permissions still gets a token for the others, and `gh` then prints GitHub's own 403 for that part.
3. **The rest stays refused.** `gh pr create`, `gh pr edit --title/--body/--base`, `gh pr ready`, `gh pr merge`, `gh pr close`/`reopen`, `gh pr review --approve`, `gh workflow run`, a contents, ref, release or settings write through `gh api`, and any write to another repository answer `403` in GitHub's error shape before anything reaches GitHub, with a message that says what `gh` can write here and names `open_or_update_draft_pr` and `reply_on_pull_request`. A GraphQL document that mixes an admitted mutation with a refused one is refused whole.
4. **Accountable.** Every write mint is audited (allowed and denied, with the repository), writes count against the route's per-session and per-workspace budgets, and a comment the session posts with `gh` does not steer the session (a Bot author never steers, `lego/backend/internal/agentsessions/pr_feedback.go:380`).
5. **Said everywhere.** The overlay, ADR090 D4, ADR047 D2 and ADR026 (the App manifest for self-hosters gains Actions and Issues write) describe what `gh` writes, what stays refused and why.
6. **Live.** On production, once a deploy carries the milestone: a session on a QA repository comments on its own draft pull request with `gh pr comment`, labels it, re-runs a failed run's jobs with `gh run rerun --failed`, and is refused `gh pr create` and `gh pr merge` with the message naming the `bex` tools; the comment does not steer the session.

## Source + Goal linkage

- **Source:** inbox note [w10/025](../done/025.md), filed by w10/m513's worker from its Devin comparison (t007) on 2026-11-19, promoted by `/loopx w10` on 2026-11-19.
- **Triage evidence (2026-11-19, `aef913d70`):**
  - **Every write is refused.** The route answers every REST method but `GET`/`HEAD` (`lego/backend/internal/sshgateway/agentcred/githubapi.go:283`) and every GraphQL mutation (`:328`) with `readOnlyRefusal` (`:293`), and its token is read-only (`lego/backend/internal/github/client.go:270`).
  - **The `bex` tools cover only part of it.** `reply_on_pull_request` posts one conversation comment on the session's own recorded pull request (`agentsession/pullrequest_comment.go:102`, body only). Nothing re-runs or cancels CI, replies in a review thread, reviews, labels or opens an issue.
  - **No App gate in production.** The production GitHub App (`bex-co`) and all five of its installations already grant `actions`, `issues`, `pull_requests` and `statuses` write (read 2026-11-19 through the App JWT), so production needs no permission change. The self-hosters' manifest in ADR026 § 2 lists `actions:read` and no `issues`, so a self-hosted App gains them by the usual permission change and installation approval.
  - **No self-steer loop.** PR feedback ignores every author whose type is not `User` (`pr_feedback.go:380`), and a write with the installation token posts as the App's bot.
- **Reference product (Devin):** "Devin's machine can carry a GitHub CLI (`gh`) token that lets Devin use GitHub features directly through the GitHub API" ([Security profiles](https://docs.devin.ai/product-guides/security-profiles), read 2026-11-19). Devin's GitHub App holds pull requests write ("create new PRs"), Actions write ("…and to rerun workflows"), issues write ("open new issues"), commit statuses write and contents write ([GitHub integration](https://docs.devin.ai/integrations/gh), read 2026-11-19). DoD 1 matches those writes on the session's repository. bex keeps two hardenings Devin does not document, and t001 records each in ADR090 with its reason. Pull requests are opened and changed only by `open_or_update_draft_pr`, because ADR090 D3 records the PR and its footer there. Code and refs move only through the Git proxy, which accepts only the session branch. t006 files any write Devin allows that this milestone refuses (approvals, workflow dispatch, commit statuses, PR state changes) as its own item if it matters.
- **Open question in the note, settled under the Devin rule:** the note asked the user which writes `gh` may make. Under [ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)'s reference-product rule (Devin, user preference 2026-10-30), the parity-sufficient answer is Devin's writes minus the two hardenings above. That needs no decision, so it is not a gate. A user who wants the stricter variant (no `gh` writes at all) has w10/026, the workspace switch that removes `gh`.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../docs/ADR090-agent-owned-delivery.md)). It continues [w10/m513](../done/m513/README.md), which gave `gh` its reads.
- **Expected outcome:** an agent that finds a flaky failure re-runs it and reads the result. An agent answers a reviewer in the review thread where the question was asked. A routine labels its pull request or opens an issue for what it found but did not fix. None of this needs a person or a new `bex` tool.
- **Why now:** w10/m513 made `gh` the agent's natural GitHub client, and the refusals now meet agents mid-task: the beancount-io routines (`beancount-rt-flaky-test-fixer` and the others) could confirm a flake only by re-running CI. Production's App already holds every permission needed.
- **Render parity included (reference product: Devin):** the sandbox's `gh` is a cloud coding-agent surface. t006 walks every admitted and refused write against Devin's `gh` token and App permissions. No REST, GraphQL, MCP or dashboard field changes.

## Direction for t001 (decided in the design record below)

- **Admitted (the default set):** on `/repos/<the session's repository>/…` only, REST `POST`/`PATCH`/`DELETE` for issue and pull-request comments, review comments and their replies, reviews other than `APPROVE`, reactions, labels, requested reviewers, issues (create, comment, edit, close), and `POST …/actions/runs/{id}/rerun`, `…/rerun-failed-jobs`, `…/cancel`, `…/actions/jobs/{id}/rerun`. For GraphQL, the mutations `gh` sends for the same commands (for example `addComment`, `updateIssueComment`, `addPullRequestReview` and `addPullRequestReviewThreadReply`, `addLabelsToLabelable`/`removeLabelsFromLabelable`, `requestReviews`, `createIssue`). Their node ids are scoped by the token's one-repository narrowing, which GitHub enforces, as `node` reads are today.
- **Refused, naming the `bex` tools:** `POST …/pulls`, `PATCH …/pulls/{n}` and the `createPullRequest`, `updatePullRequest`, `markPullRequestReadyForReview`, `convertPullRequestToDraft`, `mergePullRequest`, `closePullRequest`/`reopenPullRequest` and auto-merge mutations (the D3 verb owns the PR record). `APPROVE` reviews: an App approval may satisfy branch protection on a person's pull request; t001 checks GitHub's rule and records it. `…/dispatches`: a dispatch starts a new run on any ref with chosen inputs, while a re-run repeats a run that already ran. Contents, git data, refs, releases, statuses, checks, deployments, environments, hooks, secrets, variables, collaborators and settings. Every other repository.
- **The token:** reads plus `pull_requests`, `issues` and `actions` write, minted only for an admitted write. For an installation missing a write permission, either intersect with the installation's granted permissions or retry on GitHub's 422 without that permission; t001 picks one.
- **Verify, don't assume:** record the method, path or mutation each command sends with the pinned `gh` 2.105.0 through the driver's socket, as w10/m513 t003 did for reads.

## Design record (t001, 2026-11-19)

**How it was recorded.** gh 2.105.0 (the image's pin; the macOS arm64 build, checked against the release's `gh_2.105.0_checksums.txt`) ran with `http_unix_socket` pointed at a recording socket. The socket forwarded reads (REST `GET`/`HEAD`, GraphQL documents with no `mutation` or `subscription`) to `api.github.com` and answered every other request `403` itself, so no write reached GitHub. Targets: public `cli/cli` (pull requests #15108, #15095, #14927 (a draft) and #15000, issues #15104 and #15110, runs 44577807157 and 44652252248, job 136009738176), and `bex-co/bex#197`, which has a comment by the recording account, for `--edit-last` and `--delete-last`. The requests are kept, with the repository renamed `octo/repo`, as `lego/backend/internal/sshgateway/agentcred/testdata/gh-2.105.0-requests.json`, which `TestGitHubAPIRouteReplaysTheRecordedGhCommands` sends through the route.

**What each command sends.** Every read before a write is a `repository(owner:, name:)` or `__type` query, or a REST `GET` under `/repos/<repo>/`, which the w10/m513 read policy already admits.

| Command | Reads first | The write | Verdict |
| --- | --- | --- | --- |
| `gh pr comment <n> --body` | `PullRequestByNumber` | `addComment` | admitted |
| `gh pr comment --edit-last` / `--delete-last` | `PullRequestByNumber` | `updateIssueComment` / `deleteIssueComment` | admitted |
| `gh api -X POST repos/{owner}/{repo}/pulls/<n>/comments/<id>/replies` | — | REST `POST …/pulls/<n>/comments/<id>/replies` | admitted |
| `gh pr review --comment` / `--request-changes` | `PullRequestByNumber` | `addPullRequestReview`, `event` `COMMENT` / `REQUEST_CHANGES` in `$input` | admitted |
| `gh pr review --approve` | `PullRequestByNumber` | `addPullRequestReview`, `event` `APPROVE` | refused |
| `gh pr edit --add-label` / `--remove-label` | `PullRequestByNumber`, `PullRequestProjectItems`, `RepositoryLabelList` | `addLabelsToLabelable` / `removeLabelsFromLabelable` | admitted |
| `gh pr edit --add-reviewer` / `--remove-reviewer` | `PullRequestByNumber`, `PullRequestProjectItems` | `updatePullRequest` with only `pullRequestId`, and `requestReviewsByLogin` | admitted |
| `gh pr edit --add-assignee` | as above | `replaceActorsForAssignable` | admitted |
| `gh pr edit --title` / `--body` / `--base` | as above | `updatePullRequest` with `title` / `body` / `baseRefName` | refused |
| `gh issue create [--label]` | `IssueRepositoryInfo` [`RepositoryLabelList`] | `createIssue` | admitted |
| `gh issue comment` | `IssueByNumber` | `addComment` | admitted |
| `gh issue edit --title/--body/--add-label`, `gh issue close`, `gh issue reopen` (an issue) | `IssueByNumber` | `updateIssue`, `addLabelsToLabelable`, `closeIssue`, `reopenIssue` | admitted |
| `gh issue edit --title`, `close`, `reopen` on a pull request's number | `IssueByNumber` | `updatePullRequest`, `closePullRequest`, `reopenPullRequest` | refused |
| `gh run rerun <id>` (`--debug`) | `GET …/actions/runs/<id>`, `GET …/actions/workflows/<id>` | REST `POST …/actions/runs/<id>/rerun` (body `{"enable_debug_logging":true}`) | admitted |
| `gh run rerun <id> --failed` | as above | REST `POST …/rerun-failed-jobs` | admitted |
| `gh run rerun --job <id>` | `GET …/actions/jobs/<id>` | REST `POST …/actions/jobs/<id>/rerun` | admitted |
| `gh run cancel <id>` (`--force`) | as `rerun` | REST `POST …/cancel` (`…/force-cancel`) | admitted |
| `gh pr create` | `RepositoryInfo`, `PullRequestForBranch` | `createPullRequest` | refused |
| `gh pr ready` / `--undo` | `PullRequestByNumber` | `markPullRequestReadyForReview` / `convertPullRequestToDraft` | refused |
| `gh pr merge [--admin]` / `--auto` | `PullRequestByNumber` | `mergePullRequest` / `enablePullRequestAutoMerge` | refused |
| `gh pr close [--delete-branch]` | `PullRequestByNumber` | `closePullRequest` | refused |
| `gh pr update-branch` | `PullRequestByNumber`, `ComparePullRequestBaseBranchWith` | `updatePullRequestBranch` | refused |
| `gh workflow run` | `GET …/actions/workflows/<file>` | REST `POST …/actions/workflows/<id>/dispatches` | refused |
| `gh run delete` | as `rerun` | REST `DELETE …/actions/runs/<id>` | refused |
| `gh issue develop` | `ListLinkedBranches`, `FindRepoBranchID` | `createLinkedBranch` | refused |
| `gh issue transfer` / `delete` / `lock`, `gh pr lock` | `IssueByNumber` (and the target repository) | `transferIssue` / `deleteIssue` / `lockLockable` | refused |
| `gh label create/edit/delete`, `gh release create`, `gh variable set` | — | REST `POST …/labels`, `PATCH`/`DELETE …/labels/<name>`, `POST …/releases`, `POST …/actions/variables` | refused |
| `gh api -X PUT …/contents/…`, `-X PATCH …/git/refs/…`, `-X POST …/statuses/<sha>`, a write to another repository | — | as sent | refused |

`gh pr ready` on a ready pull request, `gh pr reopen` on a merged one and `gh pr merge` without `--admin` on a blocked one stop in gh before any write, so those rows were recorded on other targets or with `--admin`.

**Decisions.** ADR090 D4's w10/m522 amendment has each with its reason; in short:

- **Admitted:** the table's admitted rows, and beside them reactions, review comments and threads, editing and deleting a pending review or a review comment, and the REST forms of each (`githubAPIWrites` and `githubAPIMutations` in `githubapi.go`). An `updatePullRequest` is admitted only when it sets nothing but labels, assignees or the milestone. REST `PATCH issues/<n>` is refused because GitHub treats a pull request as an issue there. GitHub types GraphQL ids: on 2026-11-19, `updateIssue` with a closed pull request's id in `bex-co/bex` (a no-op if it had resolved), and `closeIssue` and `updateIssueComment` with a `cli/cli` pull request's id, each answered "Could not resolve to Issue node" or "…IssueComment node", so the GraphQL issue writes cannot reach a pull request.
- **Approvals refused:** GitHub counts an approving review from an App installation with write access toward required approvals, as a person's. The App also holds `contents` write for session pushes, so a session could approve a person's pull request into a protected branch. Dismissals and `resolveReviewThread` are refused for the same reason.
- **The token:** `pull_requests`, `issues` and `actions` write, plus `checks`, `contents` and `statuses` read, for the one repository. An installation missing part of it is served by **intersection**: on a 422 the mint reads `GET /app/installations/{id}` and asks again for what it granted. A retry chain cannot know which permission a 422 is about, and the extra read happens only after a 422 (never on production, whose installations grant all of it).
- **The verb:** `/v1/agent-session-github-write-credentials`, the read verb's HMAC and checks, audit verb `agentsessions.MintGitHubWriteCredential`, gateway env `BEX_AGENT_GITHUB_WRITE_API_URL` with the in-cluster default.
- **Bodies:** up to 1 MiB (GitHub's 65536-character comments can take several bytes a character once JSON-escaped), one JSON value that repeats no key, and no query string on a write.
- **No separate write budget.** Writes share the route's 1000-per-session and 10000-per-workspace budget, which exists so `gh` cannot spend the Git exchanges a session needs. GitHub's secondary rate limits bound bursts of content creation for the installation as they do for Devin's App.
- **The refusal:** "bex: gh in this session reads <repo> and writes to it only to comment and reply, review without approving, label, assign, request reviewers, open, edit and close issues, and re-run or cancel Actions runs. Open or update the pull request with the open_or_update_draft_pr tool of the bex MCP server, reply on it with reply_on_pull_request, and change code only by pushing your session branch."

## Render parity (Devin, t006, 2026-11-19)

Re-read [Security profiles](https://docs.devin.ai/product-guides/security-profiles) and [GitHub integration](https://docs.devin.ai/integrations/gh) on 2026-11-19. Devin's `gh` token carries its App's permissions: Actions ("view the actions configured … rerun workflows"), checks, commit statuses ("view and set commit statuses"), contents, discussions, issues ("open new issues"), pull requests ("create new PRs"), projects, workflows and repository creation, all read and write, on every repository of the installation.

| Devin's `gh` can | bex's session `gh`, after this milestone | Gap |
| --- | --- | --- |
| Comment, reply in review threads, review, label, assign, request reviewers | Same, on the session's repository | none (no approvals, below) |
| Open, comment on, edit and close issues | Same | none |
| Re-run and cancel workflow runs | Same, runs and jobs | none |
| Create pull requests and edit their title and body | Through `open_or_update_draft_pr`, which records the PR and its footer | none an agent needs: the tool does both |
| Mark ready, merge, close a pull request | Refused | filed as [w10/034](../034.md) |
| Dispatch a workflow (Actions write) | Refused | filed as [w10/033](../033.md), for the session's own branch |
| Approve a pull request | Refused | not filed: Devin documents no approvals, and an App approval satisfies required reviews |
| Set commit statuses, check runs | Refused | not filed: a session's CI results come from the repository's own workflows; nothing in bex reports through statuses |
| Push code, create branches and tags (contents, workflows) | Only `git push` of the session branch through the Git proxy | none an agent needs (ADR047's branch confinement) |
| Repository labels, milestones, locks, transfers, discussions, projects, new repositories | Refused or no route | not filed: no session task has needed them; a person can file one if it does |
| Any repository of the installation | The session's repository only | not filed: a session works on one repository (ADR047); w10/026 and w10/027 already cover the `gh` switch and the shells |

No REST, GraphQL, MCP or dashboard field changed. ADR018 has no `gh` row, so it is unchanged.
