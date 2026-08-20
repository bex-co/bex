import { createIsomorphicFn } from "@tanstack/react-start";
import { logServerError } from "@/common/lib/server-log";

function errorMessage(error: unknown): string {
  if (error instanceof Error && error.message) return error.message;
  if (typeof error === "string" && error) return error;
  return "route_error";
}

/**
 * Report a route/SSR failure to the pod log stream. Server-only: the client
 * branch is a no-op so ErrorPage can call this unconditionally without
 * bundling `getRequest` into the browser.
 */
export const reportRouteError = createIsomorphicFn()
  .server(async (error: unknown, status = 500) => {
    let path: string | undefined;
    try {
      const { getRequest } = await import("@tanstack/react-start/server");
      path = new URL(getRequest().url).pathname;
    } catch {
      // Outside a request context (tests / edge cases) — still emit msg.
    }
    logServerError({
      msg: errorMessage(error),
      path,
      status,
    });
  })
  .client((_error: unknown, _status?: number) => {
    // Client-side error UI stays silent on the pod stream.
  });

/** Run `fn`; on throw, emit a server error line then rethrow. */
export async function withReportedRouteError<T>(
  fn: () => Promise<T>,
  status = 500,
): Promise<T> {
  try {
    return await fn();
  } catch (err) {
    await reportRouteError(err, status);
    throw err;
  }
}