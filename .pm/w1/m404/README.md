# w1 · m404 — Long-running `bex agent` commands keep their login past the 15-minute access token

**Worker:** worker1 **Goal:** `bex agent chat`, `wait`, `--follow` and every other long-running `bex agent` command keeps working past the access token's 15-minute lifetime, and two `bex` processes sharing one login do not sign each other out. **Status:** todo

## Tasks (in order)

| id   | title                                                  | est | depends_on |
| ---- | ------------------------------------------------------ | --- | ---------- |
| t001 | Reproduce on production and confirm the cause          | 30m | —          |
| t002 | Refresh the token mid-run from the shared login        | 75m | t001       |
| t003 | Use it on every long-running `bex agent` path          | 45m | t002       |
| t004 | Live check on production past 15 minutes               | 45m | t003       |
| t005 | Simplify                                               | 15m | t004       |
| t006 | Test coverage                                          | 30m | t004       |
| t007 | Closeout                                               | 10m | t006       |

## Definition of done

1. On production, a `bex agent wait --follow` (or `bex agent chat`) on one QA session runs for more than 20 minutes and keeps receiving updates, with no `401`/`token expired` and no new login.
2. While it runs, other `bex` commands in a second terminal succeed every few minutes, and afterwards both terminals still work without a login (`bex agent list` succeeds).
3. Automated tests with a fake token endpoint and a short access-token lifetime show that a long-running loop refreshes before expiry; a refresh token another process rotated on disk is adopted instead of the stale in-memory one; and a failed refresh never overwrites a newer login another process wrote.

## Source + Goal linkage

- **Source:** found during the w1/m338–m341 live checks, 2026-11-12: the background watcher's `bex agent` lost its login mid-run and needed a separate login. The likely cause was read from code in the same session. Upstream `client.NewDefaultClient()` refreshes the token only when the client is built (`render-oss/cli` `pkg/client/client.go` `maybeRefreshAPIToken`), so once per command. `lego/cli/internal/agent/api.go` `defaultAPI()` builds it once and keeps its static bearer request editor for the whole run. Access tokens live 15 minutes and refresh-token rotation has a 60 s grace (`deploy/gitops/base/values/hydra.values.yaml`: `access_token: 15m`, `rotation_grace_period: 60s`). t001 confirms this on production before the fix.
- **Goal linkage:** [ADR106](../../../docs/ADR106-cli-cloud-agents-automations.md) D3 (chat, follow and wait last a whole agent turn, which routinely exceeds 15 minutes); [ADR012](../../../docs/ADR012-auth.md) auth.
- **Expected outcome:** `bex agent` users are not signed out mid-turn and never need a second login.
- **Why now:** CLI v0.6.0 (released 2026-11-12) shipped `bex agent chat`, `--follow` and `wait` to users; any turn longer than 15 minutes ends in an auth error.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard change; the fix lives in the launcher's own agent client. It uses upstream's public `client` and `config` packages and must not fork or vendor the upstream CLI (`.pm/DO_NOT_DO.md`, first-party CLI).
