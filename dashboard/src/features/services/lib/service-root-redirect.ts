import { redirect } from "@tanstack/react-router";
import { ServerDocument } from "@/graphql/definitions";
import type { RouterContext } from "@/common/types/router-context";

/**
 * Land the bare service URL on its type's primary tab, under its canonical base
 * (Render parity, w5/m57): a static_site lives at `/static/<id>` and lands on
 * Events (it has no Deploys tab — deploy history/detail is reached from the
 * Events feed); every other type lives at `/services/<id>` and lands on
 * Deploys. `type` is the raw wire serviceType (`static_site`, `web_service`, …);
 * an unknown/absent type falls back to the `/services` Deploys default.
 */
export function redirectServiceRoot(
  serviceId: string,
  type?: string | null,
): never {
  throw redirect({
    to:
      type === "static_site"
        ? "/static/$serviceId/events"
        : "/services/$serviceId/deploys",
    params: { serviceId },
    replace: true,
  });
}

/**
 * Shared beforeLoad body for both service-root index routes (`/services/<id>`
 * and `/static/<id>`): resolve the service type (cache-first — reuses the
 * layout's ServerDocument fetch, Apollo dedupes) then redirect to the canonical
 * base + primary tab. Keeping one resolver means the two bare URLs can never
 * drift apart.
 */
export async function redirectServiceRootByType(
  client: RouterContext["client"],
  serviceId: string,
): Promise<never> {
  const result = await client.query({
    query: ServerDocument,
    variables: { id: serviceId },
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  return redirectServiceRoot(serviceId, result.data?.server?.type ?? null);
}
