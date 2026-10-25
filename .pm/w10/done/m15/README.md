# w10 · m15 — Retire agent-session hibernation: close idle sandboxes after 2h, keep the conversation

**Worker:** worker10 **Goal:** An agent session's sandbox is terminated after 2 hours idle. There is no snapshot, no `hibernated`/`resuming` phase, no Pin tier and no restore. The durable transcript stays, and the next message continues the conversation in a fresh sandbox on the same branch, primed with the prior turns (ADR047 D3 continuity). Agent-session hibernation code, config, API fields and UI are deleted. App/database hibernation (Render free-tier sleep) is untouched. **Status:** done (2026-10-25: shipped `bd6423e5e` + `773aee4b0`, verified in production — see done/t004.md)

## Tasks (in order)

| id   | title                                                                                         | est  | depends_on |
| ---- | --------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Backend + driver: idle 2h → terminate; delete snapshot/rehydrate/pin; migrate legacy rows — **DONE**     | 3h   | —          |
| t002 | API surfaces: drop hibernation fields/verbs from REST/GraphQL/MCP; dashboard + mobile UI — **DONE**     | 2h   | t001       |
| t003 | Config, manifests, scripts, docs (ADR059 superseded, ADR090/047 notes, env tables) — **DONE**            | 45m  | t001       |
| t004 | Production rollout + verify (idle close at 2h, transcript kept, follow-up continues, legacy rows) — **DONE** | 1h | t002, t003 |
| t005 | Render parity (agent-session surface consistency) — **DONE**                                             | 20m  | t004       |
| t006 | Simplify — **DONE**                                                                                      | 20m  | t005       |
| t007 | Test coverage — **DONE**                                                                                 | 30m  | t005       |
| t008 | Closeout — **DONE**                                                                                      | 10m  | t006, t007 |

## Definition of done

- `BEX_AGENT_SANDBOX_IDLE_TTL` defaults to 2h, and production runs it. An idle session's sandbox is terminated; its `agent_session_transcripts` rows remain and the dashboard renders them.
- Sending a message to a closed session redispatches a fresh sandbox on the same branch, with the prior conversation as continuity context (real-DB test + production check).
- No agent-session code path snapshots, restores, pins or retains: `git grep -niE 'rehydrate|SnapshotRef|retainUntil|hibernat' -- lego/backend/internal/agentsessions lego/agent-image/driver/src dashboard/src/features/agent-sessions mobile/src/features/agent-sessions` is empty.
- Legacy `hibernated`/`resuming` rows are migrated to the closed phase and are steerable, including the production session `ags-dbdh96fsf6rc738q771g`.
- Backend real-DB suite, `make lint`, driver, dashboard and mobile suites are green.

## Source + Goal linkage

- **Source:** user decision 2026-10-25: "hibernated 的体验好像不太好，能够改成没有 hibernate 然后两小时之后强制关闭吗？" followed by "go implement, ship, and verify in production". The production investigation found the problems on the rehydrate path:
  - slow restore;
  - a mint race where the driver's first model call 403s before the sandbox id is recorded;
  - a bex-api restart orphaning a `resuming` row (`ags-dbdh96fsf6rc738q771g`).
- **Goal linkage:** ADR047/ADR090 agent sessions (ADR008 AI-native pillar).
- **Expected outcome:** follow-ups on old sessions always take the proven fresh-dispatch path, with no stuck `resuming` sessions and no snapshot storage.
- **Why now:** it blocks the user's production session, and hibernation is the only path with the race and orphan bugs.
- **Render parity included (t005):** agent-session fields change on REST/GraphQL/MCP/dashboard/mobile. There is no render.com equivalent, so this is a cross-surface consistency check.
