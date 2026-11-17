interface Config {
  apiUrl: string;
  /**
   * bex-api's REST/SSE base — `apiUrl` with the `/graphql` suffix removed.
   * bex-api serves GraphQL and the REST logs stream from the same origin
   * (docs/ADR006-bex-api.md), so the one deployment fact (where the API lives) stays
   * here rather than being re-derived inside a feature. Consumed by the SSE
   * live-log tail (`features/logs`) and by the curl example shown with a
   * freshly minted API key (`features/api-keys`).
   */
  apiBaseUrl: string;
  ssrApiUrl: string;
  /**
   * bex-api's REST base as the SSR server reaches it: `ssrApiUrl` with the
   * `/graphql` suffix removed, as `apiBaseUrl` is for the browser. Server-only
   * callers: the readiness probe and the consent and Connected agents reports.
   */
  ssrApiBaseUrl: string;
  /**
   * Origin that serves the agent-session conversation stream
   * (`/v1/agent-sessions/{id}/stream`). In production this is the SAME origin as
   * `apiBaseUrl` (api.bex.co), where the edge path-routes the stream to the ssh
   * gateway — so it defaults to `apiBaseUrl` and prod needs no extra config. A
   * LOCAL run has no such edge: bex-api and the gateway are separate origins, so
   * `VITE_AGENT_STREAM_URL` points this straight at the gateway attach listener.
   */
  agentStreamBaseUrl: string;
  /**
   * Hydra public issuer (e.g. https://oauth.bex.co). Used to show the
   * client_credentials token endpoint when minting an API key (w4/m105).
   */
  oauthIssuer: string;
  /**
   * `{oauthIssuer}/oauth2/token` — where an API key's client ID and secret
   * can be exchanged for a short-lived token. Optional since w7/m426: the key
   * itself is sent as `Authorization: Bearer`.
   */
  oauthTokenEndpoint: string;
}

const apiUrl = import.meta.env.VITE_API_URL;
const ssrApiUrl = import.meta.env.VITE_SSR_API_URL || apiUrl;
const withoutGraphQLPath = (url: string | undefined) =>
  (url ?? "").replace(/\/graphql\/?$/, "");
const apiBaseUrl = withoutGraphQLPath(apiUrl);
const oauthIssuer = (
  import.meta.env.VITE_OAUTH_ISSUER || "https://oauth.bex.co"
).replace(/\/$/, "");

if (import.meta.env.DEV && !apiUrl) {
  console.warn("[Config] VITE_API_URL is not set!");
}

export const config: Config = {
  apiUrl,
  apiBaseUrl,
  ssrApiUrl,
  ssrApiBaseUrl: withoutGraphQLPath(ssrApiUrl),
  agentStreamBaseUrl: import.meta.env.VITE_AGENT_STREAM_URL || apiBaseUrl,
  oauthIssuer,
  oauthTokenEndpoint: `${oauthIssuer}/oauth2/token`,
};
