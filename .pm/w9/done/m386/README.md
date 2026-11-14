# w9 · m386 — Sessions report their PR's state: open, merged or closed (Devin v3 `pr_state`)

**Worker:** worker9 **Goal:** every session that opened a pull request shows whether it is open (a draft too), merged or closed on REST, GraphQL, MCP and `/agents`, filterable by state, as Devin's v3 session reads do — kept current by bex reading the PR, so it needs no new GitHub App event. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Store: `pr_state` and `pr_state_at` on agent sessions — **DONE** | 45m | — |
| [t002](done/t002.md) | Reconciler: read each session PR's state from GitHub and backfill existing sessions — **DONE** | 90m | t001 |
| [t003](done/t003.md) | Webhook: apply `pull_request` closed, reopened and ready-for-review events when the App delivers them — **DONE** | 60m | t002 |
| [t004](done/t004.md) | REST, GraphQL and MCP: PR state on the session view and a `prState` list filter — **DONE** | 90m | t002 |
| [t005](done/t005.md) | Dashboard: PR state badge and filter on `/agents` and the session page — **DONE** | 90m | t004 |
| [t006](done/t006.md) | Live check on production: merging one QA PR and closing another updates both sessions — **DONE** | 45m | t003, t005 |
| [t007](done/t007.md) | Render parity (Devin): every surface this milestone touched agrees, and matches Devin — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007, t008 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t009 |

## Definition of done

On production (QA user), with the GitHub App subscribed only to `push` and `check_suite`:

- A session whose draft PR is merged reads `merged` within the reconcile interval on REST, GraphQL, MCP and `/agents`; a closed one reads `closed`; a reopened one reads `open` again.
- `GET /v1/agent-sessions?prState=merged` (and the GraphQL and MCP equivalents) lists only sessions with merged PRs.
- Sessions that already had a PR before the change carry a state after the backfill.
- When the App does deliver `pull_request`, the state updates on the event (covered by tests).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w9` (2026-11-13), item 2, filed by `/pmx` 2026-11-13.
  - Devin: v3 `SessionPullRequest` requires `pr_url` and `pr_state` (v3 OpenAPI, checked 2026-11-13); Devin's PR metrics API counts merged and closed PRs.
  - bex: the session view has only `prUrl` and `prNumber` (`lego/backend/internal/agentsessions/models.go:289-290`); the webhook ignores every event but `push` and `check_suite` (`lego/backend/internal/apps/webhook.go:443`), and the App is subscribed to those two only (`docs/ADR026-github-integration.md:30`). `github.Client.GetPullRequest` already returns `State` and `Merged` (`lego/backend/internal/github/feedback.go:154-170`).
- **Goal linkage:** Devin parity; the PR is the agent's deliverable (ADR090).
- **Expected outcome:** `/agents` shows which work landed and which was abandoned, and clients stop polling GitHub for it.
- **Why now:** m344 and m348 made the PR the centre of a session. The agent-usage breakdown (a runner-up of the same brainstorm) needs merged-PR counts. Reading PRs needs no user action, unlike m344/m348's event gate.
- **Render parity:** included (t007), against Devin, because the change touches REST, GraphQL, MCP and the dashboard.
