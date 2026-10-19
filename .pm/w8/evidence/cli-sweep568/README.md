# CLI sweep 568 — log filters on generated traffic; `task`/`taskRun`/`sandbox` silently widen

**Verdict: one bug filed — [w8/132](../../132.md).** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T18:57–19:13Z. Owned fixture `qa568-logs` (`srv-dbb6gavo17is73furrq0`, free hello-app) fed six known requests plus two tail markers; deleted with REST + URL 404 proof. Other loops' rows in the workspace were left untouched.

Passing coverage (parse via `/tmp/qa501/logs568.py` — `logs -o json` is concatenated pretty objects, not one array):

- Exact discrimination on generated traffic: all=13 rows (6 requests, 7 app, 2 build outside the untyped read); `type=request` 6; `path` α=4 (3 GET + 1 POST), β=1, γ/sub=1; path is exact-match (prefix `/qa568` → 0); `method=POST` 1; combined POST+path 1; `host` platform 6 / bogus 0; `status-code=404` 0.
- App rows carry `level: unknown` (hello-app emits unstructured lines), so `--level info`/`error` → 0 is exact-label truth, not a filter bug. Two early "0 rows" cells (β, POST+α) were ingestion lag under a broken first-pass parser; correct re-reads are the table above.
- **`--tail` live:** first attempt false-negatived on Python buffered-`select` reading (harness); unbuffered retest (`/tmp/qa501/tail568b.py`) streamed the backlog then both mid-stream request markers (`qa568-tail-…`, `qa568-tail2-…`) in arrival order.

The finding (full research in [w8/132](../../132.md)): Render-declared `task`/`taskRun`/`sandbox` log filters are admitted by the OpenAPI query gate but never parsed (`parseLogParams`/`LogQuery` have no such fields), so bogus values return the complete unfiltered page — REST 17/17 after tail traffic, CLI `--task-id` 17/17 — while bogus `host`/`instance` correctly return 0 and misspelled `taskId` correctly 400s as unsupported. The pinned client does transmit the flags (`logloader.go` → `ListLogsParams.Task/TaskRun`); GraphQL/MCP expose no such args. No ADR010/ADR018 stance records accepted-but-inert for these names.

## Unrun

- Whether Render's own service applies `task`/`taskRun` as label filters against non-workflow resources (no Render account in this harness; the filing rests on bex's own never-widen invariants, which hold either way).
- `/logs/values` and `/logs/subscribe` under task filters live (same shared parser; named in the filing's acceptance so the fix covers all three routes).
- Wildcard/regex structured-filter semantics beyond exact values (ADR010 documents bex's `*` subset; untouched here).
