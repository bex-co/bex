# ADR065 — Agent-session archive: Devin-style end-of-life organization

**Status:** Proposed (2026-08-16); implemented in **w2/m71** the same day. (Renamed from ADR065 to resolve the number collision with ADR065-security-review-round10.) Refines [ADR047](ADR047-cloud-coding-agent-sessions.md) (D9a explicitly recorded per-row **archive** and the **Filter** control as Devin affordances "not ported, because bex has no backend capability behind them — gaps, not drift"); composes with [ADR051](ADR051-agent-session-transcript.md) (the durable transcript this ADR finally makes readable after reap) and [ADR059](ADR059-agent-sandbox-hibernation.md) (archive is orthogonal to the Active/Hibernated pod-lifecycle tiers — it organizes the _list_, hibernation organizes the _pod_). No security-lineage findings; the one new destructive sink (D4 delete) lands inside the ADR061/ADR063 fresh-authorization seam from day one.

---

## Context

### The product need

Agent sessions accumulate forever and there is no way to put one away. `agent_sessions` rows are never purged (the only deletes are create-path compensation and workspace-delete cascade, `store/agentsessions.go:615-627`), every surface returns the **whole** workspace history — `ListAgentSessions` is `SELECT … ORDER BY created_at DESC` with **no LIMIT, no cursor, no filter** (`store/agentsessions.go:120-136`; REST/GraphQL take only `ownerId`) — and the dashboard polls that full list every 5–30s. The sidebar working set (ADR047 D9a's Devin-shaped rail) mixes months-old finished sessions with today's live ones, filtered only by a client-side fuzzy match over already-fetched title+repo. D9a deliberately deferred the fix to a backend capability; this ADR is that capability.

### The sharper defect this ADR must also fix: the archive would be empty

Devin's core archive guarantee is "archived sessions can still be viewed." bex cannot honor it today: the ADR051 transcript is durable in `agent_session_transcripts`, but its **only** tenant-facing read path is the gateway stream replay, and the attach-ticket mint refuses when `sandbox_id` is empty (`AGENT_SESSION_NOT_ATTACHABLE`, `agentsessions/service.go:1089-1092`) — which the Completer guarantees within the idle grace (default 30m, `BEX_AGENT_SANDBOX_IDLE_TTL`): terminate **and** hibernate both blank `sandbox_id` (`completion.go:452-457`, `:501-504`). The dashboard confirms it: `session-chat-column.tsx:77-97` renders the conversation only when `sandboxId` is set, so every reaped session shows the fallback — the ADR051 rows sit in Postgres with no reader. An archive of unviewable sessions is a trash can, not an archive; D2 closes this first.

### What Devin verifiably does (survey, 2026-08-16; docs.devin.ai / cognition.com)

- **Archive is its own verb, distinct from both sleep and terminate.** `POST …/sessions/{id}/archive` "archive[s the] session and put[s] it to sleep if currently running"; archived sessions "can still be viewed" but "cannot be modified or resumed" until unarchived ([archive endpoint](https://docs.devin.ai/api-reference/v3/sessions/post-organizations-sessions-archive)). Terminate is the destructive verb — `DELETE …/sessions/{id}`, non-resumable, with an optional `archive=true` to preserve the record while ending it ([delete endpoint](https://docs.devin.ai/api-reference/v3/sessions/delete-organizations-sessions)).
- **Archive is manual and reversible; sleep is the automatic thing.** Idle sessions auto-**sleep** (~0.1 ACU of inactivity; "Devin does not consume any usage while sleeping" — [billing/usage](https://docs.devin.ai/admin/billing/usage)); nothing auto-archives. Unarchive exists everywhere archive does (command palette, "Unarchive on Mention" in Slack), archive-all ships with a confirmation dialog **and undo**, and archiving cascades to child sessions with undo restoring them ([release notes](https://docs.devin.ai/release-notes)).
- **The list is filtered by default and archived sessions have a home.** The sidebar defaults to "non-archived sessions you started"; archived sessions live under **Folder → Archived**; the insights API filters on `is_archived`, `status`, date ranges, repos, users, and the v1 list API paginates (`limit`/`offset`) and filters by `tags` ([insights](https://docs.devin.ai/api-reference/v3/sessions/organizations-sessions-insights), [v1 list](https://docs.devin.ai/api-reference/v1/sessions/list-sessions)).
- **What survives archive:** the full session record — status, timestamps, ACU consumption, PR info, and the viewable conversation/worklog. Retention is relationship-scoped ("Cognition only retains data … for the duration of the relationship with a given Customer"), not a per-session TTL ([security](https://docs.devin.ai/admin/security)).
- **Knowledge extraction is a separate, human-approved product** (suggested Knowledge from sessions, edit-before-save, trigger descriptions — [Knowledge](https://docs.devin.ai/product-guides/knowledge)) — an archive consumer, not part of the archive mechanism.

The mapping onto bex is clean because ADR059 already built Devin's other two states: bex `hibernated` ≈ Devin sleep (pod reclaimed, state snapshot kept, resumable), bex Cancel ≈ Devin terminate-preserving (row kept). What's missing is exactly the third, orthogonal axis: **in/out of the working set**.

### What bex has today (from the code, 2026-08-16)

- **Phases** (10, DB-CHECK-enforced, migration 0073): `creating running resuming redispatching completed failed canceling canceled hibernating hibernated` (`agentsessions/models.go:25-38`). No `archived` — and it should not become one (D1).
- **The row keeps everything except the conversation's reachability**: `head_sha`/`pr_url`/`pr_number`/bounded `evidence`/`turns`/`failure_reason` + the m68 snapshot fields. The diff itself lives only in GitHub.
- **Transcripts are effectively forever**: `PruneAgentSessionTranscripts` exists but is wired into no sweep — dead code called only by its own test (`store/agentsessions.go:602-613`, `store_pg_test.go:292-295`); the real bounds are the 64 MiB/session cap and session-row cascade.
- **End-of-life today**: Cancel → `canceled` (row/transcript/PR pointers kept; snapshot blob best-effort deleted); hibernation-retention expiry → `canceled` with `failure_reason='hibernation retention window elapsed'` (`ExpireHibernatedAgentSession`, `store/agentsessions.go:400-418` — snapshot deleted, row + transcript kept; note the doc drift: CLAUDE.md's `BEX_AGENT_SNAPSHOT_RETENTION` entry says "snapshot **+ row**", the code keeps the row). Expiry being indistinguishable from user cancel by phase is a known wart this ADR partially absorbs (D5).
- **Hibernated tier is env-gated OFF in prod** (m68; `BEX_AGENT_SNAPSHOT_S3_*` unprovisioned) — today every finished session converges to a terminal row with empty `sandbox_id`, which is exactly the population an archive organizes.

---

## Decision

### D1 — Archive is an orthogonal flag, never a phase

Add `archived_at timestamptz NULL` to `agent_sessions` (null = active in the working set). Archive is **list-state, not lifecycle-state**: the 10-phase machine is untouched, the Completer/idle-grace/hibernation/retention loops read and write phases exactly as before, and a session is `(phase, archived)` — mirroring Devin, where any status can be archived. Rejected alternative: an `archived` phase — it would fork every phase consumer (Completer active-set, push projection, dashboard chips) and destroy the phase's meaning as "where the work is," when archive means "I'm done looking at it."

Semantics (Devin-faithful, adapted to bex's fire-and-forget model):

- **`archive`** — allowed in any phase, idempotent. On a session with a live sandbox it additionally **zeroes the idle grace**: archive is an explicit disinterest signal, so the Completer reclaims (hibernate-or-terminate per m68 config) at its next tick instead of waiting out `BEX_AGENT_SANDBOX_IDLE_TTL`. It does **not** cancel an in-flight turn (bex's analog of Devin's "put it to sleep if currently running" is "stop keeping the sandbox warm," not "kill the work") — the turn finishes, the Completer delivers the PR as usual, then reclaims immediately.
- **While archived, mutation verbs refuse**: `resume`, `steer`, `pin`/`unpin`, and the live-turn stream `POST` return the coded `AGENT_SESSION_ARCHIVED` (409, identical across REST/GraphQL/MCP), matching Devin's "cannot be modified or resumed." `cancel` stays allowed (safety verb). Read verbs — `get`, list (with the filter, D3), and the transcript reads (D2) — always work: viewable is the point.
- **`unarchive`** — clears the flag, idempotent; the session rejoins the working set and the mutation verbs work again (Devin's unarchive-then-resume reading). Unarchive does not rehydrate — the next `resume`/`steer` does, through the unchanged ADR059 path.
- **Snapshot lifecycle is unaffected**: an archived `hibernated` session's `retain_until` clock keeps running and the retention sweep treats it identically (pinned still exempts). Pin and archive compose — a pinned+archived session keeps its snapshot forever but stays out of the sidebar.
- **Authorization**: archive/unarchive ride `can_operate` like cancel — reversible, non-destructive, no freshness requirement.
- No cascade semantics: bex has no child sessions (Devin's cascade/undo-cascade is n/a; noted so it isn't mistaken for a gap).

### D2 — Make "archived sessions can still be viewed" true: two transcript read paths

The archive is only as good as its reads, and today's sole read dies with `sandbox_id`. Two changes, both serving live and archived sessions alike:

1. **Replay-only attach tickets.** `attach-ticket` on a session in a terminal or `hibernated` phase with empty `sandbox_id` mints a ticket with an **empty pod claim** instead of `AGENT_SESSION_NOT_ATTACHABLE`; the gateway attach listener, on an empty pod claim, serves the durable-transcript replay + `[DONE]` and **never dials** (it already does exactly this for a gone pod — the change is admitting the ticket, not new gateway behavior). The stream `POST` on such a ticket refuses. The dashboard drops its `sandboxId` gate (`session-chat-column.tsx:77`) and every completed session's conversation renders through the existing `useChat` replay with zero other client change — the same "the fix is mostly wiring" shape as ADR051. `AGENT_SESSION_NOT_ATTACHABLE` narrows to the genuinely unattachable window (provisioning, pre-dispatch).
2. **A poll-shaped transcript read for REST/MCP parity** — the read ADR047 D9 marked "optional" and never built: `GET /v1/agent-sessions/{id}/transcript` (paginated by `seq`, parts verbatim), GraphQL `agentSessionTranscript`, MCP `get_agent_session_transcript`. Non-streaming consumers (CLI scripts, CI, agents summarizing past sessions) get the history without SSE plumbing; the 64 MiB cap bounds the read. **w5/m71 amendment (2026-08-17):** the same response also carries ordered durable turns (prompt, delivery mode, timestamps, complete/truncated/reason), and transcript parts expose `partIndex`. A history reader can reconstruct user intent and distinguish a complete assistant record from a retained prefix after the sandbox is gone.

With these, **transcripts are product data, not audit data** — decided explicitly: they live as long as the session row (cascade-only deletion, like Devin's relationship-scoped retention), the dead `PruneAgentSessionTranscripts` is removed rather than wired (its `BEX_AUDIT_RETENTION_DAYS`-lineage index comment was a category error), and per-workspace transcript-storage bounds are future work if the 64 MiB/session cap proves insufficient.

### D3 — The list grows up: default filter, real filters, pagination

`ListAgentSessions` gains a filter+page contract, identical across REST (`GET /v1/agent-sessions`), GraphQL, and MCP:

- **`archived`** — `false` (default: the working set, Devin's sidebar default), `true` (the Archived folder), `all`.
- **`phase`** (repeatable), **`repo`**, **`createdBefore`/`createdAfter`** — the D9a "Filter" control's backend.
- **Keyset pagination**: `limit` (default 50, max 200) + an opaque cursor over the existing `(created_at DESC, id DESC)` order. The unpaginated full-history read disappears; the dashboard's 5s/30s poll fetches one page of the working set instead of the whole workspace history.

### D4 — Delete: the missing destructive verb, fresh-authorized

Devin separates archive (preserve) from terminate (destroy); bex's Cancel deliberately preserves and nothing destroys. Add **`delete`** — REST `DELETE /v1/agent-sessions/{id}`, GraphQL `deleteAgentSession`, MCP `delete_agent_session`: allowed only on sessions with no live sandbox (terminal or `hibernated`; live phases must cancel first — no `archive=true` escape hatch on a destructive verb), hard-deletes the row (transcripts cascade), and deletes the snapshot blob first when present, mirroring the expiry sweep's blob-before-row order. This is a **destructive sink**: it requires `AuthorizeFresh` per the ADR061 #8 / ADR063 #7 seam — never the cached check — and writes an audit event. It also gives tenants per-session erasure short of workspace deletion.

### D5 — Retention expiry auto-archives

The one automatic archive edge: `ExpireHibernatedAgentSession` additionally stamps `archived_at`. A session whose snapshot aged out is definitionally one nobody came back for — it should leave the working set by itself, and this softens the standing wart that expiry reuses `canceled` (an expired session is now distinguishable in practice: `canceled` + archived + the named `failure_reason`). A distinct `expired` phase remains rejected — not worth an 11th CHECK value and a phase-consumer sweep when `failure_reason` + `archived_at` carry the signal. Everything else stays manual, matching Devin (no idle-based auto-archive; idle-based **reclaim** is ADR059's job and already exists).

### D6 — Dashboard surface

- **Per-row archive** (list + session header) and unarchive; archived sessions move to an **Archived** section reachable from the sidebar (Devin's Folder → Archived), excluded from the working-set rail and its fuzzy search.
- The `/agents?view=list` table gains the D3 filter controls (archived/phase/repo/date) and pagination.
- Delete lives behind a confirmation dialog on the session detail page only (destructive verbs stay off the list rows), desktop-only per ADR048's mobile rule.
- Deferred, recorded: bulk archive-all + undo (Devin has it; bex ships single-row first — undo is just `unarchive`, so a later bulk verb is additive), folders and drag-and-drop organization, tags.

### D7 — Deliberately not ported

- **Knowledge extraction** (Devin's suggested-Knowledge loop) and **AI session insights/analysis** — archive consumers, not archive mechanism; each is its own future ADR. This ADR's D2 reads are their prerequisite (a summarizer needs a transcript API).
- **Server-side full-text search** over transcripts — the D3 filters cover the list; search waits for real demand.
- **Per-session cost on the View** — blocked on ADR047 D6 phase 2 (`agent_token_units`) and a session-keyed projection of `sandbox_compute_seconds`; noted as the analog of Devin's per-session ACU display.

---

## Consequences

- The working set stays small and the history stays complete: rows are still never age-purged (Devin-like relationship-scoped retention), but the default read is one page of unarchived sessions instead of an unbounded scan, and D4 gives tenants the explicit way out.
- Every completed session's conversation becomes viewable again — including the backlog already sitting unreadable in `agent_session_transcripts` — with zero migration beyond the `archived_at` column: D2's replay ticket reads the rows ADR051 already wrote.
- New surface: `archive`/`unarchive`/`delete` verbs ×3 surfaces, the transcript read ×3 surfaces, list filters+pagination ×3 surfaces, `AGENT_SESSION_ARCHIVED` 409, one migration, dashboard Archived section + filter controls. No new env vars, no new processes, no gateway listener changes (the replay path exists).
- The list contract changes shape (pagination): the dashboard and any API consumers of the previously-unbounded list migrate in the same milestone.
- CLAUDE.md's `BEX_AGENT_SNAPSHOT_RETENTION` description ("snapshot + row") is corrected to match the code (snapshot only; row and transcript kept) as part of implementation.

## Non-goals

- **Auto-archive on idle or age** — reclaim-on-idle is ADR059; archive stays a human verb plus the single D5 expiry edge.
- **An `archived` phase** or any change to the 10-phase machine.
- **Changing snapshot/hibernation lifecycle** — archive never extends, shortens, or deletes a snapshot (only D4 delete does, as part of destroying the session).
- **Knowledge base, insights, search, folders, tags, bulk verbs** — deferred per D6/D7.
- **Workspace-level data-retention policy controls** (Devin's enterprise Data Controls analog) — future work alongside billing/compliance.
