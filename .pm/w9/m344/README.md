# w9 · m344 — PR feedback loop: comments, reviews and CI failures on the agent's draft PR steer its session

**Worker:** worker9 **Goal:** feedback on an agent's draft PR reaches the agent, as with Devin. A review or comment from someone with write access whose GitHub account is linked to a workspace member starts a new turn on the same branch; a failed CI run on the head starts one automatic fix attempt; the agent replies on the PR; bots, unauthorized commenters and loops are ignored. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Amend ADR047/ADR090: who may steer a session from its PR, which events count, and the loop guards | 60m | — |
| [t002](t002.md) | GitHub App: receive comment and review events, and probe that an installation accepted them | 30m | t001 |
| [t003](t003.md) | Route a PR comment or review to its session's Steer, as quoted untrusted input | 90m | t002 |
| [t004](t004.md) | A failed CI run on the session's head steers one automatic fix attempt | 60m | t003 |
| [t005](t005.md) | The agent can reply on its PR through the gateway | 60m | t003 |
| [t006](t006.md) | Dashboard: a turn says what steered it — a PR comment from @user or a CI failure | 45m | t003 |
| [t007](t007.md) | Live end-to-end on production: a PR comment steers the session, which pushes a fix and replies | 45m | t004, t005, t006 |
| [t008](t008.md) | Render parity (Devin): the PR feedback loop matches Devin across webhook behavior, REST/GraphQL/MCP session views and the dashboard | 20m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 45m | t008, t009 |
| [t011](t011.md) | Closeout | 10m | t010 |

## Definition of done

On production, with the GitHub App's new events enabled:

- A review comment from a write-access, linked workspace member on an agent's draft PR starts exactly one new turn on the same `bex-agent/*` branch; the agent pushes and replies on the PR.
- A failed check suite on the session's head starts exactly one automatic steer naming the failed checks.
- Comments from bots, unlinked or read-only accounts, closed PRs and canceled sessions do nothing; a webhook redelivery does not steer twice.
- The session page marks each PR- or CI-steered turn with its origin.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w9` (2026-11-11), item 2, filed by `/pmx` 2026-11-12.
  - ADR047:114 and ADR090 D7:115 name the PR-comment loop as a follow-on; it was never filed.
  - `lego/backend/internal/apps/webhook.go:346-358` ignores every GitHub event except `push`, `delete` and `check_suite` (checked at `3dd698375`); `checks_pass.go:250` parses `check_suite` only for deploy gating.
  - Devin replies to comments on PRs its sessions work on, needs write access plus a linked account, ignores bots by default and always processes CI failures ([GitHub integration](https://docs.devin.ai/integrations/gh), [PR bot comment settings](https://docs.devin.ai/product-guides/bot-comment-settings)).
- **Goal linkage:** pillar 5 (cloud coding agents), Devin parity for PR delivery (ADR047, ADR090).
- **Expected outcome:** a reviewer corrects an agent PR with a comment instead of opening a new session; automation PRs (eight daily `rt-*` runs since 2026-11-11) can iterate to merge-ready.
- **Why now:** automations and child sessions now open PRs nobody is watching live, and this is the largest Devin behavior bex still lacks.
- **User gate:** t002/t007 need the user to enable `issue_comment`, `pull_request_review`, `pull_request_review_comment` and the matching write permissions on the production GitHub App. Implementation proceeds without it; the live check parks in `blocked/` until it is done.
- **Recommendations recorded (not user decisions):** the authorization rule (write access + linked member with `can_create`, run as the session creator) and the bot/loop rules follow Devin and are labeled as such in t001's ADR text.
- **Follow-up (not in this milestone):** the `/bex` PR command that starts a new session from any PR, and GitHub-event automation triggers (ADR101:199, :230), which reuse this plumbing.
- **Parity:** included (t008), against Devin, because the change touches the session views on REST/GraphQL/MCP and the dashboard.
