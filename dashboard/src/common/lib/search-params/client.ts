export function getSearchParamOnClient(key: string): string | null {
  // URLSearchParams.get already percent-decodes, as on the server (server.ts);
  // decoding again threw URIError on `?lang=%25` and replaced the app with the
  // error page.
  return new URLSearchParams(window.location.search).get(key);
}
