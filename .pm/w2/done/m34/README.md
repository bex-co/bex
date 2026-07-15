# w2 · m34 — Fix GitHub Connect: browser install-callback can't record the connection

**Worker:** worker2 **Goal:** let a normal signed-in dashboard user complete a GitHub App connection through the browser, not just via a Bearer-token API/agent call **Status:** done (all tasks complete; live browser and Bearer paths verified in production)

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | `StartConnect`: mint a short-lived HMAC-signed `state` param (encodes `workspaceID` + expiry) appended to the returned `installUrl` | 1h | — | — **DONE** |
| t002 | Carve a narrow, well-commented auth-gate exception for `GET /v1/git/callback` (first deliberate exception to `internal/api/auth.go`'s "every route sits behind the same gate" invariant) | 1h | — | — **DONE** |
| t003 | Callback handler: verify `state` (HMAC + expiry + tamper check), reject with a clear error on missing/invalid/expired state | 1h | t001, t002 | — **DONE** |
| t004 | New workspace-scoped `Connect` entry point in `github/service.go` that resolves `workspaceID` from the verified state instead of `s.Tenant(ctx)` | 1h | t003 | — **DONE** |
| t005 | Dashboard: on callback failure, redirect to `/settings` with a visible error state instead of leaving the user on a bare API error page | 1h | t003 | — **DONE** |
| t006 | Update `docs/ADR026-github-integration.md`'s "Known limitation" section to describe the implemented mechanism; note the auth-gate exception in `lego/backend/internal/api/CLAUDE.md` if it states the blanket-gate invariant | 30m | t004 | — **DONE** |
| t007 | Render parity: verify the connect flow now works consistently for both the browser (dashboard) and API/agent (Bearer) paths | 30m | t005, t006 | — **DONE** |
| t008 | Simplify | 30m | t007 | — **DONE** |
| t009 | Test coverage | 1h | t007 | — **DONE** |
| t010 | Closeout | 15m | t009 | — **DONE** |

## Definition of done

A normal signed-in dashboard user can click "Connect GitHub," complete the GitHub App install flow in the browser, and land back on `/settings` with the connection recorded and visible — verified live end-to-end, not via the Bearer-token workaround the current DoD relies on.

## Implementation evidence (2026-07-14)

- `StartConnect` now returns a 15-minute HMAC-SHA256 state token bound to the authorized workspace; callback verification covers valid, missing, tampered, expired, and oversized values. The GitHub App private key's existing PEM bytes are the shared signing key, so no new secret rollout is required; the production API manifest now sets the already-supported `BEX_DASHBOARD_URL=https://dashboard.bex.co` redirect origin.
- The auth gate bypass is exact to an otherwise-uncredentialed `GET /v1/git/callback`; method/path neighbors stay 401, while supplied Bearer/session credentials retain the original authorized `Connect` path. A full-handler test proves authenticated start → anonymous signed callback → connection persisted.
- State callbacks persist against the encoded workspace, validate `installation_id` against GitHub before writing, and redirect failures to bounded `/settings?git_error=…` values with `Referrer-Policy: no-referrer`. The dashboard renders localized, non-reflective retry guidance.
- Current official parity evidence matches the implemented browser shape: [GitHub's installation flow](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party) selects an account and repository grants before returning through the app's setup URL; [Render redirects to GitHub and then back to a connected repo list](https://render.com/docs/github), and uses the same `installations/new` route to manage an existing installation. No bex-specific behavior drift remains.
- Live browser evidence is green across the bex boundary: the normal signed-in production Settings UI disconnected, invoked **Connect GitHub**, reached the real `bex-co` organization installation, completed the UI-minted signed callback, returned to `/settings`, and visibly showed **Connected as bex-co**. A forged-state browser navigation returned to the styled card with bounded retry guidance. GitHub's existing-installation sudo prompt prevented an unnecessary repository-grant edit; it is provider behavior shared by Render's management flow, not a bex callback failure.
- The compatibility fix redirects only state/browser callbacks; a full-handler regression test configures `DashboardURL`, requires the signed browser redirect, then requires `200 {"status":"connected"}` with no redirect for the authenticated path on the same server.
- Green verification on the final combined tree: backend `go test ./...`, `go build ./...`, and `make lint-backend` (0 issues); operator `make test` (including envtest); dashboard `yarn typecheck`, `yarn lint`, `yarn test` (177 files / 1059 tests), and `yarn build`; exact Markdown Prettier check; `git diff --check`.
- Commit `46709508` deployed successfully through GitHub Actions run `29391435953`: all tests, image builds, signing, SBOM attestation, CVE gates, GitOps digest write-back, and production rollouts passed.
- Final production Bearer replay passed: installation `90623475` returned HTTP `200`, `Content-Type: application/json`, no redirect, and `{"status":"connected"}`; sanitized connection state before and after remained `connected:true`, `accountLogin:"bex-co"`, `installationId:90623475`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 (fifth pass) — `docs/ADR026-github-integration.md` line 88 ("Known limitation — the browser install-callback needs a bex-api session"); code-level confirmation in `lego/backend/internal/api/auth.go` (`hasSessionCredential`/`hasBearer` both false on GitHub's redirect) and `lego/backend/internal/github/{service,rest}.go` (no `state` param exists today). Discovered live during `w2/m8`'s DoD run 2026-07-12, deferred without a follow-up milestone until now.
- **Goal linkage:** completes the GitHub integration `w2/m8`/`m9` shipped — a broken primary UX path for a headline feature (private-repo deploys, push-to-deploy) is not real parity.
- **Expected outcome:** the GitHub connect flow works for actual dashboard users, not just API/agent callers.
- **Why now:** this is a live, user-facing dead end on an already-shipped feature, not speculative work — the fix direction is already designed in the ADR, just never built. Render parity included — the Connect capability must work consistently for both the browser (dashboard) and API/agent paths; today only one does.
