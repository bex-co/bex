# Connect an agent — Claude Code / Cursor → bex `/mcp` over OAuth 2.1

bex-api serves a Render-compatible MCP server at `/mcp` (streamable-HTTP), the same one-core-three-adapters surface as REST/GraphQL ([ADR006-bex-api.md](ADR006-bex-api.md)). w4/m9 added the OAuth 2.1 provider side ([ADR012-auth.md §7](ADR012-auth.md#7-oauth-21-provider-for-agents-one-dashboard-login-first-party-sessions--third-party-clients-w4m9)) so a real MCP client — Claude Code, Cursor, or your own agent — can hold its own revocable, user-consented credential instead of a shared secret. This page is the connect recipe: what the discovery/token dance looks like, config snippets for both clients, the API-key path for headless use, and what to check when it doesn't come up clean.

## What happens on first connect (RFC 9728 + DCR + PKCE)

1. The agent calls `POST/GET https://api.bex.co/mcp` with no credential and gets **401** with `WWW-Authenticate: Bearer resource_metadata="https://api.bex.co/.well-known/oauth-protected-resource"` — only present when the deployment sets `BEX_OAUTH_ISSUER`/`BEX_OAUTH_RESOURCE` (see [Troubleshooting](#troubleshooting) if it isn't).
2. It fetches that metadata URL and gets back `{resource, authorization_servers: ["<issuer>"], bearer_methods_supported: ["header"]}` — the RFC 9728 hint that points it at Hydra as the authorization server.
3. It **self-registers** as an OAuth2 client via Hydra's public Dynamic Client Registration (`POST /oauth2/register`) — no manual client setup on either side.
4. It opens a browser for an **authorization-code + PKCE (S256)** flow. The login screen is bex's own dashboard, backed by Kratos — if you're already logged into the dashboard, this is a redirect with no UI at all. Consent auto-accepts for a client the platform operator has blessed (`skip_consent`, or listed in `OAUTH_TRUSTED_CLIENTS`); an **unrecognized self-registered client is denied by default** (see [Troubleshooting](#troubleshooting)).
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

`BEX_TOKEN` here is an [API-key exchange](#the-api-key-alternative-headless--ci-any-client) token, refreshed by hand or by a wrapper script — there's no interactive consent screen on the Cursor side today, so the API-key path is the practical one for this client.

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
- **Consent denied right after login, even though you approved it in the browser** — the dashboard's headless consent route only auto-accepts a client Hydra marks skippable or one listed in `OAUTH_TRUSTED_CLIENTS`; a freshly self-registered agent client doesn't start in either bucket, so it's denied by design. The platform operator needs to bless the client once (Hydra admin `PATCH` on the registered `client_id` → `skip_consent: true`, or add it to `OAUTH_TRUSTED_CLIENTS`) before end users can consent to it.
- **Token works once, then 401s a few minutes later** — expected: access tokens are deliberately short-lived (15m, `ADR012-auth.md §8`). An OAuth-flow token carries a refresh token your client should use silently; an API-key token has none — just re-run the `client_credentials` exchange.
