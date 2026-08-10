# w5 · m65 — Agent sessions: retire the Evidence digest + make draft-PR delivery opt-in

**Worker:** worker5 **Goal:** the agent-session detail experience is the conversation itself — no lossy "Evidence" digest anywhere (web, mobile, PR body), and a session opens a draft PR only when the user explicitly asked for one at create time. **Status:** done (2026-08-09)

## Tasks (in order)

| id   | title                                                                        | est | depends_on   |
| ---- | ---------------------------------------------------------------------------- | --- | ------------ |
| t001 | bex-api: opt-in draft-PR delivery (`openPr`, default off) across REST/GraphQL/MCP + Completer | 45m | — — **DONE** |
| t002 | bex-api: strip the evidence sections from the draft-PR body | 20m | t001 — **DONE** |
| t003 | dashboard: remove the Evidence side panel + inline PrCard | 40m | — — **DONE** |
| t004 | dashboard: "Open a draft PR" opt-in control in the new-session composer | 30m | t001 — **DONE** |
| t005 | mobile: remove the Evidence section from the session detail screen | 30m | — — **DONE** |
| t006 | Render parity (cross-surface consistency: REST/GraphQL/MCP/web/mobile) | 30m | t002, t004, t005 — **DONE** |
| t007 | Simplify (`/simplify` over the changed code) | 30m | t006 — **DONE** |
| t008 | Test coverage for the shipped behavior | 45m | t006 — **DONE** |
| t009 | Closeout | 15m | t007, t008 — **DONE** |

## Definition of done

- Creating an agent session without asking for a PR results in a completed session that pushed its `bex-agent/*` branch but opened **no** pull request (verified against a live or stubbed completion); creating one with the explicit opt-in still yields the draft PR.
- When a PR is opened, its body contains session id / branch / head / API link only — no "Changed files", "Commands run", "Test output (tail)", or truncation-note sections.
- The web session detail page renders no Evidence panel, no header Evidence toggle, and no inline PrCard; the header's `#N` draft-PR badge remains the sole PR affordance for opted-in sessions. The mobile detail screen renders no Evidence section.
- The GraphQL/REST **wire** `evidence` field still exists and serves data (API consumers unaffected); only the UI consumers dropped it.
- `go test ./...` (backend), dashboard `yarn typecheck && yarn lint && yarn test`, and the mobile test suite all pass.

## Source + Goal linkage

- **Source:** user directive 2026-08-09, from a live walk of `https://dashboard.bex.co/agents/ags-d9sle9558lbc738s7220`: the Evidence panel shows "0 commits" + misclassified tool output (grep "No matches found" rendered as "Test output") on every session regardless of outcome. Root cause analysis in-session: the ADR047 D4 evidence digest is a phase-1 fire-and-forget artifact — the driver's `extractEvidence` (`lego/agent-image/driver/src/delivery.ts:172`) classifies any `output`/`stdout`/`stderr` string as test output — and the ADR051 durable transcript now carries the full narrative, making the digest a lossy duplicate. Exact user scope: "前端删 Evidence 面板(web + 可选 mobile),wire 字段保留,PR body 里的 evidence 段一并清理,PrCard 也删除,默认不创建 pr,除非用户如此要求".
- **Goal linkage:** pillar 5 (cloud coding-agent sessions, `docs/ADR047-cloud-coding-agent-sessions.md`) product quality; ADR051 made the conversation the product — this retires the superseded digest surface and stops unrequested PR spam on connected repos.
- **Expected outcome:** session detail = chat only on web and mobile; clean PR bodies; no PR opened unless the user asked. Wire compatibility preserved for API/mobile consumers.
- **Why now:** every completed session currently shows misleading garbage to the user, and every session opens a draft PR on the tenant's repo whether or not they wanted one — both are live, user-reported product defects on the shipped surface.
- **Render parity note:** included as cross-surface consistency (the new `openPr` option must land identically on REST/GraphQL/MCP, and the Evidence removal consistently on web + mobile). Render has no cloud coding-agent product — there is no render.com behavior to compare against; agent sessions are a bex extension (ADR047).
- **Deliberately out of scope:** removing the backend evidence harvest/storage (the wire field stays per user direction); any replacement PR affordance in the chat (the header `#N` badge suffices — file a follow-up note if a richer one is wanted); fixing the `extractEvidence` test-output misclassification (moot for the UI once nothing renders it).

## Outcome (2026-08-09)

Shipped and verified against every DoD item.

- **Draft-PR delivery is opt-in.** `agentConfig.openPr` (default `false`) is accepted identically on REST, GraphQL, and MCP and persisted with the config, so the choice survives every later steer turn (`Steer` decodes the stored blob and never rewrites it). The Completer skips the git-connection lookup and `OpenDraftPullRequest` entirely when a session did not opt in, finalizing `completed` with its `headSha` — the pushed `bex-agent/*` branch is the unconditional delivery. An **unreadable config fails safe to opted-out**: an unwanted PR cannot be un-opened, a branch can always have one opened from it. Verified the driver never opens PRs itself, so the Completer gate is the complete gate.
- **The evidence digest is gone as a presentation surface** — web panel + header toggle + `evidence-panel.tsx`, the inline `pr-card.tsx`, the mobile Evidence card, and the PR body's Changed-files/Commands-run/Test-output/truncation sections. The **wire field is retained and still populated** (guarded by `TestEvidenceRemainsOnTheWire`); only the renderers went. The header's `#N` badge is now the session's sole PR affordance.
- **Cross-surface proof rides the repo's own harness:** `TestRESTGraphQLMCPCreateParity` now sets `OpenPR: true` and compares the whole `AgentConfig` by `==`, so a surface that dropped the field fails. `TestOpenPRDefaultsToOptedOutOnEverySurface` covers the safety-critical direction (absent ⇒ opted out) on all three.

**`/simplify` (t007) found five real defects, all fixed:**

1. `scripts/agent-session-verify.sh` — the live E2E script ADR047 cites would have **failed on every run**: leg 1 created a default (now opted-out) session then hard-asserted `prUrl`. Leg 1 now passes `openPr=true`, and a new **leg 1b** verifies the new default end to end (completed + `headSha` + empty `prUrl`).
2. A seventh `AgentSessionView` fixture in `dashboard/src/common/.../agent-sessions-nav-section.test.tsx` was missed by the sweep (still set `evidence`, lacked `openPr`) and **no gate caught it** — test files are excluded from `tsc -b`. Fixed, and all seven copies collapsed onto `src/test/mocks/agent-session.ts`. Root cause filed as `041`.
3. The `steer_agent_session` **MCP tool description** still promised it "updates the same draft PR" — false by default, and it is what an agent client reads to decide what the verb does. Corrected, plus two matching `service.go` comments.
4. The persisted config was decoded twice per completion with two divergent error policies; now decoded once, retiring both `openPRRequested` and `taskOf`.
5. Dead mobile `StyleSheet` keys and a new private `boolArg` GraphQL shadow, both removed; `mobile/.../detail/evidence.ts` renamed to `github-links.ts` (its remaining job is the GitHub URL guard).

**Gates:** backend `go test ./...` + `make lint-backend` (0 issues); dashboard typecheck + lint + **2001 tests / 298 files** + production build; mobile typecheck + lint + format + **316 tests** + `expo:check` + both bundles. The 9 `make lint` findings in the **operator** module are pre-existing (all in files this milestone never touched) and were left alone.

**Docs:** ADR047 D4 revised (opt-in delivery + why the digest was retired), ADR018's stale "opens a draft PR with the evidence digest" claim corrected, and the ADR047 D9a / `dashboard/CLAUDE.md` "second panel goes right" rule kept while noting its reference implementation is gone.

**Follow-ups filed:** `041` (test files escape typecheck — the hole that let the fixture rot), `042` (mobile composer has no PR opt-in; deliberate per ADR048, recorded rather than left implicit).

**Not verified live.** Everything above is proven by the automated suites and a stubbed completion, which the DoD allows. The `scripts/agent-session-verify.sh` legs — including the new 1b — need a real cluster + GitHub App and have not been run.
