# CLI sweep 569 — log label-value discovery (`/logs/values`) vs the filters it feeds

**Verdict: no new bug filed** (one cell re-touches [w8/132](../../132.md), already filed in sweep 568). Same isolated QA session, `bex-canary`, production pin `d98d07f62`, 2026-10-19T19:14–19:15Z. Read-only against baseline resources via same-credential REST (`/tmp/qa501/values569.py`); no fixture needed — sibling boundary evidence to the CLI logs sweeps, in the sweep-544 sense.

- **Discovery sets are scoped and truthful:** hello-go offers `type=[app,request]`, `level=[unknown]` (unstructured app lines), `method=[GET,HEAD,POST]`, `statusCode=[200,301,503]`, `host=[hello-go.onbex.co]` (answered from the App record, as ADR010 documents), and 50 historical `instance` ids from retention-window pod churn. Echo matches its own history (3 instances); the baseline cron offers only `type=[app]`; baseline Postgres offers `level=[error,info]`.
- **Refusals named:** Postgres + `label=method` → `managed Postgres log label "method" is unsupported (want instance|level)`; bogus/`resource`/`task` labels → `label must be one of instance|host|statusCode|method|level|type`; missing label and missing resource refuse at the query boundary; synthetic and just-deleted resource ids → uniform 404.
- **Discovery honors the accompanying filters:** `method` values under `type=app` → `[]` (request-only label over app streams); `type` values under `level=error` → `[]` on a service with no error lines; two `resource` params merge value sets; a duplicated `label` param resolves to the first.
- **Agreement with w8/132:** `label=level&task=tsk-bogus569` returned the unfiltered `['unknown']` — the same accepted-but-ignored widening on the values route, which the filing's acceptance already names (`/logs/values` shares `parseLogParams`). Not refiled.

## Unrun

- Discovery in pod-log fallback mode (`BEX_LOKI_URL` unset → store-backed labels 503 by design; production has the store).
- The dashboard Filters popover consuming these values (UI surface; the ADR010 narrowed-fallback behavior is source-covered).
