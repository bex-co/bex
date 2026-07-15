# w2 · m36 — Cron-job runs API: list · get · cancel

**Worker:** worker2 **Goal:** Cron runs become first-class objects: bex's `GET /cron-jobs/{id}/runs`, `GET .../runs/{runId}`, and `POST .../runs/{runId}/cancel` extensions exist on all three surfaces, Render's current trigger/active-cancel routes remain compatible, and cancel actually kills the running Job. **Status:** **DONE 2026-07-14**

## Tasks (in order)

| id   | title                                                    | est | depends_on | status     |
| ---- | -------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Verify Render's run-object shape + cancel semantics      | 30m | —          | — **DONE** |
| t002 | Run list/get over `status.runs`, Render envelope         | 45m | t001       | — **DONE** |
| t003 | Cancel verb: kill the active Job, honest terminal status | 45m | t002       | — **DONE** |
| t004 | GraphQL + MCP mirrors                                    | 40m | t003       | — **DONE** |
| t005 | Dashboard: runs table on the cron service page           | 40m | t004       | — **DONE** |
| t010 | Add cron `lastSuccessfulRunAt`                           | 30m | t005       | — **DONE** |
| t006 | Render parity                                            | 30m | t010       | — **DONE** |
| t007 | Simplify                                                 | 30m | t006       | — **DONE** |
| t008 | Test coverage                                            | 45m | t006       | — **DONE** |
| t009 | Closeout                                                 | 15m | t008       | — **DONE** |

## Definition of done

For a cron-job service: run history lists with Render's field names and cursor envelope; a single run fetches by id; canceling an in-flight run terminates its k8s Job and the run settles to the captured cancel status (a completed run's cancel is a 4xx, never a silent no-op); all three API surfaces behave identically; the dashboard cron page shows the runs table with a working Cancel.

## Outcome and DoD evidence (2026-07-14)

- Re-verified Render's live contract: `POST /cron-jobs/{id}/runs` returns a run after canceling an active execution, and `DELETE .../runs` cancels current. Historical list/get/per-run cancel and all MCP run tools are explicitly documented bex extensions in `docs/render-artifacts/cron-runs.md`.
- Added stable derived `crr-…` IDs, Render status/timestamp fields, cursor paging, get, terminal-conflict cancel, and `lastSuccessfulRunAt`; REST, GraphQL, and MCP all delegate to the same service verbs. ADR006, ADR018, and ADR020 record the contract and divergences.
- `App.spec.cancelRun` keeps the backend mechanism-free. The operator foreground-deletes the exact Job, waits for deletion before a replacement, records and retains `Canceled` history, prevents stable `runAt` recreation, and applies `ForbidConcurrent` to scheduled runs.
- Authenticated live verification on the mock cluster passed for REST trigger/list/get/cancel, GraphQL list/get/cancel, and MCP list/get. A sleeping run's Kubernetes Job disappeared and its API/CR status settled to `canceled`/`Canceled`; a terminal cancel returned 409.
- The dashboard Events page was exercised in a real headless Chrome session against the browser-owned live service: it rendered the pending run, required confirmation, canceled it, and updated the row to Canceled. That pass caught and fixed a stale post-cancel refetch race.
- Verification: operator `make test`; backend `go build ./... && go test ./...`; dashboard `yarn test` (177 files, 1,061 tests), `yarn typecheck`, and `yarn lint`; scoped operator-controller/backend golangci-lint and changed-dashboard Prettier checks all passed. Repository-wide `make lint` remains blocked only by existing `cmd/pg-sni-proxy` and `internal/build/build_test.go` findings; the global dashboard format check still reports unrelated baseline files.
- Simplification was a manual behavior-preserving review because no `/simplify` capability is installed in this Codex environment. It found the replacement-overlap window, dashboard page-count handling, and post-cancel cache race; all accepted fixes were applied and re-verified.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 3, 2026-07-14 (item 1); code fact: only the trigger exists (`apps/rest.go:504` `POST /v1/cron-jobs/{id}/runs`), run data buried in `status.runs`.
- **Goal linkage:** Render parity for the current trigger/active-cancel contract, plus explicit bex run-object extensions; extends the deploys-as-objects pattern (w2/m5/m10/m31).
- **Expected outcome:** the cron row's run-history half stops being read-only-inside-the-service-object; ADR018's cron row gains the runs endpoints.
- **Why now:** w2 is down to two open milestones; the deploys envelope/cancel patterns to copy are freshest now (m30/m31 just closed). Render parity task included — all-surface change.
