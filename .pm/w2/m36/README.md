# w2 · m36 — Cron-job runs API: list · get · cancel

**Worker:** worker2 **Goal:** Cron runs become first-class objects: Render's `GET /cron-jobs/{id}/runs`, `GET .../runs/{runId}`, and `POST .../runs/{runId}/cancel` exist on all three surfaces, and cancel actually kills the running Job. **Status:** todo

## Tasks (in order)

| id   | title                                                    | est | depends_on |
| ---- | -------------------------------------------------------- | --- | ---------- |
| t001 | Verify Render's run-object shape + cancel semantics      | 30m | —          |
| t002 | Run list/get over `status.runs`, Render envelope         | 45m | t001       |
| t003 | Cancel verb: kill the active Job, honest terminal status | 45m | t002       |
| t004 | GraphQL + MCP mirrors                                    | 40m | t003       |
| t005 | Dashboard: runs table on the cron service page           | 40m | t004       |
| t006 | Render parity                                            | 30m | t005       |
| t007 | Simplify                                                 | 30m | t006       |
| t008 | Test coverage                                            | 45m | t006       |
| t009 | Closeout                                                 | 15m | t008       |

## Definition of done

For a cron-job service: run history lists with Render's field names and cursor envelope; a single run fetches by id; canceling an in-flight run terminates its k8s Job and the run settles to the captured cancel status (a completed run's cancel is a 4xx, never a silent no-op); all three API surfaces behave identically; the dashboard cron page shows the runs table with a working Cancel.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 3, 2026-07-14 (item 1); code fact: only the trigger exists (`apps/rest.go:504` `POST /v1/cron-jobs/{id}/runs`), run data buried in `status.runs`.
- **Goal linkage:** Render parity — the last Render REST endpoint family (cron runs) bex lacks; extends the deploys-as-objects pattern (w2/m5/m10/m31).
- **Expected outcome:** the cron row's run-history half stops being read-only-inside-the-service-object; ADR018's cron row gains the runs endpoints.
- **Why now:** w2 is down to two open milestones; the deploys envelope/cancel patterns to copy are freshest now (m30/m31 just closed). Render parity task included — all-surface change.
