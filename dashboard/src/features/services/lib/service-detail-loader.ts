import type { ParsedLocation } from "@tanstack/react-router";
import { ServerDocument } from "@/graphql/definitions";
import type { ServerQuery } from "@/graphql/definitions";
import {
  isNotFoundError,
  titleLoaderFetchPolicy,
  type RouteResource,
} from "@/common/lib/document-head";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";
import type { RouterContext } from "@/common/types/router-context";
import { serviceBaseForType, type ServiceBase } from "./service-base";

export type ServerResource = NonNullable<ServerQuery["server"]>;

/**
 * Load a service for the service-detail layout, canonicalizing the URL base
 * first: a static_site lives under `/static/<id>` and every other type under
 * `/services/<id>` (Render parity, w5/m57). When the request arrives under the
 * wrong base for this service's type we bounce to the canonical one — carrying
 * the sub-path, query, and hash — before rendering, so `/services/<static-id>`
 * and `/static/<compute-id>` both settle on the right tree. Reuses the layout's
 * single fetch (network on entry/preload, cache on retained-match re-runs —
 * `titleLoaderFetchPolicy`) and keeps loadRouteResource's not-found-vs-error
 * distinction for the shell.
 */
export async function loadServiceDetail(
  client: RouterContext["client"],
  serviceId: string,
  base: ServiceBase,
  location: ParsedLocation,
  cause: "preload" | "enter" | "stay" = "enter",
): Promise<RouteResource<ServerResource>> {
  const result = await client.query({
    query: ServerDocument,
    variables: { id: serviceId },
    fetchPolicy: titleLoaderFetchPolicy(cause),
    errorPolicy: "all",
  });
  const server = result.data?.server ?? undefined;
  // Only redirect once the type is known — a null server (not-found/error)
  // keeps the shell's own not-found/error handling.
  if (server?.type) {
    const canonical = serviceBaseForType(server.type);
    if (canonical !== base) redirectToBase(canonical, base, location);
  }
  const resource =
    server && (server.displayName?.trim() || server.name?.trim())
      ? server
      : null;
  if (resource) return { state: "ready", resource };
  if (!result.error || isNotFoundError(result.error))
    return { state: "not-found" };
  return { state: "error" };
}

/** Swap the leading base segment (`/services` ↔ `/static`) and redirect,
 *  carrying the sub-path + query + hash (throws — never returns). */
function redirectToBase(
  canonical: ServiceBase,
  current: ServiceBase,
  location: ParsedLocation,
): never {
  const pathname = location.pathname.startsWith(`${current}/`)
    ? canonical + location.pathname.slice(current.length)
    : canonical + location.pathname;
  redirectPreservingSuffix(pathname, location);
}
