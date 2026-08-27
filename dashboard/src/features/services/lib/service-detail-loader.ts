import type { ParsedLocation } from "@tanstack/react-router";
import {
  EnvironmentsDocument,
  ProjectsDocument,
  ServerDocument,
  ServicesDocument,
} from "@/graphql/definitions";
import type { ServerQuery } from "@/graphql/definitions";
import {
  isNotFoundError,
  titleLoaderFetchPolicy,
  type RouteResource,
} from "@/common/lib/document-head";
import { isUnauthenticatedError } from "@/common/apollo/auth-error-link";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";
import type { RouterContext } from "@/common/types/router-context";
import { serviceBaseForType, type ServiceBase } from "./service-base";

export type ServerResource = NonNullable<ServerQuery["server"]>;

/**
 * Warm the topbar breadcrumb queries (projects → environments + services list)
 * alongside a service-detail title load so ServiceBreadcrumbs does not waterfall
 * after paint.
 */
export async function warmServiceBreadcrumbs(
  client: RouterContext["client"],
  serviceId: string,
  ownerId: string | null | undefined,
  cause: "preload" | "enter" | "stay",
): Promise<void> {
  if (ownerId == null) return;
  const fetchPolicy = titleLoaderFetchPolicy(cause);
  const [projectsResult] = await Promise.all([
    client
      .query({
        query: ProjectsDocument,
        variables: { ownerId },
        fetchPolicy,
        errorPolicy: "all",
      })
      .catch(() => undefined),
    client
      .query({
        query: ServicesDocument,
        variables: { ownerId },
        fetchPolicy,
        errorPolicy: "all",
      })
      .catch(() => undefined),
  ]);
  const projectId = projectsResult?.data?.projects?.find((project) =>
    (project?.serviceIds ?? []).includes(serviceId),
  )?.id;
  if (!projectId) return;
  await client
    .query({
      query: EnvironmentsDocument,
      variables: { projectId },
      fetchPolicy,
      errorPolicy: "all",
    })
    .catch(() => undefined);
}

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
  ownerId?: string | null,
): Promise<RouteResource<ServerResource>> {
  // Breadcrumb chrome is not required to paint the detail shell — kick it off
  // without awaiting so a cold enter/preload still gates only on ServerDocument
  // (one RTT). Hover-intent and stay already hide most of this path.
  void warmServiceBreadcrumbs(client, serviceId, ownerId, cause);
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
  if (result.error && isUnauthenticatedError(result.error))
    return { state: "unauthenticated" };
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
