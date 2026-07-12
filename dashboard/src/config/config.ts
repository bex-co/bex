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
}

const apiUrl = import.meta.env.VITE_API_URL;
const ssrApiUrl = import.meta.env.VITE_SSR_API_URL || apiUrl;

if (import.meta.env.DEV && !apiUrl) {
  console.warn("[Config] VITE_API_URL is not set!");
}

export const config: Config = {
  apiUrl,
  apiBaseUrl: (apiUrl ?? "").replace(/\/graphql\/?$/, ""),
  ssrApiUrl,
};
