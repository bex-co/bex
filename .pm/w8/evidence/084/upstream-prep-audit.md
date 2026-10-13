# w8/084 upstream preparation audit

Date: 2026-10-12

## Patch prepared

`084-upstream.patch` against `render-oss/cli` pin `a764810a768202704e7206eb7b87a47211fcd98e` (prepared in `a temporary upstream clone`).

Premise holds at the pin: `pkg/postgres/repo.go:91` calls raw generated `RestartPostgres` (returns `*http.Response`) and hands it to `client.ErrorFromResponse`; `firstNonNilErrorField` (`pkg/client/client.go:135-139`) finds no `HTTPResponse` field and returns nil, so every 4xx/5xx is reported as success (`pkg/tui/views/restartview.go:60-65` then prints `<id> restarted successfully`). The raw body is also never closed.

Change (one line): `RestartPostgresDatabase` now calls `RestartPostgresWithResponse`, which parses through `ParseRestartPostgresResponse` (reads and closes the body, sets `Body`/`HTTPResponse`), so the shared error helper sees the status. Request, signature and 200/202 acceptance are unchanged; transport errors still propagate. No exported API changes.

Tests added:

- `pkg/postgres/repo_test.go` — repository: 200 and 202 succeed; 400/404/409/500/503 return the named `received response code N (CODE): message` error, 429 returns `client.ErrTooManyRequests`; closed-server transport error is returned.
- `cmd/restart_test.go` — root `render restart <dpg-id> --confirm` against an httptest API: 404 fails in json/yaml/text with the named diagnostic on stderr and empty stdout; `srv-` 404 sibling control fails; 200/202 print the JSON success string.

## Shared-caller review

- `RestartPostgresDatabase`: 1 definition (`pkg/postgres/repo.go:90`), 1 service wrapper (`pkg/postgres/service.go:99`), 1 dispatcher (`pkg/resource/service.go:229`), reached only via `views.RestartResource` — used by both the noninteractive root command (`cmd/restart.go:42`) and the interactive `RestartView` (`restartview.go`), so both paths are fixed by the one change.
- Production calls of a raw generated client method (non-`WithResponse`): 2 — this one, and `pkg/analytics/post.go:26` (telemetry, intentionally reads `StatusCode` directly and closes the body; not an `ErrorFromResponse` consumer, unaffected).
- `ErrorFromResponse` production call sites: 66; all others already pass decoded `*XxxResponse` wrappers.
- Service sibling `pkg/service/repo.go:151` already uses `RestartServiceWithResponse` (control test passes before and after). Postgres suspend/resume/delete, KV/cron/workflow restart rules untouched.

## Tests run

```sh
cd a temporary upstream clone
go test ./pkg/postgres/... ./pkg/resource/... ./pkg/service/... ./pkg/tui/views/... ./pkg/client/ ./cmd/
go vet ./pkg/postgres/... ./cmd/
```

All `ok` (postgres, service, tui/views, tui/views/workflows, client, cmd); vet clean; gofmt clean.

## Fail-before evidence

With only `pkg/postgres/repo.go` reverted to the pin, `go test ./pkg/postgres/ ./cmd/ -run Restart`:

- FAIL `TestRestartPostgresDatabase_ReturnsAPIErrors` — all 6 subtests (400/404/409/429/500/503): "An error is expected but got nil."
- FAIL `TestRestart_PostgresAPIRefusal_FailsInEveryOutputFormat` — json/yaml/text.
- PASS (controls, as expected): 200/202 success, transport error, service 404 sibling.

After restoring the fix all pass.

## Upstream dedupe

- `origin/main` fetched 2026-10-12: `668af8a87195f4de4dead5f41722c46f840fd94e` (32 commits past pin). `pkg/postgres/repo.go:91` on main still calls raw `RestartPostgres`; 0 commits since the pin touch `pkg/postgres/repo.go`, `pkg/client/client.go`, `restartview.go` or `cmd/restart.go`. Neither test file exists on main. Patch `git apply --check`s cleanly onto main.
- `gh search prs|issues --repo render-oss/cli` for `RestartPostgres`, `postgres restart`, `restarted successfully`: no results (first attempt rate-limited, retried successfully). Search is not exhaustive.

## What remains

- External submission (issue/PR to render-oss/cli) needs explicit user authorization; nothing was sent.
- Upstream acceptance and a Render CLI release containing the fix.
- Bex pin refresh in `lego/cli` + Bex CLI release (no Bex-side code change is needed; the Bex server correctly returns 404).
- Native receipts: repeat the synthetic `bex restart dpg-00000000000000000000 --confirm -o json` (expect exit 1, named 404) and an owned Free Postgres restart with SQL start-time/data controls; update the compatibility ledger.
