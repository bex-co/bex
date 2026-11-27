import { isNotFoundError, isForbiddenError } from "@/common/lib/graphql-error";
import { isUnauthenticatedError } from "@/common/apollo/auth-error-link";
import { ownDeleteActive } from "@/common/lib/own-delete";
import type { RouteResource } from "@/common/lib/document-head";

/**
 * Has this detail query settled on "there is no such resource"?
 *
 * A dead id is not a silent null. bex-api answers `server(id: "<dead>")` with
 * `data.server = null` AND an `errors` entry saying why (`NOT_FOUND`), so a
 * page that reads a bare `!error` treats every deleted resource as an outage.
 * Both halves are needed: settled-and-empty, and the error — if any — is a
 * not-found one (w6/m44).
 */
export function resourceNotFound(
  resource: unknown,
  loading: boolean,
  error: Error | undefined,
): boolean {
  return !loading && !resource && (!error || isNotFoundError(error));
}

/**
 * Which state a settled-empty detail query draws in place of its page, as
 * Render's resource pages decide it (docs/render-artifacts/resource-not-found.md,
 * w3/m706), or null while it loads or once it has the resource:
 *
 * - `"not-found"`: bex-api resolved the id to nothing — the page's own
 *   not-found state ("Service not found", "Page not found", …).
 * - `"forbidden"`: the read was refused (`FORBIDDEN`) — Render's Access denied.
 * - `"unauthenticated"`: the session expired (401) — bex's Sign in state; the
 *   Apollo auth link is already arranging the login redirect (w3/m80).
 * - `"error"`: any other failure — Render's "An error occurred".
 *
 * The four are mutually exclusive, so a page never shows two at once. While a
 * delete this dashboard started is in flight or just done, a not-found read is
 * that delete's own doing: it stays null (the page keeps its frame) and the
 * delete flow owns the toast and the landing (w4/645).
 */
export type ResourceReadState =
  "not-found" | "forbidden" | "unauthenticated" | "error";

export function resourceReadState(
  resource: unknown,
  loading: boolean,
  error: Error | undefined,
  routeState?: RouteResource<unknown>["state"],
): ResourceReadState | null {
  if (loading || resource) return null;
  // Apollo caches a refused read's null without its error, so a page reading
  // the cache after its route's loader sees "no error". The loader saw the
  // answer: its refusal stands over an errorless empty read.
  if (!error && routeState === "forbidden") return "forbidden";
  if (!error || isNotFoundError(error)) {
    return ownDeleteActive() ? null : "not-found";
  }
  if (isUnauthenticatedError(error)) return "unauthenticated";
  if (isForbiddenError(error)) return "forbidden";
  return "error";
}
