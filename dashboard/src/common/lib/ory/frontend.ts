import { Configuration, FrontendApi } from "@ory/client-fetch";
import { KRATOS_PUBLIC_URL } from "./config";

/**
 * Creates a `FrontendApi` client for Kratos's self-service flow API.
 *
 * On the server there's no browser cookie jar for `credentials: "include"`
 * to draw from, so pass the incoming request's raw Cookie header (see
 * `common/server-fn/session.ts` for the SSR-side caller).
 */
export function createFrontendApi(cookie?: string): FrontendApi {
  return new FrontendApi(
    new Configuration({
      basePath: KRATOS_PUBLIC_URL,
      credentials: cookie ? undefined : "include",
      headers: cookie ? { Cookie: cookie } : {},
    }),
  );
}
