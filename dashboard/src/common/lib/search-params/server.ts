import { getRequest } from "@tanstack/react-start/server";

export function getSearchParamOnServer(key: string): string | null {
  const request = getRequest();
  // URLSearchParams.get already percent-decodes; decoding again threw URIError
  // on `?lang=%25` and answered 500 (w21/m394 t017, R25-15).
  return new URL(request.url).searchParams.get(key);
}
