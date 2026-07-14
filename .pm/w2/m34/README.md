# w2 · m34 — Fix GitHub Connect: browser install-callback can't record the connection

**Worker:** worker2 **Goal:** let a normal signed-in dashboard user complete a GitHub App connection through the browser, not just via a Bearer-token API/agent call **Status:** todo

## Tasks (in order)

| id   | title                                                                                                    | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `StartConnect`: mint a short-lived HMAC-signed `state` param (encodes `workspaceID` + expiry) appended to the returned `installUrl` | 1h  | —          |
| t002 | Carve a narrow, well-commented auth-gate exception for `GET /v1/git/callback` (first deliberate exception to `internal/api/auth.go`'s "every route sits behind the same gate" invariant) | 1h  | —          |
| t003 | Callback handler: verify `state` (HMAC + expiry + tamper check), reject with a clear error on missing/invalid/expired state | 1h  | t001, t002 |
| t004 | New workspace-scoped `Connect` entry point in `github/service.go` that resolves `workspaceID` from the verified state instead of `s.Tenant(ctx)` | 1h  | t003       |
| t005 | Dashboard: on callback failure, redirect to `/settings` with a visible error state instead of leaving the user on a bare API error page | 1h  | t003       |
| t006 | Update `docs/ADR026-github-integration.md`'s "Known limitation" section to describe the implemented mechanism; note the auth-gate exception in `lego/backend/internal/api/CLAUDE.md` if it states the blanket-gate invariant | 30m | t004       |
| t007 | Render parity: verify the connect flow now works consistently for both the browser (dashboard) and API/agent (Bearer) paths | 30m | t005, t006 |
| t008 | Simplify                                                                                                    | 30m | t007       |
| t009 | Test coverage                                                                                               | 1h  | t007       |
| t010 | Closeout                                                                                                    | 15m | t009       |

## Definition of done

A normal signed-in dashboard user can click "Connect GitHub," complete the GitHub App install flow in the browser, and land back on `/settings` with the connection recorded and visible — verified live end-to-end, not via the Bearer-token workaround the current DoD relies on.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 (fifth pass) — `docs/ADR026-github-integration.md` line 88 ("Known limitation — the browser install-callback needs a bex-api session"); code-level confirmation in `lego/backend/internal/api/auth.go` (`hasSessionCredential`/`hasBearer` both false on GitHub's redirect) and `lego/backend/internal/github/{service,rest}.go` (no `state` param exists today). Discovered live during `w2/m8`'s DoD run 2026-07-12, deferred without a follow-up milestone until now.
- **Goal linkage:** completes the GitHub integration `w2/m8`/`m9` shipped — a broken primary UX path for a headline feature (private-repo deploys, push-to-deploy) is not real parity.
- **Expected outcome:** the GitHub connect flow works for actual dashboard users, not just API/agent callers.
- **Why now:** this is a live, user-facing dead end on an already-shipped feature, not speculative work — the fix direction is already designed in the ADR, just never built. Render parity included — the Connect capability must work consistently for both the browser (dashboard) and API/agent paths; today only one does.
