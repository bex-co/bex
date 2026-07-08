# w4 · m9 — OAuth 2.1 provider for agents: one dashboard login, first-party sessions + third-party clients

**Worker:** worker4 **Goal:** The existing `dashboard/` auth module becomes bex's single identity surface serving **both** sides: (a) first-party — the dashboard itself keeps logging in via Kratos exactly as today (HttpOnly session cookie, no OAuth tokens in the browser); (b) third-party — an OAuth 2.1 client (e.g. the Claude Code agent over MCP) registers via **DCR**, runs auth-code + PKCE against Hydra, and lands on the **same dashboard login page**, where Kratos's **native `oauth2_provider` integration** accepts the login challenge itself — no custom login provider anywhere. A headless consent acceptor grants implicit consent for trusted clients; bex-api's already-live Hydra introspection (m2) validates the resulting Bearer tokens. **Status:** done — verified end-to-end locally against real Hydra + real Kratos (native bridge) + the real dashboard consent route + real bex-api, including a real-browser pass through the actual React login page (`scripts/auth-oauth21-e2e.sh` + Playwright); all gates green (go 9 pkgs, dashboard 320 tests + build).

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on             |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------------------- |
| t001 | Kratos↔Hydra native bridge: `oauth2_provider.url` in kratos.values + Hydra `urls.login/consent` (base + local) | 45m | —                      | — **DONE** |
| t002 | Dashboard/auth passes `login_challenge` through (login + sign-up flows); first-party login byte-identical | 45m | w4/m9/t001             | — **DONE** |
| t003 | Headless consent acceptor as a dashboard server route (auto-accept trusted/`skip_consent`, deny unknown) | 60m | w4/m9/t001             | — **DONE** |
| t004 | Agent discovery: Hydra DCR enabled + bex-api RFC 9728 protected-resource metadata for `/mcp`            | 75m | w4/m9/t001             | — **DONE** |
| t005 | E2E verify (DCR → PKCE → dashboard Kratos login → consent → Bearer → `/mcp`) + docs/auth.md §7          | 60m | w4/m9/t002, w4/m9/t003, w4/m9/t004 | — **DONE** |
| t006 | Simplify — `/simplify` over the code this milestone changed                                             | 30m | w4/m9/t005             | — **DONE** |
| t007 | Test coverage — challenge passthrough, consent accept/deny, resource metadata                           | 45m | w4/m9/t005             | — **DONE** |

## Definition of done

- A **DCR-registered** OAuth 2.1 client completes auth-code + PKCE against Hydra where the login step is the **existing dashboard login page**: the user gets/reuses a Kratos session there and **Kratos itself accepts the login challenge** (`oauth2_provider` native integration) — no hand-built login provider in bex-api or the dashboard.
- Trusted/`skip_consent` clients receive **implicit consent** via the headless acceptor; an unknown client is denied (a real consent UI is future work, not this milestone).
- The issued access token authorizes bex-api calls (`/mcp`, `/graphql`) via `Authorization: Bearer` and passes the already-live Hydra introspection (m2) — no bex-api auth-gate changes.
- **First-party unchanged:** the dashboard still authenticates with its Kratos session cookie; no OAuth access/refresh tokens are ever stored browser-side (`DO_NOT_DO.md` holds — this milestone touches the login page only to forward `login_challenge`).
- bex-api's `/mcp` advertises **RFC 9728 protected-resource metadata** (`/.well-known/oauth-protected-resource` + `WWW-Authenticate: … resource_metadata=` on 401) pointing at the Hydra issuer, so an MCP client (Claude Code) can discover the authorization server per the MCP authorization spec.
- **RFC 8707 audience discipline (MCP MUST):** Hydra's handling of the `resource` parameter is verified and recorded, and bex-api validates the introspected token's audience when the OAuth issuer is configured — a token minted for another resource does not authorize `/mcp`.
- The full flow is verified end-to-end locally (scripted/documented), and the prod values (kratos/hydra base overlays) are wired.

## Source + Goal linkage

- **Source:** promoted from inbox note `w4/001` (→ `done/005.md`; originally deferred from the reverted, never-committed dashboard-as-OAuth2-client attempt of 2026-07-07) + user directive 2026-07-07: "reuse the existing dashboard/auth module — Kratos login for the dashboard itself, and third-party clients like the Claude Code agent via OAuth 2.1". Research basis: Kratos native `oauth2_provider` integration (Kratos accepts Hydra login challenges itself); IETF `draft-ietf-oauth-browser-based-apps`; Ory guidance (first-party = Kratos sessions, Hydra = third-party/machine); MCP authorization spec (OAuth 2.1 + DCR + RFC 9728).
- **Goal linkage:** w4 MISSION-IAM (identity for bex) × `docs/vision.md` pillars 3–5 (agent-native): agents become first-class OAuth clients instead of holders of hand-minted API keys. Reuses m1 (Hydra+Kratos live) and m2 (introspection live).
- **Expected outcome:** a Claude-Code-shaped MCP client can self-register (DCR), send its user through bex's own login page once, and call bex-api with a user-consented Bearer token — while the dashboard's own auth remains byte-identical Kratos sessions.
- **Why now:** the MCP server (w2/m1) is live but every agent needs a manually provisioned API key; the MCP authorization spec standardizes exactly this flow; the substrate (m1/m2) is deployed; and the reverted first attempt already produced the design research — the corrected path is cheap and unblocks w2's agent story.

## Design guardrails (from DO_NOT_DO)

- The dashboard **never** becomes an OAuth2 client of Hydra; browser-held tokens stay banned. This milestone is the **provider** side only.
- No custom login provider: Kratos's `oauth2_provider.url` owns the login challenge. The only hand-built piece is the headless consent acceptor (explicitly allowed), which lives in the dashboard's server layer per the "dashboard/auth is the IdP surface" decision — its Hydra-admin URL is SSR-only env, never in the client bundle.
