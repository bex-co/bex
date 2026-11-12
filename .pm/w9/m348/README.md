# w9 · m348 — A `/bex` comment on a pull request starts a session that takes it over, as Devin's `/devin` does

**Worker:** worker9 **Goal:** a write-access collaborator who comments `/bex <task>` on an open pull request no session owns gets a new agent session working on that PR's branch, and a reply on the PR with the session link. A `/bex` comment on a PR a session already owns is feedback for that session. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A session can take over an existing same-repository pull request: it works on, and pushes to, that PR's own branch | 90m | — |
| [t002](t002.md) | Intake: a `/bex` comment on an open PR no session owns starts a session as the commenter's member, and the App replies with its link | 90m | t001 |
| [t003](t003.md) | Inline review comments (`pull_request_review_comment`) take `/bex` and feed the owning session, and the session page shows the comment that started it | 45m | t002 |
| [t004](t004.md) | Docs: ADR047 D11, ADR026 webhook events and ADR094 D3's branch rule record the takeover | 20m | t003 |
| [t005](t005.md) | Live check on production: `/bex` on a QA pull request starts a session that pushes to its branch, and a second `/bex` steers it | 45m | t004 |
| [t006](t006.md) | Render parity (Devin): `/bex` matches Devin's `/devin` and the session reads the same on REST, GraphQL, MCP and the dashboard | 20m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

## Definition of done

On production:

- A `/bex fix the failing test` comment (any case, first word) on an open same-repository PR that no session owns, from a write-access collaborator linked to a member with `can_create`, starts one session. It works on the PR's head branch, its pushes land on that PR, and the App replies on the PR with the session link.
- The same comment redelivered starts nothing more. A `/bex` comment on a PR a session owns steers that session. A bot, a commenter without write access or without a linked member, a closed or merged PR, and a plain issue start nothing.
- A fork PR gets a reply that bex cannot push to a fork, and no session.
- The session page shows the comment that started it.

## Source + Goal linkage

- **Source:** w9/085, promoted 2026-11-12 by `/loopx w9`. The note grew past an hour once triage found that sessions can only push `bex-agent/*` branches (`lego/backend/internal/agentsessions/service.go:491-495`, `agentsession.BranchPrefix`) and always start from the default branch, so a session cannot work on someone else's PR today.
- **Devin reference** (docs.devin.ai/integrations/gh, read 2026-11-12):
  - `/devin` must begin the comment and is case-insensitive.
  - It works in PR conversation comments and in inline review comments.
  - The commenter needs `write` or `admin` on the repository and a linked account.
  - Devin "starts a new session with that repository, uses the PR as context (taking over the PR if appropriate)" and "replies on the PR with a link to the session".
  - If a session already works on the PR, the comment goes to that session instead.
  - Comments on issues, and on closed or merged PRs, are ignored.
- **Recommendation recorded, not a user decision:** the session pushes to the PR's own branch (Devin's "taking over"), not to a stacked `bex-agent/*` PR. This is the Devin-consistent option under ADR047 / ADR008 § Render parity first, and it relaxes ADR094 D3's `bex-agent/` prefix rule for this path only:
  - a session that takes over a PR is authorized for exactly that one branch, as every session is for its own;
  - only same-repository PRs qualify;
  - the trigger needs a commenter who could push that branch themselves.

  The amendment's sign-off is still open. The stacked-PR variant is a follow-up if the user prefers it.
- **Goal linkage:** pillar 5 (cloud coding agents), Devin parity for PR collaboration (ADR047 D11, after w9/m344).
- **Expected outcome:** reviewers hand work to an agent from the PR itself, without opening the dashboard, and the agent's commits land on the PR under review.
- **Why now:** w9/m344 already receives, authorizes and routes PR comments; this adds the one Devin entry point that session-less PRs lack.
- **Gate:** the live check needs the same GitHub App event subscriptions w9/m344 is blocked on (Issue comment, Pull request review, and Pull request review comment for t003).
- **Parity:** included (t006), against Devin, because the change adds a GitHub-facing entry point and a session field read on REST, GraphQL, MCP and the dashboard.
