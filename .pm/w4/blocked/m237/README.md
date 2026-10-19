# w4 · m237 — Live request logs arrive in seconds, not on the range-poll clock

**Worker:** worker4 **Goal:** with Live tail on, a request line reaches the Log Explorer within ~5 s of hitting the service in All and HTTP Requests views — the log store becomes a producer of the subscribe stream alongside pod stdout. **Status:** blocked (t001–t005 done 2026-10-19, loopx w4; t006 live closeout waits on the deploy)

## Tasks (in order)

| id             | title                                                                 | est | depends_on                 |
| -------------- | --------------------------------------------------------------------- | --- | -------------------------- |
| [t001](done/t001.md) | Backend: log-store poll producer for request logs in the subscribe stream — **DONE** | 60m | —                          |
| [t002](done/t002.md) | Dashboard: offer live tail for request views and send their filters — **DONE**  | 30m | w4/m237/t001               |
| [t003](done/t003.md) | Render parity — **DONE**                                                         | 15m | w4/m237/t002               |
| [t004](done/t004.md) | Simplify — **DONE**                                                              | 15m | w4/m237/t003               |
| [t005](done/t005.md) | Test coverage — **DONE**                                                         | 30m | w4/m237/t003               |
| [t006](t006.md) | Closeout                                                              | 20m | w4/m237/t004, w4/m237/t005 |

Six tasks, 170m (2h50m).

## Definition of done

- With Live tail on, a request line appears in the Log Explorer within ~5 s of the request in the **All** view and the **HTTP Requests** view, at a human request rate.
- Live request lines carry the same fields as the history rows for the same request (status pill renders); each underlying request appears exactly once across history, live frames and the range refetch, in timestamp order — including requests sharing one timestamp.
- Subscribing with `type=request` (with or without its structured filters) is accepted and streams only matching request lines; a mixed app+request subscription merges pod and store lines on one stream. Producerless subscriptions are still refused before a slot is taken.
- With no log store wired, a request subscription fails closed before streaming headers; the historical path is unchanged.
- Checks pass: backend `go test ./...` (real dependencies in CI), dashboard `yarn lint`, `yarn typecheck`, `yarn test`.

## Source + Goal linkage

- **Source:** w4/367 (live `/qa-find-bugs w4` pass 305022, 2026-10-19), promoted in-run by loopx w4: the backend producer route is the parity fix and spans backend + dashboard + tests. The live capture: 7 curls, app lines ≤1 s, first request line at ~30 s; the closed reason string is `http metrics have not appeared yet in this sampling window (1/2m)` on the same seconds-old requests' Metrics cards.
- **Goal linkage:** the logs surface of the Render-compatible hosting product; ADR010 (observability) owns the "tail reads pod logs" line this milestone extends with a second, store-backed producer — it does not change the pod tail's semantics.
- **Expected outcome:** Live tail means the same thing in every log view; `render logs --tail` and SSE/NDJSON clients get request lines live too, since the producer sits behind `followLogs`.
- **Why now:** the only live producer gap left beside `type=predeploy`; the dashboard's store-only gating and the server refusal both rest on it.
- **Render parity closing task:** included — REST subscribe dialects all ride the stream this changes. GraphQL/MCP expose no log tail.
