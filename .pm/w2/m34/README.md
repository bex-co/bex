# w2 · m34 — Fix GitHub Connect: browser install-callback can't record the connection

**Worker:** worker2 **Goal:** let a normal signed-in dashboard user complete a GitHub App connection through the browser, not just via a Bearer-token API/agent call **Status:** todo (t001–t006 done; t007 live verification pending)

## Tasks (in order)

| id   | title                                                                                                    | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `StartConnect`: mint a short-lived HMAC-signed `state` param (encodes `workspaceID` + expiry) appended to the returned `installUrl` | 1h  | —          | — **DONE** |
| t002 | Carve a narrow, well-commented auth-gate exception for `GET /v1/git/callback` (first deliberate exception to `internal/api/auth.go`'s "every route sits behind the same gate" invariant) | 1h  | —          | — **DONE** |
| t003 | Callback handler: verify `state` (HMAC + expiry + tamper check), reject with a clear error on missing/invalid/expired state | 1h  | t001, t002 | — **DONE** |
| t004 | New workspace-scoped `Connect` entry point in `github/service.go` that resolves `workspaceID` from the verified state instead of `s.Tenant(ctx)` | 1h  | t003       | — **DONE** |
| t005 | Dashboard: on callback failure, redirect to `/settings` with a visible error state instead of leaving the user on a bare API error page | 1h  | t003       | — **DONE** |
| t006 | Update `docs/ADR026-github-integration.md`'s "Known limitation" section to describe the implemented mechanism; note the auth-gate exception in `lego/backend/internal/api/CLAUDE.md` if it states the blanket-gate invariant | 30m | t004       | — **DONE** |
| t007 | Render parity: verify the connect flow now works consistently for both the browser (dashboard) and API/agent (Bearer) paths | 30m | t005, t006 |
| t008 | Simplify                                                                                                    | 30m | t007       |
| t009 | Test coverage                                                                                               | 1h  | t007       |
| t010 | Closeout                                                                                                    | 15m | t009       |

## Definition of done

A normal signed-in dashboard user can click "Connect GitHub," complete the GitHub App install flow in the browser, and land back on `/settings` with the connection recorded and visible — verified live end-to-end, not via the Bearer-token workaround the current DoD relies on.

## Implementation evidence (2026-07-14; live DoD pending)

- `StartConnect` now returns a 15-minute HMAC-SHA256 state token bound to the authorized workspace; callback verification covers valid, missing, tampered, expired, and oversized values. The GitHub App private key's existing PEM bytes are the shared signing key, so no new secret rollout is required; the production API manifest now sets the already-supported `BEX_DASHBOARD_URL=https://dashboard.bex.co` redirect origin.
- The auth gate bypass is exact to an otherwise-uncredentialed `GET /v1/git/callback`; method/path neighbors stay 401, while supplied Bearer/session credentials retain the original authorized `Connect` path. A full-handler test proves authenticated start → anonymous signed callback → connection persisted.
- State callbacks persist against the encoded workspace, validate `installation_id` against GitHub before writing, and redirect failures to bounded `/settings?git_error=…` values with `Referrer-Policy: no-referrer`. The dashboard renders localized, non-reflective retry guidance.
- Current official parity evidence matches the implemented browser shape: [GitHub preserves `state` through App installation](https://docs.github.com/en/apps/sharing-github-apps/sharing-your-github-app); [Render redirects to GitHub and then back to a connected repo list](https://render.com/docs/github). No remaining behavior drift was identified in the source comparison.
- Green verification: backend `go test ./...`, `go build ./...`, and `make lint-backend` (0 issues); operator `make test` (including envtest) plus a kustomize render assertion for `BEX_DASHBOARD_URL`; dashboard `yarn typecheck`, `yarn lint`, `yarn test` (158 files / 967 tests), and `yarn build`; `git diff --check`; repository Markdown Prettier pass.
- Live environment audit: `bex.co` and `dashboard.bex.co` return 200 and `api.bex.co` is reachable, but an uncredentialed production callback still returns `401` with the OAuth protected-resource challenge—the exact pre-fix behavior. A non-mutating production Bearer `StartConnect` check is green (authorized existing connection + install URL), and its missing `state` further proves the old build is live. The recovered local CAPD cluster has no bex API/dashboard workloads or route, so it cannot stand in for the GitHub App's production Setup URL.
- **Remaining gate:** after an explicit `$ship` authorizes commit/push and the build is deployed, complete the real dashboard → GitHub install → callback → `/settings` browser round trip plus the live Bearer-path regression in [t007](t007.md). No production deployment or GitHub App configuration was mutated during this implementation run.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 (fifth pass) — `docs/ADR026-github-integration.md` line 88 ("Known limitation — the browser install-callback needs a bex-api session"); code-level confirmation in `lego/backend/internal/api/auth.go` (`hasSessionCredential`/`hasBearer` both false on GitHub's redirect) and `lego/backend/internal/github/{service,rest}.go` (no `state` param exists today). Discovered live during `w2/m8`'s DoD run 2026-07-12, deferred without a follow-up milestone until now.
- **Goal linkage:** completes the GitHub integration `w2/m8`/`m9` shipped — a broken primary UX path for a headline feature (private-repo deploys, push-to-deploy) is not real parity.
- **Expected outcome:** the GitHub connect flow works for actual dashboard users, not just API/agent callers.
- **Why now:** this is a live, user-facing dead end on an already-shipped feature, not speculative work — the fix direction is already designed in the ADR, just never built. Render parity included — the Connect capability must work consistently for both the browser (dashboard) and API/agent paths; today only one does.
