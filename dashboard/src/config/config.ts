interface Config {
  apiUrl: string;
  /**
   * bex-api's REST/SSE base — `apiUrl` with the `/graphql` suffix removed.
   * bex-api serves GraphQL and the REST logs stream from the same origin
   * (docs/ADR006-bex-api.md), so the one deployment fact (where the API lives) stays
   * here rather than being re-derived inside a feature. Consumed by the SSE
   * live-log tail (`features/logs`).
   */
  apiBaseUrl: string;
  ssrApiUrl: string;
  /**
   * Origin that serves the agent-session conversation stream
   * (`/v1/agent-sessions/{id}/stream`). In production this is the SAME origin as
   * `apiBaseUrl` (api.bex.co), where the edge path-routes the stream to the ssh
   * gateway — so it defaults to `apiBaseUrl` and prod needs no extra config. A
   * LOCAL run has no such edge: bex-api and the gateway are separate origins, so
   * `VITE_AGENT_STREAM_URL` points this straight at the gateway attach listener.
   */
  agentStreamBaseUrl: string;
}

const apiUrl = import.meta.env.VITE_API_URL;
const ssrApiUrl = import.meta.env.VITE_SSR_API_URL || apiUrl;
const apiBaseUrl = (apiUrl ?? "").replace(/\/graphql\/?$/, "");

if (import.meta.env.DEV && !apiUrl) {
  console.warn("[Config] VITE_API_URL is not set!");
}

export const config: Config = {
  apiUrl,
  apiBaseUrl,
  ssrApiUrl,
  agentStreamBaseUrl: import.meta.env.VITE_AGENT_STREAM_URL || apiBaseUrl,
};
