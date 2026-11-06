# w9 · m172 — Cloud Agents MCP phase 4: completion webhooks, per-session spend ceiling, `outcome.cost`

**Worker:** worker9 **Goal:** Orchestrators are pushed completion events instead of polling and can cap spend per task, with cost visible on `outcome`. **Status:** todo — t001–t006 done (shipped in w21); t007 closeout after the live checks in w9/077 (and w9/076 for m259)

## Tasks (in order)

| id | title | est | depends_on |
| -- | ----- | --- | ---------- |
| t001 | Completion webhook events: turn succeeded / failed / needs input — **DONE** | 1.5h | w21/m257/t003 |
| t002 | Per-session `maxSpendUSD` ceiling in the model proxy — **DONE** | 2h | w10/m14/t004, w10/m14/t005 |
| t003 | `outcome.cost`: decimal-string model USD and sandbox seconds — **DONE** (modelUSD only; sandboxSeconds is w9/078) | 1h | w10/m14/t004, w21/m257/t003 |
| t004 | Render parity (vs Devin): REST, GraphQL, MCP and dashboard consistency — **DONE** | 30m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

Workspace webhooks deliver the three agent-session event types with `{id,state,outcome}`; `maxSpendUSD` is enforced in the model proxy (and rejected with the named 400 until the meter exists); `outcome.cost` matches the ADR091 meter. Verified by backend tests and a live run.

## Source + Goal linkage

- **Source:** [ADR094](../../../docs/ADR094-coding-agent-mcp.md) D8, D9 (completion push), D10 phase 4.
- **Goal linkage:** Pillar: AI-native coding agents with priced, capped usage (ADR091).
- **Expected outcome:** Callers react to events and bound per-task spend.
- **Why now:** t001 is independent; t002/t003 are gated on `w10/m14` t004–t005 (cost meter, caps), so this milestone is last in the ADR094 sequence.
- **Render parity:** included as a closing task, checked against Devin (the reference product for cloud coding-agent surfaces, ADR047), because this milestone changes REST/GraphQL/MCP/dashboard-facing behavior.

## Completion record (partial)

- Shipped 2026-11-05: migrations 0201 (`agent_sessions.max_spend_microusd`) and 0202 (`agent_session_turn_events` + phase trigger); `maxSpendUSD` on create across REST, GraphQL and MCP, enforced in the model mint (lower of session ceiling and ADR091 monthly cap) with `AGENT_MODEL_SPEND_CAP_REACHED` `scope: "session"` and the named 400s `AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE`/`_INVALID`; `outcome.cost.modelUSD`; the webhook events `agent_session_turn_succeeded`/`_failed` (and the advertised, never-emitted `agent_session_needs_input`) with `{id, state, outcome}` and no summary. Recorded as "As built" in ADR094 D7, D8, D9.
- Deviations: `outcome.cost.sandboxSeconds` is not shipped (no per-session sandbox attribution; follow-up `w9/078`); the webhook emission is a DB trigger rather than a hook in `agentsessions/completion.go`.
- Why t007 is open: the definition of done asks for a live run, and `scripts/dev-env.sh` takes N 1-10 (no `dev-21`). Verified instead by Go tests against real Postgres + OpenFGA (`scripts/backend-test-deps.sh`; the promtool-based egressquery/metrics/usage packages fail on macOS for an unrelated docker-mount reason), `make lint-backend` clean, and dashboard vitest for the webhook picker. The live run is `w9/077`; dashboard follow-up is `w21/005`.

## Moved from w21/m260 to w9/m172 (2026-11-05)

- **Source.** User decision 2026-11-05: move every open w21 item to w9 and follow up there. History, IDs in text and links are rewritten (`w21/m260` → `w9/m172`; notes `004`/`006`/`007` → `w9/076`/`078`/`077`).
- **Gate re-evaluated (review conclusion).** The live run no longer needs a dev-21 stack. Like the ADR095 acceptance (w9/m168–m171), it runs on production as the QA user, signing in through `scripts/qa-login.sh`. Agent sessions run in the `bex` workspace, which has `ready=true` and a model key. The webhook receiver is a short-lived free web service the worker deploys, then deletes and verifies absent.
- **Next.** w9/077 runs the live completion-webhook and spend-ceiling checks. Then t007 closes this milestone. `outcome.cost.sandboxSeconds` stays out of scope (w9/078, user decision pending).
