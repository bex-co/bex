# w3 · m404 — Long-running `bex agent` commands keep their login past the 15-minute access token

**Worker:** worker3 **Goal:** `bex agent chat`, `wait`, `--follow` and every other long-running `bex agent` command keeps working past the access token's 15-minute lifetime, and two `bex` processes sharing one login do not sign each other out. **Status:** done (2026-11-26; verified on production, see Live check)

## Tasks (in order)

| id   | title                                                  | est | depends_on |
| ---- | ------------------------------------------------------ | --- | ---------- |
| t001 | Reproduce on production and confirm the cause — **DONE** | 30m | —          |
| t002 | Refresh the token mid-run from the shared login — **DONE** | 75m | t001       |
| t003 | Use it on every long-running `bex agent` path — **DONE** | 45m | t002       |
| t004 | Live check on production past 15 minutes — **DONE** | 45m | t003       |
| t005 | Simplify — **DONE** | 15m | t004       |
| t006 | Test coverage — **DONE** | 30m | t004       |
| t007 | Closeout — **DONE** | 10m | t006       |

## Definition of done

1. On production, a `bex agent wait --follow` (or `bex agent chat`) on one QA session runs for more than 20 minutes and keeps receiving updates, with no `401`/`token expired` and no new login.
2. While it runs, other `bex` commands in a second terminal succeed every few minutes, and afterwards both terminals still work without a login (`bex agent list` succeeds).
3. Automated tests with a fake token endpoint and a short access-token lifetime show that a long-running loop refreshes before expiry; a refresh token another process rotated on disk is adopted instead of the stale in-memory one; and a failed refresh never overwrites a newer login another process wrote.

**Closeout (2026-11-26):** DoD 1 and 2 hold on production (Live check below: 22 min 41 s of `--follow` and `wait` under eight rotations of the shared login, no `401`, no new login, both builds' `bex agent list` working afterwards). DoD 3 holds in `lego/cli/internal/agent/auth_test.go`, with the 15-minute lifetime replaced by the real cause the t001 finding names: rotation by a second command.

## Source + Goal linkage

- **Source:** found during the w1/m338–m341 live checks, 2026-11-12: the background watcher's `bex agent` lost its login mid-run and needed a separate login. The likely cause was read from code in the same session. Upstream `client.NewDefaultClient()` refreshes the token only when the client is built (`render-oss/cli` `pkg/client/client.go` `maybeRefreshAPIToken`), so once per command. `lego/cli/internal/agent/api.go` `defaultAPI()` builds it once and keeps its static bearer request editor for the whole run. Access tokens live 15 minutes and refresh-token rotation has a 60 s grace (`deploy/gitops/base/values/hydra.values.yaml`: `access_token: 15m`, `rotation_grace_period: 60s`). t001 confirms this on production before the fix.
- **Goal linkage:** [ADR106](../../../../docs/ADR106-cli-cloud-agents-automations.md) D3 (chat, follow and wait last a whole agent turn, which routinely exceeds 15 minutes); [ADR012](../../../../docs/ADR012-auth.md) auth.
- **Expected outcome:** `bex agent` users are not signed out mid-turn and never need a second login.
- **Why now:** CLI v0.6.0 (released 2026-11-12) shipped `bex agent chat`, `--follow` and `wait` to users; any turn longer than 15 minutes ends in an auth error.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard change; the fix lives in the launcher's own agent client. It uses upstream's public `client` and `config` packages and must not fork or vendor the upstream CLI (`.pm/DO_NOT_DO.md`, first-party CLI).

## t001 finding (2026-11-26): the cause is a second `bex` command, not a 15-minute token

A `bex` built from `b869771f19` signed in as the QA user by device login into its own config directory (`BEX_CLI_CONFIG_DIR`), workspace `bex`.

- **The CLI's access token lives seven days, not 15 minutes.** The fresh login's `expires_at` was 604,788 s (≈168 h) ahead. `access_token: 15m` in `hydra.values.yaml` is Hydra's global default; the bex CLI's own client overrides both of its grants with `CLI_TOKEN_LIFESPAN=168h` (`scripts/auth-bootstrap-client.sh`, w4/m38, 2026-08-17). A lone `bex agent` run therefore keeps its token for days.
- **A refresh revokes the access token before it, at once.** With the stored login moved into its last day (`expires_at` set to now + 1 h in that private file, which is when upstream refreshes on every command), one `bex workspaces` rotated the login. Probed 7 s later, the previous access token answered `401` and the new one `200`.
- **Reproduced end to end.** Terminal A, 09:14:23 UTC: `bex agent run --follow` on a three-step `sleep 60` turn (session `ags-dc3vgrpm1rus738gvo4g`, `bex` workspace, disposable). Terminal B, 09:15:41: `bex agent list` with the login in its last day, which rotated it. A kept printing the turn, because the stream carries a ticket rather than the bearer, but at the turn's end, 09:17:38, it printed `OK` and then `Error: get agent session ags-dc3vgrpm1rus738gvo4g: 401 unauthorized` and exited 1. B succeeded.

So the user-visible bug is real, but it has a different cause. Upstream refreshes on every command in a token's last 24 hours, and every refresh signs out each other running `bex` process that still holds the previous token. A run that outlives the token's own expiry is the rarer case. **t002 is revised:** before every request the agent client re-reads the login on disk and adopts a newer one another process wrote, without a refresh call, so it never rotates the login itself on that path. It refreshes only within 2 minutes of expiry, or once after a `401`. A refresh goes through upstream's public `oauth` client and is written back only while the file still holds the login it refreshed from. A failed refresh writes nothing; upstream's own failure path instead clears the refresh token. t004 drives terminal B the same way, with the login in its last day before every command, so each one rotates the login under the running `bex agent`.

## Live check (t004, 2026-11-26)

A `bex` built from this milestone's tree ran against production with the same QA device login and config directory as t001. No new login was made at any point.

| Time (UTC) | Terminal | What happened |
| --- | --- | --- |
| 09:28:07 | A | `bex agent run --follow` started a turn of twelve `sleep 110` steps (session `ags-dc3vn9pm1rus738gvov0`, `bex` workspace, disposable) |
| 09:28 | A2 | `bex agent wait ags-dc3vn9pm1rus738gvov0` began polling the same session |
| 09:31:10 – 09:52:19 | B | `bex agent list`, eight times, three minutes apart, each with the stored login moved into its last day first, so each one rotated the login and revoked the token A and A2 held. All eight exited 0 |
| 09:28:17 – 09:48:44 | A | printed each of the twelve `▸ Terminal sleep 110` steps as it started, about every 1 min 51 s |
| 09:50:45 | A2 | `succeeded (turn 1)`, exit 0 |
| 09:50:48 | A | `DONE`, the session summary, exit 0, after 22 min 41 s |
| 09:52:20 | both | `bex agent list` from the fixed build and from the `b869771f19` build exited 0 |

Neither A nor A2 printed a `401`, `unauthorized` or a login prompt. Under the same rotation, the `b869771f19` build failed in t001 at the end of its turn.

The live build predates t005's behavior-preserving edits, so a build of the final tree reran t001's scenario: terminal A `bex agent run --follow` at 09:52:42 (session `ags-dc402qmsfruc73ft24h0`, disposable), and terminal B `bex agent list` at 09:53:58 with the login in its last day. A printed `OK` and exited 0 at 09:56:00. The three sessions were then deleted, the CLI login was logged out (which revokes its grant), and both Kratos sessions were revoked.
