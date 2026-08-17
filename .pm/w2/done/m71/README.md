# w2 · m71 — ADR065: Devin-style agent-session archive

**Worker:** worker2 **Goal:** implement [docs/ADR065-agent-session-archive.md](../../../docs/ADR065-agent-session-archive.md) — `archived_at` as an orthogonal list-state flag with archive/unarchive/delete verbs across REST/GraphQL/MCP, replay-only attach tickets + a poll-shaped transcript read so completed sessions' conversations are viewable again, default-filtered + paginated session lists, retention-expiry auto-archive, and the dashboard Archived section — so the agent-session working set stays small while history stays complete and readable. **Status:** done (2026-08-16 — every DoD clause implemented and test-pinned: backend `go test ./...` + real-PG store/agentsessions on a fresh DB + `make lint-backend` 0 issues; dashboard 308 test files / 2124 tests + lint + typecheck green; parity record in done/t010.md, simplify record in done/t011.md, coverage map in done/t012.md)

## Tasks (in order)

| id   | title                                                                                             | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Migration + store: `archived_at`, archive/unarchive/delete ops, list filters + keyset pagination — **DONE** | 45m | —                |
| t002 | Service: archive/unarchive verbs + `AGENT_SESSION_ARCHIVED` 409 gate + idle-grace zeroing — **DONE** | 45m | t001             |
| t003 | Service: fresh-authorized hard `delete` verb (blob-before-row, audit event) — **DONE**             | 30m | t001             |
| t004 | Replay-only attach tickets: mint on terminal/hibernated, gateway empty-pod-claim replay — **DONE** | 45m | —                |
| t005 | Poll-shaped transcript read ×3 surfaces + remove dead `PruneAgentSessionTranscripts` — **DONE**    | 45m | t001             |
| t006 | List filters + pagination wired through REST/GraphQL/MCP — **DONE**                                | 45m | t001             |
| t007 | Retention-expiry auto-archive + CLAUDE.md retention-wording fix — **DONE**                         | 30m | t002             |
| t008 | Dashboard: drop the `sandboxId` conversation gate (terminal-session replay) — **DONE**             | 30m | t004             |
| t009 | Dashboard: per-row archive/unarchive, Archived section, filter/pagination controls, delete dialog — **DONE** | 60m | t002, t003, t006 |
| t010 | Render parity — surface consistency check (REST/GraphQL/MCP/dashboard) — **DONE**                  | 30m | t007, t008, t009 |
| t011 | Simplify — `/simplify` over the changed code — **DONE**                                            | 30m | t010             |
| t012 | Test coverage — behavior + failure-mode tests for archive/read/delete/pagination — **DONE**        | 45m | t010             |
| t013 | Closeout — **DONE**                                                                                | 15m | t012             |

## Definition of done

- `agent_sessions` carries `archived_at`; archive/unarchive are idempotent `can_operate` verbs on all three surfaces; while archived, `resume`/`steer`/`pin`/`unpin`/stream-`POST` refuse with the coded `AGENT_SESSION_ARCHIVED` 409 (identical shape ×3 surfaces) while `get`/list/transcript reads and `cancel` keep working; archiving a session with a live sandbox zeroes the idle grace (reclaim at next Completer tick) without canceling an in-flight turn.
- A completed session's conversation is viewable after reap: `attach-ticket` on a terminal/`hibernated` session with empty `sandbox_id` mints a replay-only ticket (empty pod claim), the gateway serves the durable-transcript replay + `[DONE]` without dialing and refuses the stream `POST`, and the dashboard renders the conversation for terminal sessions (the `session-chat-column.tsx` `sandboxId` gate is gone). `GET /v1/agent-sessions/{id}/transcript` + GraphQL `agentSessionTranscript` + MCP `get_agent_session_transcript` return the seq-paginated verbatim parts.
- The session list defaults to unarchived, filters on `archived`/`phase`/`repo`/`createdBefore`/`createdAfter`, and keyset-paginates (`limit` default 50, max 200) identically across REST/GraphQL/MCP; the dashboard polls one page of the working set, shows an Archived section, per-row archive/unarchive, and the filter controls.
- `DELETE /v1/agent-sessions/{id}` (+ GraphQL/MCP twins) hard-deletes a terminal/`hibernated` session under `AuthorizeFresh` — snapshot blob deleted before the row, transcripts cascade, audit event written; refused on live phases.
- `ExpireHibernatedAgentSession` also stamps `archived_at`; the dead `PruneAgentSessionTranscripts` is removed; CLAUDE.md's `BEX_AGENT_SNAPSHOT_RETENTION` entry says snapshot-only (row + transcript kept).
- Backend `go test ./...` + lint + dashboard suite green; every DoD clause above is pinned by a test.

## Source + Goal linkage

- **Source:** [docs/ADR065-agent-session-archive.md](../../../docs/ADR065-agent-session-archive.md) (Proposed 2026-08-16; user handoff "hand off to /pm" the same day). The ADR came from a Devin-archiving research pass (docs.devin.ai/cognition.com) + a code audit that found: no archive flag anywhere, `ListAgentSessions` unbounded/unfiltered (`store/agentsessions.go:120-136`), and — sharpest — the durable ADR051 transcript unreachable once the Completer blanks `sandbox_id` (attach mint refuses at `agentsessions/service.go:1089-1092`), so every reaped session shows no conversation. ADR047 D9a had already recorded per-row archive + the Filter control as Devin affordances "not ported, because bex has no backend capability behind them."
- **Goal linkage:** pillar 5 (cloud coding-agent sessions, ADR008/ADR047) — session end-of-life organization + making the ADR051 conversation deliverable actually readable; completes the ADR047 D9 "optional JSON transcript read" parity item.
- **Expected outcome:** the agents working set shows only unarchived sessions (one page, filtered server-side); any completed session's conversation replays in the dashboard and is readable via REST/GraphQL/MCP; tenants can archive, unarchive, and hard-delete sessions; expired hibernations leave the working set by themselves.
- **Why now:** sessions accumulate forever with no way to put one away, the full-history list is polled every 5–30s and grows without bound, and the transcript-unreachability defect silently voids ADR051's shipped value — every day of fire-and-forget usage widens the backlog of unviewable history that D2's replay ticket will retroactively fix.
- **Render parity included:** the milestone changes REST/GraphQL/MCP and the dashboard. Note Render has no agent-session product — parity here means bex's own three-surface consistency contract (identical fields/semantics/error shapes per ADR006), with Devin as the UX exemplar, not render.com.

## Out of scope (per ADR065 D6/D7 + Non-goals)

- Bulk archive-all/undo, folders, tags, server-side full-text transcript search, knowledge extraction, AI session insights, per-session cost on the View (blocked on ADR047 D6 phase 2), auto-archive on idle/age, any phase-machine change, any snapshot/hibernation lifecycle change beyond D4's delete.
