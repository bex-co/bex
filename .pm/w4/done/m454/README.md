# w4 · m454 — Approving or revoking an AI agent's OAuth access is audited as GrantOAuthAccessEvent and RevokeOAuthAccessEvent

**Worker:** worker4 **Goal:** a member who approves an OAuth client (an MCP agent such as Claude Code, Codex or Cursor) or revokes it under Connected agents leaves Render's `GrantOAuthAccessEvent` / `RevokeOAuthAccessEvent` row in the workspace audit log, on REST, GraphQL, the dashboard and the CSV export **Status:** done (2026-11-17: the definition of done held on production, pin `5bcfe8939f5b`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Approving an OAuth client records GrantOAuthAccessEvent — **DONE** | 1h | — |
| [t002](done/t002.md) | Revoking a connected agent records RevokeOAuthAccessEvent — **DONE** | 45m | — |
| [t003](done/t003.md) | REST, GraphQL, CSV, dashboard labels and docs for the OAuth access events — **DONE** | 30m | t001, t002 |
| [t004](done/t004.md) | Production check: an MCP client approve then revoke leaves one row of each — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t008](done/t008.md) | Closeout — **DONE** | 15m | t006, t007 |

## Definition of done

- On production, as the QA user (`QA_EMAIL`) in a Pro workspace: authorizing an MCP client through the dashboard consent page records exactly one `GrantOAuthAccessEvent` in each workspace the user belongs to, naming the client (`metadata.client_id`, `metadata.client_name`) and the granted scopes, never a token; REST `GET /v1/owners/{ownerId}/audit-logs`, GraphQL `auditLogs` and the CSV export show it.
- Revoking that client under Settings → Security & Compliance → Connected agents records exactly one `RevokeOAuthAccessEvent` per workspace with the same client keys.
- A consent Hydra already remembers (`skip`) records nothing, and neither does a token refresh; the platform's own sign-in clients (the bex CLI device flow, the ops Grafana client) record no `GrantOAuthAccessEvent` (their sign-in belongs to w9/m452's `LoginEvent`/`LogoutEvent`).
- A forged `POST` to the grant route for a client the caller never approved records nothing (bex-api checks Hydra).
- The operator `revoke-oauth-grant` command records `RevokeOAuthAccessEvent` instead of `oauth.RevokeGrant`.
- A consent or revoke never fails because the audit write failed.
- ADR018's Audit logs row no longer says Render has no OAuth provider, and lists both events.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for one aspect of render parity` (2026-11-16, item 5), filed by `/pmx them all to w4`. Items 1–4 and item 5's sign-in half were already filed as w9/m449–m452 by another session the same day; this milestone is the remainder. Evidence:
  - Render's pinned OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`, `components.schemas.auditLog.properties.event`) lists `GrantOAuthAccessEvent` and `RevokeOAuthAccessEvent`. [render.com/docs/audit-logs](https://render.com/docs/audit-logs) (read 2026-11-16) does not describe them.
  - Render's MCP server has supported OAuth since 2026-07-22 ([changelog](https://render.com/changelog/render-mcp-now-supports-oauth-for-claude-code-codex-and-cursor)): the agent opens a browser, the user signs in to Render and authorizes account access. That is the action these events name.
  - bex: the consent accept runs in the dashboard SSR straight against the Hydra admin (`dashboard/src/common/server-fn/hydra-consent.ts:510` `acceptConsent`) and writes no audit row. Connected agents revoke (`dashboard/src/common/server-fn/hydra-connected-agents.ts:124`) revokes at Hydra, then calls bex-api `POST /v1/oauth/revocations` (`lego/backend/internal/cliauth/service.go:164` `recordRevocation`), which bumps the marker and writes no audit row. The operator path (`lego/backend/internal/oauthgrant/oauthgrant.go`) records `oauth.RevokeGrant` in the ops workspace.
  - [ADR018](../../../../docs/ADR018-render-parity.md)'s Audit logs row says "Render has no third-party OAuth provider", stale since Render's MCP OAuth.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) and bex's AI-native vision: an approved agent can act in every workspace its user belongs to, so "which agent was given access, and when" is the audit log's first question for an AI-native platform.
- **Expected outcome:** a workspace admin sees when each member approved or revoked an agent's access, as a Render admin does.
- **Why now:** OAuth-connected agents are bex's main AI-native entry point, and today granting one leaves no trace. w9/m449–m452 are editing the same `audit/service.go` `renderEvents` map and the dashboard labels; this milestone touches only the OAuth rows, so expect merge conflicts at most. It does not need w9/m450's `source_ip`/`user_agent`: once m450 lands these rows get them through its shared seam.
- **Decision state:** these are `/pm-brainstorm` **recommendations** under ADR008, not user decisions, and still need the user's sign-off:
  - mapping bex's Hydra consent accept and revoke to Render's two events. Render documents neither event's trigger or metadata, so a read-only capture from the reference account that shows otherwise replaces the mapping.
  - one row in each workspace the user belongs to, because an OAuth grant covers all of them. This is w9/m452's placement for `LoginEvent`.
  - excluding the platform's own sign-in clients.
  - metadata keys `client_id`, `client_name` and `scopes`, which are bex keys until Render's are captured.
- **Render parity task:** included. REST, GraphQL, the CSV export and the dashboard labels change. MCP has no audit-log tool on Render or bex.

## Unblock (2026-11-17, `/loopx w4` worker)

- **Affects:** t004 (production check), then t008 (closeout).
- **Source:** verified evidence. No user decision is involved.
- **Release gate: clear.** The newest `chore(deploy): pin platform images to` commit is `5e0fb5ad8` (09:41 UTC), pinning `5bcfe8939f5b`. `git merge-base --is-ancestor 3bb04c834 5bcfe8939f5b` exits 0.
- **w4/701: clear.** [w4/701](../701.md) closed in `523d1b819` (round 647). Re-checked at 09:53 UTC as the QA user: `GET /v1/owners/{ownerId}/audit-logs?limit=3` answers 200 in `bex` (`tea-d98210cbbpdc73dcrkvg`) and `bex-canary` (`tea-daif693dqjvc73e7as3g`), with rows from 09:51 UTC. The QA user's third workspace, `tian-personal` (`tea-da2isimlm39c739m4ofg`), answers 400 `PLAN_FEATURE_REQUIRED` because it is on Hobby. That is the plan gate, not the outage, so the replay reads the two Pro workspaces.
- **Next:** t004 replays the definition of done on production as the QA user. If it holds, t008 closes the milestone.

## Live check: DONE (2026-11-17, `/loopx w4` worker, pin `5bcfe8939f5b`)

The replay ran as the QA user. That user belongs to three workspaces: `bex` (`tea-d98210cbbpdc73dcrkvg`) and `bex-canary` (`tea-daif693dqjvc73e7as3g`) are on Pro, and `tian-personal` is on Hobby. Hobby answers audit-log reads with `PLAN_FEATURE_REQUIRED`, so the rows were read in the two Pro workspaces. Every definition-of-done item that production can show holds:

- **Grant.**
  - What ran: a DCR public client `qa-m454-0954` (`54b86f42-…`, audience `https://api.bex.co/mcp`, scopes `openid offline_access bex.read`), authorize with PKCE S256, then approve on `dashboard.bex.co/auth/consent`. The page named the client and listed `bex.read`, and its approval returned a code that exchanged for a token.
  - Result: exactly one `GrantOAuthAccessEvent` at 09:57:21 UTC in each Pro workspace: `aud-60ob6pk85t14mfhrj0o9` in `bex` and `aud-sed2kbrtfl9mdp3i5ghv` in `bex-canary`.
  - REST `metadata` keys are `client_id`, `client_name` (`qa-m454-0954`), `scopes` (`openid offline_access bex.read`), `source_ip` and `user_agent`. The actor is the QA user, and no row carries a token.
- **Nothing extra.** None of these added a row:
  - a second authorize, where the consent route 302'd headlessly with no page;
  - a `refresh_token` grant;
  - a replayed `POST /v1/oauth/grants` for the approved client, which answered 204;
  - a forged `POST /v1/oauth/grants` for a second DCR client the user never approved, which answered `404 recent OAuth grant not found`;
  - a `bex login`-shaped device flow for the bex CLI client (`429024F5…`, `skip_consent`) at 09:56:35–09:56:50.
- **Revoke.** Revoke was clicked on the client's row under Settings → Security → Connected agents and confirmed in the dialog. The row left the list, and the client's access token answered 401 at once. Result: exactly one `RevokeOAuthAccessEvent` at 09:58:55 UTC in each Pro workspace: `aud-dbu2anovchns73divjs0` in `bex` and `aud-dbu2anovchns73divjt0` in `bex-canary`, with `client_id`, `client_name`, `source_ip` and `user_agent`.
- **Surfaces.**
  - GraphQL `auditLogs` returns the same four rows: `action` `GrantOAuthAccessEvent` or `RevokeOAuthAccessEvent`, `resource` `oauth-client:54b86f42-…`, `targetName` `qa-m454-0954`, `oauthGrantScopes` `[openid, offline_access, bex.read]` on the grant, and `actorMethod` `session`.
  - The `bex` Audit Log card lists both rows, with the client name over `oauth-client:… · Scopes openid offline_access bex.read`.
  - The card's "Export as CSV" for 2026-11-17 carries both rows. The grant's metadata JSON holds `resource`, `targetName`, `oauthGrantScopes`, `actorMethod`, `source_ip` and `user_agent`.
- **Not live-checkable here:**
  - the operator `revoke-oauth-grant` verb (`oauthgrant.go:235` records `core.AuditVerbRevokeOAuthAccess`) and the fail-open writes. Tests cover both (t002, t007).
  - the ADR018 Audit logs row, which lists both events and no longer says Render has no OAuth provider.
- **Cleanup.** Both DCR clients were deleted through their registration URIs (204). The CLI device refresh token was revoked, and its access token answers 401. The run's three Kratos sessions were logged out.
