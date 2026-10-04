# m152 live acceptance — 2026-10-02 (UTC 2026-10-03T06:50–07:04Z)

**Deployed:** dashboard from GitOps pin `20fb64867` → `1263d12ae` (contains `79910e98c` and the requested `18958459c`). **Fixture:** free cron job `qa-20261003-cron-m152` / `srv-db0aba0ehcmc739j13rg`. Source: public `bex-co/bex` `main`, `examples/hello-go`, Go, `go build -o app .`. Command `date -u; echo qa-m152-cron-fired`, schedule `*/5 * * * *`, auto-deploy off. Created 06:50:17Z, deploy live 06:54:06Z.

**Browser:** independent headless Chromium (Playwright library, own QA session, 1440×900, UTC), because the shared MCP browser was busy. The sampler is the finding's `main time[datetime]` sampler, run every 30 s on an untouched, visible tab. Console: 0 errors / 0 warnings on every page.

## Sample 1 — fresh page after the 06:55:08Z success, unchanged data until 07:00

| UTC | Last run (06:55:08) | Next run (07:00:00) | Created (06:50:17) | Deploy row (06:54:06) |
| --- | --- | --- | --- | --- |
| 06:55:32.8 | now (−25 s) | in 4m (267 s) | 5m (−316 s) | Deployed 1 minute ago |
| 06:56:02.8 | now (−55 s) | in 4m (237 s) | 5m | 1 minute ago |
| 06:56:32.8 | **1m** (−85 s) | **in 3m** (207 s) | **6m** | **2 minutes ago** |
| 06:57:02.8 | 1m | in 3m | 6m | 2 minutes ago |
| 06:57:32.8 | **2m** (−145 s) | **in 2m** (147 s) | **7m** | **3 minutes ago** |
| 06:58:02.8 | 2m | in 2m | 7m | 3 minutes ago |

## Sample 2 — another fresh page, 06:58:19–06:59:49 (same instants)

`3m / in 1m / 7m / 4 minutes ago` → `3m / in 1m / 7m` → `4m / now / 9m / 5 minutes ago` → `4m / now / 9m`. At 07:00:19, the next background poll had picked up the 07:00:10Z run (`now`, `in 4m` → 07:05:00, `10m`, `Deployed 6 minutes ago`), and the page stayed rendered.

## Open page vs reload

The open page showed `1m / in 3m / 11m / 8 minutes ago` at 07:02:10.6. A fresh load at 07:02:16.0 showed `2m / in 2m / 11m / 8 minutes ago`. Every open-page label equals the bucket for a clock sample no more than 60 s old (−61 s → 1m, 229 s → in 3m), so reload corrects nothing beyond the allowance. Before the fix, labels stayed frozen for several minutes (`now / in 4m / 4m` across 2 minutes).

## API instants and runs (unchanged)

- GraphQL `service`: `createdAt 06:50:17Z`, `lastSuccessfulRunAt 07:00:10Z`, `nextRunAt 07:05:00Z`. REST `serviceDetails` and MCP `get_service` report the same instants and schedule `*/5 * * * *`.
- Runs: CronJob schedule `*/5 * * * *`. Jobs `…-29850175` completed 06:55:08Z and `…-29850180` completed 07:00:10Z. `/v1/logs` shows `qa-m152-cron-fired` at 06:55:05Z and 07:00:07Z. No extra jobs were created.

## Cleanup

`deleteService` 07:02:42Z → `true`. At 07:03:33Z: REST 404, name-filtered list `[]`, no matching App/CronJob/Job/Pod/Secret/ConfigMap/ServiceAccount/NetworkPolicy in the namespace, and no cluster-wide pod/job/secret match. The session was revoked at the end of the run.
