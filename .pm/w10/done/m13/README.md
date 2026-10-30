# w10 · m13 — Agent-owned delivery: the coding agent commits, pushes and opens its draft PR

**Worker:** worker10 **Goal:** A bex cloud coding-agent session delivers its work the way a human engineer (and Devin / Open SWE / Copilot cloud agent) does: the **agent** writes logically-split commits with real messages, pushes its own `bex-agent/<id>` branch through the existing Pod-bound Git proxy, and opens or updates its draft PR through a platform tool. The PR body follows the repo's `PULL_REQUEST_TEMPLATE` and carries a server-appended bex footer. A platform system-prompt overlay tells the agent where it is, what it can use and what the deliverable looks like. The driver no longer `git add -A` + commits + pushes on the agent's behalf, and the delivery/evidence/credential-scrub machinery that only existed for that driver-owned path is deleted. **Status:** done (2026-10-30; DoD amended by user decision 2026-10-29, see t011)

## Tasks (in order)

| id   | title                                                                                       | est  | depends_on       |
| ---- | ------------------------------------------------------------------------------------------- | ---- | ---------------- |
| t001 | ADR090 agent-owned delivery + amend ADR047/051/054/059/062 and forward-point security ADRs — **DONE** | 1h   | —                |
| t002 | Platform system-prompt overlay: text + per-adapter plumbing — **DONE**                                 | 1.5h | t001             |
| t003 | Agent-side Git environment: identity, credential-free remote, hooks — **DONE**                         | 30m  | t001             |
| t004 | Gateway + bex-api `pull-request` verb (open/update draft, head pinned, footer, caps) — **DONE**        | 2h   | t001             |
| t005 | `bex` stdio MCP server in the image exposing `open_or_update_draft_pr` — **DONE**                      | 1h   | t004             |
| t006 | Driver turn end: delete driver commit/push, post-turn delivery check + one agent nudge — **DONE**      | 1h   | t002, t003, t005 |
| t007 | Completer rework: record the agent's PR, no platform-opened PR, undelivered-work reason — **DONE**     | 1h   | t004, t006       |
| t008 | Delete the evidence surface end to end (driver, Completer, store, REST/GraphQL/MCP, UI) — **DONE**     | 1.5h | t007             |
| t009 | Delete credential scrub / Git-object scan / standalone real-key mode / pre-snapshot scrub — **DONE**   | 1.5h | t006             |
| t010 | Docs + scripts + CI truth: agent-image README, backend AGENTS.md, verify scripts — **DONE**            | 45m  | t008, t009       |
| t016 | Dev-only OpenRouter model route (Claude profile → OpenRouter, DeepSeek flash) for live legs — **DONE** | 1h   | —                |
| t011 | Live E2E on `dev-10`: real turn → multi-commit branch → agent-opened draft PR with footer — **DONE**   | 1h   | t010, t016       |
| t012 | Render parity (cross-surface consistency of the changed agent-session contract) — **DONE**             | 20m  | t011             |
| t013 | Simplify — **DONE**                                                                                    | 20m  | t012             |
| t014 | Test coverage — **DONE**                                                                               | 45m  | t012             |
| t015 | Closeout — **DONE**                                                                                    | 10m  | t013, t014       |

## Definition of done

- `git grep -nE 'deliverBranch|commitMessage|scanBranchObjects|extractEvidence|secretNeedles|scrubPersistedState|snapshot/scrub' lego/agent-image lego/backend/internal/agentsession lego/backend/internal/agentsessions lego/backend/internal/sandbox` (scoped: `commitMessage` is an unrelated deploy/events field elsewhere in the backend) returns nothing; `lego/agent-image/bex-pre-snapshot` and `credential-scrub-test.sh` are gone and `.github/workflows/backend-test.yml` no longer runs the latter.
- The driver never runs `git commit` or `git push`. Its only Git writes are the setup-phase clone/checkout and repo-local identity config (t003).
- Every hosted session's `session/new` / `session/load` carries the platform overlay through the adapter's supported channel. Claude uses ACP `systemPrompt {mode:"append"}` (claude-agent-acp ≥ 0.92). Codex uses `developer_instructions`, or the ACP field once a stable codex-acp ships it. Any other adapter gets a first-prompt preamble. A driver unit test asserts each path.
- The gateway `pull-request` verb opens or updates a **draft** PR whose head is the session's bound branch and nothing else. It refuses when the branch is not ahead of base, always appends the bex footer, and records `prUrl`/`prNumber`/`headSha` on the session as soon as the PR exists. A real-Postgres backend test plus an agentcred test cover Pod binding, the head pin, size caps, idempotent update and footer presence.
- `evidence` is removed from REST, GraphQL and MCP, and the column is dropped by a down-migratable migration. Dashboard generated types are regenerated; `cd lego/backend && go test ./...`, `make lint`, driver `npm test` + `npm run typecheck`, and `dashboard/yarn test` are green.
- t011 records a live `dev-10` session (run first on the t016 OpenRouter/DeepSeek route; final acceptance also needs one run on a real Claude model, because a DeepSeek miss is ambiguous between the overlay and the model) with ≥ 1 agent-authored commit, carrying a non-template message, on `bex-agent/<id>`, plus a draft PR whose body follows the repo template and ends with the bex footer. A steering turn updates the same PR. If no authorized model key exists, t011 records the blocker and the milestone stays open; it does not close on code alone (same rule as `w10/m12` t006).

**DoD amendment (2026-10-29).** **User decision 2026-10-29:** the DeepSeek (OpenRouter platform route) production evidence is sufficient; the real-Claude-model run is waived.

## Source + Goal linkage

- **Source:** user request 2026-10-22: "bex agent 提交代码是通过程序提交的…应该改成 agent 提交", with a system-prompt overlay brief in the Devin / Claude Code on the web style. The brief covers the outer daemon wrapper, the session situation, the environment's capabilities and gaps, and a draft PR with template + footer. Follow-up "平台为什么要添乱": the driver-owned path's main justification, scanning for a real model key before push, is void since ADR062. Hosted sandboxes hold only `ModelKeyPlaceholder` (`lego/backend/internal/agentsession/model.go:90-103`). The Git proxy already pins pushes to the exact branch (`sshgateway/agentcred/agentcred.go` `validateReceivePack`), and ADR084 #17 already rejected "agent bypasses the pre-push scan" for exactly that reason. The deep-research pass (2026-10-22) covered the ACP `systemPrompt` RFD, claude-agent-acp 0.94.0 / codex-acp PR #653 source, the Claude Agent SDK prompt docs, Open SWE's `open_pull_request` tool + PR guard, Copilot/Claude-Code-web/Devin delivery docs, and the GitHub MCP server's PR tools. Its synthesis is in ADR090 (t001).
- **Goal linkage:** ADR008 AI-native pillar / ADR047 cloud coding-agent sessions. The deliverable (branch + draft PR) is the product's output, and today it is a single `bex agent: <prompt first line>` squash commit with a metadata-only PR body.
- **Expected outcome:** agent PRs read like a teammate's: reviewable commit history, a templated description of what changed and how it was verified, and attribution. Roughly 1,000+ lines of driver/backend delivery, evidence and scrub code plus a CI script go away, and the overlay gives every engine the same environment contract.
- **Why now:** `w10/m12` just moved Claude to `claude-agent-acp@0.94.0`, the first pinned adapter that honours ACP `systemPrompt` append, so the overlay has a native channel today. The agent can _already_ push mid-turn (remote = proxy URL, `:8082` egress open in both phases, mint allows active phases), so the driver's after-the-fact commit is redundant as well as low quality.
- **Render parity included (t012)** in the narrow form: agent sessions have no render.com equivalent, but this milestone removes `evidence` and makes `prUrl`/`prNumber`/`headSha` appear mid-turn on REST, GraphQL and MCP. t012 checks the three surfaces, the dashboard and mobile agree.
- **Live-model route (updated 2026-10-22):** the user authorized `OPENROUTER_API_KEY` (present in `.env`) for live turns. t016 adds a dev-only OpenRouter route through the Claude profile, which unblocks t011 and `w10/blocked/m12/t006` on DeepSeek. Final acceptance still includes one real-Claude-model run, and t011 still needs a healthy shared cluster.
