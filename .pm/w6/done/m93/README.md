# w6 · m93 — Postgres internal connection-string host mismatch; log live-tail resume cursor (w9/053 follow-up)

**Worker:** worker6 **Goal:** a connection string a user copies from the dashboard always resolves; a live-tail log view never shows a line that didn't happen twice **Status:** done — both fixes shipped and gated green; live verification carried to `w6/040` (blocked on the deploy pipeline). t002's duplicate-line premise was disproved, not implemented — see `done/t002.md`

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Fix Postgres InternalConnectionString host mismatch against its own psql command | 25m | — | — **DONE** |
| t002 | Implement backend SSE resume cursor for log live-tail (w9/053's deferred follow-up) | 40m | — | — **DONE** |
| t003 | Render parity: REST/GraphQL/UI agree on both fixes | 20m | t001, t002 | — **DONE** |
| t004 | Simplify | 15m | t003 | — **DONE** |
| t005 | Test coverage | 30m | t004 | — **DONE** |
| t006 | Closeout | 10m | t005 | — **DONE** |

## Definition of done

- Create a fresh Postgres, reveal connection info: the Internal connection string's host and the psql command's host are identical (both `.svc`-qualified) — live-verifiable on `dashboard.bex.co`.
- On a Cron Job's Logs tab with Live tail on, across at least 2 real scheduled-run ticks, the live-tail output for each run matches the historical (non-live) query for the same run, with no duplicate lines — live-verifiable on `dashboard.bex.co`.
- Both fixes agree across REST/GraphQL (and UI); any render.com divergence is recorded, not silently shipped.

## Outcome (2026-08-25)

**t001 was real and is fixed. t002's headline symptom was not — its stated mechanism cannot produce a duplicate line — but triaging it uncovered a different, real defect at the same call site, which is fixed instead.**

`PostgresConnectionInfo` echoed CNPG's raw `uri` (host spelled 2-label) while every other host on the same response — the psql command, the pooler strings, the per-replica strings — came from the operator's canonical `.svc`-qualified `Status.Host`. It now derives from `Status.Host` too, matching Key Value's precedent. One correction to the filed task: the two hosts agree only for a **private** database; with public access on, psql deliberately switches to the external host, so the tests split by shape.

For the log live-tail, the filed diagnosis was that an SSE replay lands in the same millisecond bucket and defeats the client dedupe. That is backwards: same millisecond + same pod + same message is an _identical_ `logLineKey`, which is exactly what the dedupe drops — and a replay re-parses the same kubelet bytes through the same code, so it cannot produce a differing key. Two independent dedupes (the live ring buffer's key set, which survives retries by design, and `mergeLogLines`' cross-source pass) both stand in its way, and the `w9/053` fix was confirmed deployed before the hunt. The live evidence does not isolate a defect either: live-on accumulates for the whole session while live-off is a bounded window, so they are different result sets by construction. No duplicate-line fix was made.

The real defect: `NewPodLogStream` set `Follow: true` with no lower bound, so **every** subscribe re-read the pod's entire log from offset 0. `LogQuery.Since` already existed and was already honored — but only by discarding lines _after_ kubelet had shipped the whole history into bex-api. That bound now reaches `PodLogOptions.SinceTime`, SSE frames carry `id:`, and `Last-Event-ID` resumes the window — so the browser EventSource's own invisible reconnect stops paying for a replay. Two self-inflicted bugs were caught by test during that change and fixed: an unindexed format verb that would have appended `%!(EXTRA …)` to every NDJSON line, and an empty `id:` on a stamp-less line, which per the SSE spec resets the client's cursor and would have silently disabled resume.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co` hosting surfaces, 2026-08-25 (run from a `/loop 10m` session), journeys 11 (Postgres: create, connection-info reveal) and 10 (Cron Job: create, schedule, logs). Full findings recorded in the hunt's task-notification transcript; no screenshots were captured this cycle (hunt gap) — evidence is the exact quoted connection-string values (t001) and the reproduced-with-control-case behavior pattern (t002), both restated in full in each task.
- **Goal linkage:** trustworthy hosting primitives (ADR009 Postgres, ADR006 bex-api Render-compatibility) — a copy-pasted connection string that silently uses the wrong host form, and a log viewer that shows a phantom duplicate, both erode confidence in the platform's basic promises even though neither is a blocker.
- **Expected outcome:** the Postgres connection-info panel is internally consistent; the log live-tail no longer exhibits the "boundary second" duplicate-line gap `w9/053` (done, 2026-08-22) explicitly deferred.
- **Why now:** both were found live in the same hunt cycle that also confirmed `w6/m50`/`m51`/`m52`'s fixes are correct but blocked from production by the `deploy.yml` runner outage tracked in `w6/040.md` — filing this now keeps the backlog current while that separate blocker is worked, rather than losing the findings.
- **Render parity task included:** t003 — both fixes touch REST/GraphQL surfaces (`PostgresConnectionInfo`, the log-subscribe endpoint).
