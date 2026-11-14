# w22 · m408 — The message queue and Stop turn are hardened: keyed sends never run twice, an interrupt needs a request, a reorder is safe, a blocked message backs off

**Worker:** worker22 **Goal:** the four risks the w22/m403 review found by reading are closed, each with a test that fails on the old behavior. **Status:** done (2026-11-14)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | A retried send that starts a turn never starts two: the idempotency key is honored for every keyed send — **DONE** | 60m | — |
| t002 | A turn is finalized `interrupted` only if bex-api recorded a stop request for it — **DONE** | 45m | — |
| t003 | A reorder never trades places with the message being started, and an edit after a lapsed lease is not lost — **DONE** | 45m | — |
| t004 | A blocked message does not run `Steer` more than about twice a minute — **DONE** | 30m | — |
| t005 | Render parity (Devin) — **DONE** | 20m | t001, t002, t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout — **DONE** | 10m | t006, t007 |

## Definition of done

- A keyed send retried after a lost reply returns the earlier outcome whether the first attempt started a turn, queued, or was already dispatched; it never starts two turns or queues a duplicate; the same key with another message is `AGENT_SESSION_IDEMPOTENCY_KEY_REUSED`.
- A turn is finalized `interrupted` only when bex-api recorded a stop request for that turn; a status `interrupted` without one takes the normal completion path, so a turn that owes a structured result still fails without it.
- A reorder never trades places with a message the dispatcher holds a lease on (a coded 409 `AGENT_QUEUED_MESSAGE_DISPATCHED`), and the turn runs the latest saved text of a message that was edited after its lease lapsed.
- A message blocked by the spend ceiling or the live-sandbox cap is retried at most about twice a minute; unblocked messages are delivered as before.
- Every change has a test that fails on the old code, against real Postgres, and ADR047 D12 "As built" records it.

## Source + Goal linkage

- **Source:** inbox note w22/014 (promoted), found by the w22/m403 review on 2026-11-14 by reading `lego/backend/internal/store/agentsession_queue.go`, `lego/backend/internal/agentsessions/{service.go,queue.go,message_queue.go,completion.go}`. The design is [ADR047 D12](../../../../docs/ADR047-cloud-coding-agent-sessions.md) and its "As built" notes.
- **Goal linkage:** ADR008's AI-native thesis and pillar 5 (agent sessions); the reference product is Devin ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)), whose queued messages survive idle and resume and can be edited, removed and reordered ([release notes](https://docs.devin.ai/release-notes/2026), 2026-09-09 and 2026-10-16).
- **Expected outcome:** the queue's promises hold under retries, lease lapses and a hostile status file, not only on the happy path.
- **Why now:** w22/m403 shipped the queue on 2026-11-14 and these are its known gaps; the longer the queue is in use, the more retried sends and lapsed leases it meets.
- **Render parity included (reference product: Devin):** the key behavior is visible on REST, GraphQL, MCP and the CLI, so t005 compares each surface and Devin.

## Outcome (2026-11-14)

- **Shipped:** a keyed send is now idempotent in every state: `agent_session_turns` records the key and request hash of a turn a direct send starts (migration 0268, written in the same transaction as the turn row), `Service.Steer` looks the key up before the queue-or-start decision (turns table, then queued messages), and a retry returns the earlier outcome (200 for a turn it started, 202 for a message it queued with its dispatched or dropped state); the same key with another message is `AGENT_SESSION_IDEMPOTENCY_KEY_REUSED`. `Service.Interrupt` stamps `interrupt_requested_at` before it writes the sentinel, and the Completer honors the sandbox's `interrupted` status only for a stamped current turn, otherwise the normal completion path (and its structured-result check) runs. A move that would give a leased message another place is refused with `AGENT_QUEUED_MESSAGE_DISPATCHED`, and the begin-turn transaction refuses a message edited since its claim so the dispatcher re-claims it. A head blocked by the spend ceiling or the live-sandbox cap is claimed again only after 30 s (`queueBlockedBackoff`).
- **Choice made:** a message edited between claim and begin-turn is refused and re-claimed, not re-read, because `Steer` resolves and authorizes the model key, the attachment binding and `can_create` for the claimed author, which a re-read of the prompt alone would not redo.
- **Checks:** the real-dependency backend suite passed in all 78 packages (14,872 tests) before the review fixes, and the agentsessions, store, api, core and cmd packages pass against real Postgres after them; vet, gofmt, golangci-lint and deadcode are clean. 11 mutations (drop the key lookup, drop the turn-table key write, drop the stamp check, stamp after the sentinel, never stamp, match the stamp on any turn, let a leased head move, drop the begin-turn re-read, drop the backoff, make enqueue ignore a turn-held key, skip the in-transaction key check) were each caught by a test, with none left in the tree.
- **Review:** the migration's CHECK constraints are `NOT VALID` so it does not scan `agent_session_turns` under an exclusive lock; the current-turn lookup in the Completer and the key-used replay are shared helpers.
- **Not done, left as follow-ups by the review:** turning the 12-argument `BeginQueuedAgentSessionTurn`/`BeginKeyed…` into a `TurnStart` struct (about 15 test files), a single-column read for the stop request, and one query for the two key lookups.
- ADR047 D12 "As built" and ADR018's item for w22/m403 record each change.
