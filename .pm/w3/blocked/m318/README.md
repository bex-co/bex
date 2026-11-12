# w3 · m318 — Terminated and errored sandboxes stay readable, as on Render

**Worker:** worker3 **Goal:** an ended sandbox answers `GET` with `status: terminated` and `terminatedAt`, lists under `status=terminated`, terminates idempotently, keeps its execution history readable, and refuses a connect-token mint with 409 — on REST, GraphQL and MCP **Status:** blocked — t001–t007 done; t008 Closeout waits on the production deploy and its live check

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A sandbox record that outlives the OpenSandbox object — **DONE** | 1.5h | — |
| t002 | Create, terminate, the lifetime reaper and the sweeper write and close it — **DONE** | 1.5h | t001 |
| t003 | Reads fall back to it: GET, by-id routing, list, terminate, executions — **DONE** | 1.5h | t002 |
| t004 | Connect-token mints answer 409 for an ended or errored sandbox — **DONE** | 45m | t003 |
| t005 | Render parity — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005 |
| t008 | Closeout | 15m | t006, t007 |

## Definition of done

- After `POST /v1/sandboxes/{id}/terminate`, or a lifetime expiry, `GET /v1/sandboxes/{id}` answers 200 with `status: terminated` and `terminatedAt`; `GET /v1/sandboxes` omits it by default and lists it under `status=terminated`; a repeat terminate answers 204; `GET …/execs` still lists its executions; GraphQL `sandbox`/`sandboxes` and MCP `list_sandboxes` agree
- `POST …/runs/{operation}/token` and `…/files/{operation}/token` answer 409 for a terminated or errored sandbox, so the Render CLI v2.36 prints "sandbox … was terminated at …"
- An ended sandbox stays readable for 30 days (bex's suspended-sandbox and execution-history retention; Render documents no window), then answers 404
- Live after deploy: in `bex-canary` as the QA user, create → terminate → GET 200 terminated, token 409, and the unmodified v2.36 CLI's `sandbox exec` on it prints the terminated explanation

## Source + Goal linkage

- **Source:** promoted from inbox note `062` (2026-11-10), found by w3/m303 t004 (production evidence in the note); Render's pinned OpenAPI `sandbox` object (`status` enum with `terminated`/`errored`, nullable `terminatedAt`), list-sandboxes' `status` filter ("If omitted, returns sandboxes in every status except `terminated`"), terminate ("Idempotent: returns 204 from any state, including `terminated`"), the connect-token 409s, and CLI v2.36 `pkg/sandbox/repo.go` `explainConflict` ("returns for a terminated or errored sandbox"); render.com/docs/sandboxes-creating ("Termination is permanent… terminated sandboxes are excluded by default", listed with `--all`)
- **Goal linkage:** pillar 1, Render-compatible REST and CLI (ADR006, ADR042); Render parity first (ADR008); ADR018's "A terminated or errored sandbox" row
- **Expected outcome:** the Render CLI and SDK explain an ended sandbox instead of reporting it missing, and an agent can still read what ran in a sandbox after it ended
- **Why now:** v2.36 (adopted by w3/m303) ships the 409 explanation, and every ended sandbox today is indistinguishable from a mistyped id
- **Render parity:** included — REST, GraphQL and MCP sandbox surfaces change. The dashboard has no sandbox list (only the sandbox-group dialog), so it is unaffected. Render documents no retention window for ended sandboxes; 30 days is bex's own sandbox retention, recorded in ADR018 as an assumption to re-check, not a divergence.

## Progress (2026-11-10)

- Migration 0224 `sandbox_records` and `store.RecordSandbox` (one upsert: a live record at create, the end stamped once, keeping the created view), `EndAbsentSandboxes`, `ExpireSandboxRecords` (global, per sweep), `EndedSandbox(es)`, `EndedSandboxWorkspaces`; `PruneSandboxExecutions` keeps a recorded sandbox's history.
- Create records the sandbox; terminate (`lifecycle`), the lifetime reaper and agent cleanup (`terminateObserved`) end it; the idle sweeper ends a recorded sandbox OpenSandbox stopped listing and expires records ended 30 days ago.
- `Get`, `listFiltered` (under `status=terminated`), `executionsTarget` and the by-id router fall back to the ended record behind the same owner-or-admin boundary (`ownerCheck`, now shared with the live list); a repeat terminate answers 204; both mints, suspend and resume answer 409 `SANDBOX_ENDED` for an ended sandbox, and the mints for a live `errored` one. `Sandbox.terminatedAt` on REST and MCP; GraphQL gains `createdAt` and `terminatedAt`.
- Render parity (t005): REST, GraphQL and MCP read the same ended sandbox (`TestTerminatedSandboxStaysReadable`); ADR018's row is ✅ with the 30-day window recorded as an assumption to re-check; ADR042 updated. A sandbox created before the migration and ended after it keeps only its OpenSandbox-derived fields (no name or group). Not filed: agent-session sandboxes are recorded too, as the live list shows them.
- Tests: `TestSandboxRecordsPG`, `TestTerminatedSandboxStaysReadable`, `TestErroredSandboxRefusesConnectTokens`, `TestSweeperEndsAbsentSandboxesAndExpiresOldRecords`, the reaper assertion in `TestReconcileWorkspaceInventoryHonorsLongLifetimes`.

## Live check, partial — 2026-11-12 (`/qa-find-bugs-cli` sweep 4488, w32)

- **Deployed:** pin `3d66946c9e16` contains `c2eee0472`.
- **Terminate path passes on production:** a sandbox stopped with `bex sandboxes stop` answers `409 SANDBOX_ENDED … was terminated at <real time>` and lists under `--status terminated` (sweeps 865a, 42aa, `bex-canary`).
- **Lifetime-expiry path fails the DoD for a sandbox younger than ten minutes:** `--timeout 60` sandbox `sbx-dbqnb0qibtfs738vkrb0` answered `404 SANDBOX_NOT_FOUND` from 08:16:02 to 08:24:56 UTC, then read `terminated` with `terminatedAt` 08:25:09, nine minutes after it ended. Cause and fix: [w32/017](../../../w32/017.md) (`EndAbsentSandboxes`' ten-minute guard and sweep-time stamp).
- **t008 Closeout waits for w32/017**, because the first DoD bullet names "a lifetime expiry".
