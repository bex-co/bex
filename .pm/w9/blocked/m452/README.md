# w9 · m452 — Members' sign-ins and sign-outs are audited as LoginEvent and LogoutEvent

**Worker:** worker9 **Goal:** a member's dashboard sign-in and sign-out each leave Render's `LoginEvent`/`LogoutEvent` row, with source IP and user agent, in every workspace they belong to **Status:** blocked (t001–t004 and t006–t008 done; t005 and t009 wait on the production pin carrying the w9/m452 commit)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Decide how bex-api learns of a sign-in and a sign-out — **DONE** | 45m | w9/m450/t002 |
| [t002](done/t002.md) | Record LoginEvent once per new session in each of the member's workspaces — **DONE** | 1h 30m | t001 |
| [t003](done/t003.md) | Record LogoutEvent from the dashboard sign-out and the CLI logout — **DONE** | 1h | t001 |
| [t004](done/t004.md) | REST, GraphQL, CSV, dashboard labels and docs for the sign-in events — **DONE** | 30m | t002, t003 |
| [t005](t005.md) | Production check: a QA sign-in and sign-out each leave one row per workspace | 30m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006 |
| [t009](t009.md) | Closeout | 15m | t007, t008 |

## Definition of done

- On production, signing in to the dashboard as the QA user records exactly one `LoginEvent` in each workspace the user belongs to, timestamped at the sign-in and carrying `source_ip` and `user_agent`; further requests on that session record no more, and two bex-api replicas seeing the new session at once still record one row per workspace.
- A sign-in through account recovery records one too.
- Signing out from the dashboard records one `LogoutEvent` per workspace.
- REST, GraphQL and the CSV export show both; a Hobby workspace records them and its admin sees the existing plan notice.
- A sign-in or sign-out never waits on, or fails because of, the audit write.

## Progress (2026-11-17)

- **Implemented:** `internal/signins` records `auth.Login` (Render's `LoginEvent`) the first time the auth gate sees a Kratos session that authenticated within the last hour, once in each of the member's workspaces, dated at `authenticated_at`, with `source_ip`/`user_agent` from the request through the trusted proxies (the Kratos device only when the request has none). Row ids derive from the session and workspace, so a retry or a second replica records nothing more; each replica remembers recorded sessions; the write is in the background and a failed one retries on the session's next request. `auth.Logout` (`LogoutEvent`) comes from the dashboard's new `recordSignOut` GraphQL mutation (`dashboard/src/common/lib/ory/report-sign-out.ts`, sent before the Kratos logout flow, best effort, 2 s; ids derived from the session, so a retried sign-out records once) and from `POST /v1/oauth/revoke` once Hydra revoked a human grant. REST and GraphQL map both; the CSV export reads GraphQL's `action`, so it follows.
- **t001 decisions** (recommendations, matching Render): first sight of a session as the sign-in (covers recovery; a Kratos `web_hook` would miss it), one row per current workspace, sign-out by report plus the CLI logout, expiry records none, a CLI device login is no `LoginEvent`. Production `whoami` keys (`id`, `authenticated_at`, `devices[].ip_address`/`user_agent`) checked as the QA user 2026-11-17. Written in [ADR012](../../../../docs/ADR012-auth.md) § 8c-1 and ADR006 § Audit log.
- **Deviations:** (1) the dashboard has no per-event label table (it shows Render's names as-is), so t004's label step needed no change; (2) the sign-in's origin is the request's (as every other row since w9/m450), not the Kratos device, which only stands in; (3) t006's cross-surface check ran in tests (`TestSignInEventsAgreeOnEverySurface`); the live comparison is part of t005; (4) no migration: dedupe is the derived audit id, not a new table.
- **Tests (t008):** `internal/signins` (first sight, per-replica and cross-replica dedupe, concurrent first requests, retry after a failed write, the window, sign-out rows, the mutation's caller check), `TestPGSignInsRecordOnceAcrossReplicas` (real Postgres; fails with two rows per workspace when ids are not derived), `TestAuthGateReportsEachSessionToTheSignInRecorder`, `TestDashboardSignInAndSignOutAreAudited`, `TestSignInEventsAgreeOnEverySurface`, `TestLogoutRecordsTheSignOut`, dashboard `report-sign-out` and `logout` tests.
- **Gate (t005):** clears by itself once production runs the w9/m452 commit (`git log` of the latest `chore(deploy): pin platform images to <sha>` on `origin/main` contains it). Then sign in to the dashboard as the QA user (`bash scripts/qa-login.sh` signs in through Kratos, which counts), browse, sign out from the dashboard, and read each of the QA user's workspaces' `GET /v1/owners/{ownerId}/audit-logs` and the CSV export: one `LoginEvent` and one `LogoutEvent` per workspace with `source_ip` and `user_agent`. Then t009.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w9` (2026-11-16), filed by `/pmx them all to w9`. Evidence:
  - [render.com/docs/audit-logs](https://render.com/docs/audit-logs) (read 2026-11-16): workspace member events include `LoginEvent` (member logged in) and `LogoutEvent` (member logged out); both are in the pinned enum.
  - bex: sign-in and sign-out happen entirely in Kratos; `deploy/gitops/base/values/kratos.values.yaml` has no `web_hook`, and recovery mints a session outside the after-login hooks; the dashboard signs out directly against Kratos (`dashboard/src/common/lib/ory/logout.ts`); bex-api sees a session only through `whoami` (`lego/backend/internal/api/auth.go`); the CLI logout goes through bex-api (`lego/backend/internal/cliauth/service.go`).
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first); sign-in history is the first evidence in a stolen-credential investigation and complements w22/m402 (enforced two-factor).
- **Expected outcome:** a workspace admin sees when and from where each member signed in and out, as on Render.
- **Why now:** depends on w9/m450's columns (t001 depends on `w9/m450/t002`); ordered after w9/m451 because it has the most open design points.
- **Decision state:** t001's choices are `/pm-brainstorm` **recommendations**, not user decisions, and matching Render is the default under ADR008: first sight of a Kratos session as the sign-in (alternative: a Kratos after-login `web_hook`); one row per current workspace; `LogoutEvent` from a dashboard call before the Kratos logout flow plus the CLI logout, with session expiry recording none; a CLI device-flow login is not a `LoginEvent` (unverified against Render).
- **Render parity task:** included. REST, GraphQL, the CSV export, the dashboard sign-out and labels change; MCP has no audit-log tool on Render or bex.
