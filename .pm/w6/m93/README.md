# w6 · m93 — Postgres internal connection-string host mismatch; log live-tail resume cursor (w9/053 follow-up)

**Worker:** worker6 **Goal:** a connection string a user copies from the dashboard always resolves; a live-tail log view never shows a line that didn't happen twice **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on         |
| ---- | ---------------------------------------------------------------------------------- | --- | ------------------- |
| t001 | Fix Postgres InternalConnectionString host mismatch against its own psql command | 25m | —                    |
| t002 | Implement backend SSE resume cursor for log live-tail (w9/053's deferred follow-up) | 40m | —                    |
| t003 | Render parity: REST/GraphQL/UI agree on both fixes                               | 20m | t001, t002           |
| t004 | Simplify                                                                          | 15m | t003                 |
| t005 | Test coverage                                                                     | 30m | t004                 |
| t006 | Closeout                                                                          | 10m | t005                 |

## Definition of done

- Create a fresh Postgres, reveal connection info: the Internal connection string's host and the psql command's host are identical (both `.svc`-qualified) — live-verifiable on `dashboard.bex.co`.
- On a Cron Job's Logs tab with Live tail on, across at least 2 real scheduled-run ticks, the live-tail output for each run matches the historical (non-live) query for the same run, with no duplicate lines — live-verifiable on `dashboard.bex.co`.
- Both fixes agree across REST/GraphQL (and UI); any render.com divergence is recorded, not silently shipped.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co` hosting surfaces, 2026-08-25 (run from a `/loop 10m` session), journeys 11 (Postgres: create, connection-info reveal) and 10 (Cron Job: create, schedule, logs). Full findings recorded in the hunt's task-notification transcript; no screenshots were captured this cycle (hunt gap) — evidence is the exact quoted connection-string values (t001) and the reproduced-with-control-case behavior pattern (t002), both restated in full in each task.
- **Goal linkage:** trustworthy hosting primitives (ADR009 Postgres, ADR006 bex-api Render-compatibility) — a copy-pasted connection string that silently uses the wrong host form, and a log viewer that shows a phantom duplicate, both erode confidence in the platform's basic promises even though neither is a blocker.
- **Expected outcome:** the Postgres connection-info panel is internally consistent; the log live-tail no longer exhibits the "boundary second" duplicate-line gap `w9/053` (done, 2026-08-22) explicitly deferred.
- **Why now:** both were found live in the same hunt cycle that also confirmed `w6/m50`/`m51`/`m52`'s fixes are correct but blocked from production by the `deploy.yml` runner outage tracked in `w6/040.md` — filing this now keeps the backlog current while that separate blocker is worked, rather than losing the findings.
- **Render parity task included:** t003 — both fixes touch REST/GraphQL surfaces (`PostgresConnectionInfo`, the log-subscribe endpoint).
