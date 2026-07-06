import { Configuration, FrontendApi } from "@ory/client-fetch";
import { KRATOS_PUBLIC_URL, KRATOS_SSR_URL } from "./config";

/**
 * Creates a `FrontendApi` client for Kratos's self-service flow API.
 *
 * Picks the SSR (in-cluster) or public (browser-facing) base URL depending
 * on where this runs. On the server there's no browser cookie jar for
 * `credentials: "include"` to draw from, so pass the incoming request's raw
 * Cookie header (see `common/server-fn/session.ts` for the SSR-side caller).
 */
export function createFrontendApi(cookie?: string): FrontendApi {
  return new FrontendApi(
    new Configuration({
      basePath: import.meta.env.SSR ? KRATOS_SSR_URL : KRATOS_PUBLIC_URL,
      credentials: cookie ? undefined : "include",
      headers: cookie ? { Cookie: cookie } : {},
    }),
  );
}
