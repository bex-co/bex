# w22 · m408 — The message queue and Stop turn are hardened: keyed sends never run twice, an interrupt needs a request, a reorder is safe, a blocked message backs off

**Worker:** worker22 **Goal:** the four risks the w22/m403 review found by reading are closed, each with a test that fails on the old behavior. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | A retried send that starts a turn never starts two: the idempotency key is honored for every keyed send | 60m | — |
| t002 | A turn is finalized `interrupted` only if bex-api recorded a stop request for it | 45m | — |
| t003 | A reorder never trades places with the message being started, and an edit after a lapsed lease is not lost | 45m | — |
| t004 | A blocked message does not run `Steer` more than about twice a minute | 30m | — |
| t005 | Render parity (Devin) | 20m | t001, t002, t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- A keyed send retried after a lost reply returns the earlier outcome whether the first attempt started a turn, queued, or was already dispatched; it never starts two turns or queues a duplicate; the same key with another message is `AGENT_SESSION_IDEMPOTENCY_KEY_REUSED`.
- A turn is finalized `interrupted` only when bex-api recorded a stop request for that turn; a status `interrupted` without one takes the normal completion path, so a turn that owes a structured result still fails without it.
- A reorder never trades places with a message the dispatcher holds a lease on (a coded 409 `AGENT_QUEUED_MESSAGE_DISPATCHED`), and the turn runs the latest saved text of a message that was edited after its lease lapsed.
- A message blocked by the spend ceiling or the live-sandbox cap is retried at most about twice a minute; unblocked messages are delivered as before.
- Every change has a test that fails on the old code, against real Postgres, and ADR047 D12 "As built" records it.

## Source + Goal linkage

- **Source:** inbox note w22/014 (promoted), found by the w22/m403 review on 2026-11-14 by reading `lego/backend/internal/store/agentsession_queue.go`, `lego/backend/internal/agentsessions/{service.go,queue.go,message_queue.go,completion.go}`. The design is [ADR047 D12](../../../docs/ADR047-cloud-coding-agent-sessions.md) and its "As built" notes.
- **Goal linkage:** ADR008's AI-native thesis and pillar 5 (agent sessions); the reference product is Devin ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)), whose queued messages survive idle and resume and can be edited, removed and reordered ([release notes](https://docs.devin.ai/release-notes/2026), 2026-09-09 and 2026-10-16).
- **Expected outcome:** the queue's promises hold under retries, lease lapses and a hostile status file, not only on the happy path.
- **Why now:** w22/m403 shipped the queue on 2026-11-14 and these are its known gaps; the longer the queue is in use, the more retried sends and lapsed leases it meets.
- **Render parity included (reference product: Devin):** the key behavior is visible on REST, GraphQL, MCP and the CLI, so t005 compares each surface and Devin.
