# w9 · m452 — Members' sign-ins and sign-outs are audited as LoginEvent and LogoutEvent

**Worker:** worker9 **Goal:** a member's dashboard sign-in and sign-out each leave Render's `LoginEvent`/`LogoutEvent` row, with source IP and user agent, in every workspace they belong to **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Decide how bex-api learns of a sign-in and a sign-out | 45m | w9/m450/t002 |
| [t002](t002.md) | Record LoginEvent once per new session in each of the member's workspaces | 1h 30m | t001 |
| [t003](t003.md) | Record LogoutEvent from the dashboard sign-out and the CLI logout | 1h | t001 |
| [t004](t004.md) | REST, GraphQL, CSV, dashboard labels and docs for the sign-in events | 30m | t002, t003 |
| [t005](t005.md) | Production check: a QA sign-in and sign-out each leave one row per workspace | 30m | t004 |
| [t006](t006.md) | Render parity | 30m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006 |
| [t009](t009.md) | Closeout | 15m | t007, t008 |

## Definition of done

- On production, signing in to the dashboard as the QA user records exactly one `LoginEvent` in each workspace the user belongs to, timestamped at the sign-in and carrying `source_ip` and `user_agent`; further requests on that session record no more, and two bex-api replicas seeing the new session at once still record one row per workspace.
- A sign-in through account recovery records one too.
- Signing out from the dashboard records one `LogoutEvent` per workspace.
- REST, GraphQL and the CSV export show both; a Hobby workspace records them and its admin sees the existing plan notice.
- A sign-in or sign-out never waits on, or fails because of, the audit write.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w9` (2026-11-16), filed by `/pmx them all to w9`. Evidence:
  - [render.com/docs/audit-logs](https://render.com/docs/audit-logs) (read 2026-11-16): workspace member events include `LoginEvent` (member logged in) and `LogoutEvent` (member logged out); both are in the pinned enum.
  - bex: sign-in and sign-out happen entirely in Kratos; `deploy/gitops/base/values/kratos.values.yaml` has no `web_hook`, and recovery mints a session outside the after-login hooks; the dashboard signs out directly against Kratos (`dashboard/src/common/lib/ory/logout.ts`); bex-api sees a session only through `whoami` (`lego/backend/internal/api/auth.go`); the CLI logout goes through bex-api (`lego/backend/internal/cliauth/service.go`).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); sign-in history is the first evidence in a stolen-credential investigation and complements w22/m402 (enforced two-factor).
- **Expected outcome:** a workspace admin sees when and from where each member signed in and out, as on Render.
- **Why now:** depends on w9/m450's columns (t001 depends on `w9/m450/t002`); ordered after w9/m451 because it has the most open design points.
- **Decision state:** t001's choices are `/pm-brainstorm` **recommendations**, not user decisions, and matching Render is the default under ADR008: first sight of a Kratos session as the sign-in (alternative: a Kratos after-login `web_hook`); one row per current workspace; `LogoutEvent` from a dashboard call before the Kratos logout flow plus the CLI logout, with session expiry recording none; a CLI device-flow login is not a `LoginEvent` (unverified against Render).
- **Render parity task:** included. REST, GraphQL, the CSV export, the dashboard sign-out and labels change; MCP has no audit-log tool on Render or bex.
