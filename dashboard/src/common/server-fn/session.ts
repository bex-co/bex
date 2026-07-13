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

/**
 * The Kratos session behind the current request, or null. Pass `cookie`
 * explicitly from a server route handler, which holds the `Request` itself
 * (`hydra-consent.ts`); omit it and the ambient request's Cookie header is used.
 */
export async function fetchSession(cookie?: string): Promise<Session | null> {
  try {
    return await createFrontendApi(
      cookie ?? (await getRequestCookie()),
    ).toSession();
  } catch {
    return null;
  }
}
