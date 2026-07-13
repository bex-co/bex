# Connect an agent — Claude Code / Cursor → bex `/mcp` over OAuth 2.1

bex-api serves a Render-compatible MCP server at `/mcp` (streamable-HTTP), the same one-core-three-adapters surface as REST/GraphQL ([ADR006-bex-api.md](ADR006-bex-api.md)). w4/m9 added the OAuth 2.1 provider side ([ADR012-auth.md §7](ADR012-auth.md#7-oauth-21-provider-for-agents-one-dashboard-login-first-party-sessions--third-party-clients-w4m9)) so a real MCP client — Claude Code, Cursor, or your own agent — can hold its own revocable, user-consented credential instead of a shared secret. This page is the connect recipe: what the discovery/token dance looks like, config snippets for both clients, the API-key path for headless use, and what to check when it doesn't come up clean.

## What happens on first connect (RFC 9728 + DCR + PKCE)

1. The agent calls `POST/GET https://api.bex.co/mcp` with no credential and gets **401** with `WWW-Authenticate: Bearer resource_metadata="https://api.bex.co/.well-known/oauth-protected-resource"` — only present when the deployment sets `BEX_OAUTH_ISSUER`/`BEX_OAUTH_RESOURCE` (see [Troubleshooting](#troubleshooting) if it isn't).
2. It fetches that metadata URL and gets back `{resource, authorization_servers: ["<issuer>"], bearer_methods_supported: ["header"]}` — the RFC 9728 hint that points it at Hydra as the authorization server.
3. It **self-registers** as an OAuth2 client via Hydra's public Dynamic Client Registration (`POST /oauth2/register`) — no manual client setup on either side.
4. It opens a browser for an **authorization-code + PKCE (S256)** flow. The login screen is bex's own dashboard, backed by Kratos. Then you see a **consent screen** naming the agent and the scopes it asked for; approve it and the flow continues, deny it and the agent gets `error=access_denied` (w4/m16, [ADR012-auth.md §7](ADR012-auth.md#7-oauth-21-provider-for-agents-one-dashboard-login-first-party-sessions--third-party-clients-w4m9)). The approval is remembered for an hour, so a re-authorization inside that window is a redirect with no UI. Clients the platform operator has blessed (`skip_consent`, or listed in `OAUTH_TRUSTED_CLIENTS`) skip the screen entirely — that's an optional convenience for first-party agents, **not** a prerequisite for connecting.
5. The agent exchanges the code for an access + refresh token at Hydra's public `/oauth2/token` and attaches `Authorization: Bearer <token>` to every `/mcp` call from then on. bex-api introspects it at Hydra (cached ≤30s), checks the audience when present, and authorizes the call through OpenFGA exactly like a REST/GraphQL caller — same identity, same permissions.

None of this is bex-specific glue: it's the standard MCP authorization spec's discovery flow, so any MCP client that implements it (not just the two below) connects the same way.

## Claude Code

```bash
claude mcp add --transport http bex-api https://api.bex.co/mcp
```

Claude Code auto-discovers OAuth from the 401's `resource_metadata` hint — no extra flags. Trigger the login from inside a session with `/mcp`, or ahead of time with `claude mcp login bex-api`; both open a browser to the dashboard login (a headless shell prints the URL to open elsewhere and paste the redirect back).

For CI or another headless caller, skip OAuth entirely and pass a static [API key](#the-api-key-alternative-headless--ci-any-client) as a bearer header instead:

```bash
claude mcp add --transport http bex-api https://api.bex.co/mcp \
  --header "Authorization: Bearer ${BEX_TOKEN}"
```

## Cursor

Cursor's MCP client doesn't do RFC 9728 auto-discovery — it takes a credential at config time. Edit `~/.cursor/mcp.json` (global) or `.cursor/mcp.json` (project):

```json
{
  "mcpServers": {
    "bex-api": {
      "url": "https://api.bex.co/mcp",
      "headers": {
        "Authorization": "Bearer ${env:BEX_TOKEN}"
      }
    }
  }
}
```

`BEX_TOKEN` here is an [API-key exchange](#the-api-key-alternative-headless--ci-any-client) token, refreshed by hand or by a wrapper script. bex's side of the OAuth dance is complete (discovery → DCR → login → consent screen), but Cursor's MCP config is header-based rather than discovery-driven, so it never starts that flow — the API-key path stays the practical one for this client until it does.

## The API-key alternative (headless / CI, any client)

An API key _is_ a Hydra `client_credentials` OAuth2 client bex minted — mint one from the dashboard (Settings → API Keys) or `POST /v1/api-keys` (the secret is returned exactly once), then exchange it for a short-lived (15m) bearer token whenever you need one ([ADR006-bex-api.md#auth](ADR006-bex-api.md#auth)):

```sh
TOKEN=$(curl -s -X POST https://oauth.bex.co/oauth2/token \
  -d "grant_type=client_credentials&client_id=$KEY_ID&client_secret=$KEY_SECRET" | yq .access_token)
```

It hits the same `/mcp` introspection and OpenFGA authorization path a user-consented token does; the one difference is an empty `aud`, which bex-api accepts by design so plain API keys keep working ([ADR012-auth.md §7](ADR012-auth.md#7-oauth-21-provider-for-agents-one-dashboard-login-first-party-sessions--third-party-clients-w4m9)).

## Local dev: stdio, no auth at all

`api mcp-stdio` (or `BEX_MCP_STDIO=1`) serves the same MCP tools over stdin/stdout for an agent launched on the same host — its trust boundary is the subprocess itself, so no bearer, no OAuth dance, no `BEX_HYDRA_ADMIN_URL` needed. Claude Code: `claude mcp add bex-api-local -- api mcp-stdio` (stdio is the default transport, so no `--transport` flag).

## Troubleshooting

- **401 with a bare `WWW-Authenticate: Bearer` (no `resource_metadata`), or the `.well-known` URL 404s** — this deployment hasn't set `BEX_OAUTH_ISSUER`/`BEX_OAUTH_RESOURCE`, so there's nothing for a discovery-driven client to find. Nothing to fix agent-side: either ask the platform operator to set both, or use the [API-key path](#the-api-key-alternative-headless--ci-any-client) instead, which needs neither.
- **503 on `/mcp` (or any route)** — Hydra or Kratos is unreachable; bex-api fails closed rather than falling through to an unauthenticated response ([ADR012-auth.md](ADR012-auth.md)). This is an operational outage, not a client misconfiguration — retry once the platform is back.
- **`error=access_denied` back at the agent** — the consent screen was denied (by you, or by someone closing the tab on it). Re-run the connect and approve it; nothing needs fixing platform-side. Note the flow also denies when the browser that answers consent isn't the one that logged in (the session must own the authorization) — if you signed in as a different user mid-flow, sign out and start over.
- **The consent screen 503s ("consent provider not configured")** — the dashboard has no `HYDRA_ADMIN_URL`, so it can't reach Hydra's admin API to complete the grant. Operator fix, not a client one.
- **The login page asks you to confirm your password even though you're already signed in** — expected on your _first_ authorization. Kratos accepts each login challenge without asking Hydra to remember the login, so Hydra has no session for the browser yet and its login request is not skippable; Kratos answers with a re-authentication flow ("confirm that it is you"). Confirm once — subsequent authorizations pass through with no UI at all ([ADR012-auth.md §7](ADR012-auth.md#7-oauth-21-provider-for-agents-one-dashboard-login-first-party-sessions--third-party-clients-w4m9)).
- **The login page sits on a loading skeleton and the connect never completes** — a dashboard from before w4/m17. Kratos answers the signed-in browser's flow request with `200 null` (it has already accepted the challenge and there is nothing to render), which the login page rendered as "no flow yet", forever. Fixed by handing the challenge back to Kratos as a browser navigation; if you see this, the dashboard needs updating — signing out first is the workaround.
- **You want an agent to skip the consent screen entirely** (a first-party CI agent, say) — the operator blesses its `client_id` once: Hydra admin `PATCH` → `skip_consent: true`, or add it to the dashboard's `OAUTH_TRUSTED_CLIENTS`. That is purely a convenience; unblessed clients connect fine, they just ask the user first.
- **Token works once, then 401s a few minutes later** — expected: access tokens are deliberately short-lived (15m, `ADR012-auth.md §8`). An OAuth-flow token carries a refresh token your client should use silently; an API-key token has none — just re-run the `client_credentials` exchange.
