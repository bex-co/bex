# m65 burst replay on production — 2026-10-31

Workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), QA human via a fresh device login (bex CLI v0.2.1 = pinned Render CLI v2.27.0 contract), `https://api.bex.co/v1/`.

## Pre-clean (user decision 2026-10-31, `/unblock w8` A)

[Baseline](canary-before.json): 11 suspended Free services (2026-09-12 … 2026-10-26) and one Free Postgres (`dpg-db9r654flioc739mcpbg`, 2026-10-17); none created within 2 h. The 11 services were deleted by ID (`204`, then `GET` `404`: [delete](canary-cleanup-delete.txt), [404](canary-cleanup-404.txt)). The Postgres was already gone (`404`) — removed by a concurrent QA session, whose own new fixtures (`qa-20261031-9d3b80-*`) were left untouched.

## Fixture and bursts

Owned Free Postgres 18 `qa-20261031-w8m65-pg` = `dpg-dbj655mgv99s73ecla60`, created 21:50:14Z, available after ~90 s. Through `bex psql … --command`, one statement each ([times](bursts.txt)):

```sql
DO $qa$ BEGIN FOR i IN 1..<n> LOOP RAISE LOG '%', 'QAW8M65_7cc515_B<n> ' || lpad(i::text,4,'0'); END LOOP; END $qa$;
```

for n = 300 (21:52:39–50Z) and n = 1005 (21:52:50–53:03Z) — the same shape as sweep218.

## Retrieval

| path | 300 burst | 1005 burst |
| --- | --- | --- |
| REST `GET /v1/logs` (`text`, `limit=100`, backward, `nextStartTime/nextEndTime` cursor) — [result](rest-pages.json) | 300 rows, 300 distinct numbers, 3 pages, 0 duplicates | 1005 rows, 1005 distinct, 11 pages, 0 duplicates |
| bex CLI `logs -o json` walked by `--end` — [result](cli-walk.jsonl) | 300 / 300 | 1005 / 1005 |
| sweep218 before the CNPG 1.30.1 manager (w7/m156) | 102 / 300 | 109 / 1005 |

Neighbors: a Unicode LOG line `QAW8M65_7cc515_U 雪 λ Ι` comes back byte-exact; `SELECT` from a missing relation exits 1 and its `ERROR: relation … does not exist` line is retrievable.

## Cleanup

`DELETE /v1/postgres/dpg-dbj655mgv99s73ecla60` → `204`; `GET` → `404`; workspace Postgres list `[]`. The device-login CLI config, cookie jars and temp directory were removed after the run.

## Limits

- Retrieval checked through REST and the bex CLI (which is the pinned upstream CLI). GraphQL, MCP and the dashboard log views were not replayed; they read the same backend log query, and the defect was source retention, not a surface.
- The manager identity (CNPG 1.30.1, in-place instance managers) and primary/replica/pooler/backup compatibility rest on the w7/m156 production evidence, not a fresh cluster read (no strict read-only kubeconfig in this session).
