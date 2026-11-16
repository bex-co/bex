// The one string an API key is (docs/ADR012-auth.md §8e, w7/m426): the user
// sends it as `Authorization: Bearer <key>`, as a Render API key is sent.

/**
 * The bearer string of a freshly minted key. bex-api returns it as
 * `createApiKey.key`; when the answer carries none (a bex-api from before the
 * field, during a rollout), it is composed from the two halves the response
 * always has, in the server's own format `bex_<id>_<secret>`, so the dialog
 * never shows an empty field.
 */
export function bearerKey(
  id: string,
  secret: string,
  key?: string | null,
): string {
  return key || `bex_${id}_${secret}`;
}

/**
 * A request that works when pasted: list the workspace's services with the key
 * as the bearer (the request Render's "make your first request" step uses).
 * `apiBaseUrl` is the REST origin (`config.apiBaseUrl`). This is a code sample,
 * not prose, so it is the same in every language.
 */
export function curlExample(apiBaseUrl: string, key: string): string {
  const base = apiBaseUrl.replace(/\/+$/, "");
  return [
    "curl --request GET \\",
    `     --url '${base}/v1/services?limit=20' \\`,
    "     --header 'Accept: application/json' \\",
    `     --header 'Authorization: Bearer ${key}'`,
  ].join("\n");
}
