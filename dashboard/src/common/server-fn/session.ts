import type { Session } from "@ory/client-fetch";
import { createIsomorphicFn } from "@tanstack/react-start";
import { createFrontendApi } from "@/common/lib/ory/frontend";

/**
 * The incoming request's Cookie header when rendering on the server; in the
 * browser, fetch sends cookies itself (`credentials: "include"`). An
 * isomorphic fn (not a bare `import.meta.env.SSR` guard) so the compiler
 * strips the server-only import from the client bundle — otherwise
 * TanStack Start's import-protection warns on every dev-server boot.
 */
const getRequestCookie = createIsomorphicFn()
  .server(() =>
    import("@tanstack/react-start/server").then((m) =>
      m.getRequestHeader("cookie"),
    ),
  )
  .client(() => undefined);

export async function fetchSession(): Promise<Session | null> {
  try {
    return await createFrontendApi(await getRequestCookie()).toSession();
  } catch {
    return null;
  }
}
