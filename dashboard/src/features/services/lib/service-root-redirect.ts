import { redirect } from "@tanstack/react-router";
import { ServerDocument } from "@/graphql/definitions";
import type { RouterContext } from "@/common/types/router-context";
import { isCronType } from "@/features/services/lib/service-type";

/**
 * Land the bare service URL on its type's primary tab, under its canonical base
 * (Render parity, w5/m57): a static_site lives at `/static/<id>` and lands on
 * Events (it has no Deploys tab — deploy history/detail is reached from the
 * Events feed); a cron job lands on Builds (its Deploys tab, split from Runs as
 * on Render, w2/m360); every other type lives at `/services/<id>` and lands on
 * Deploys. `type` is the raw wire serviceType (`static_site`, `web_service`, …);
 * an unknown/absent type falls back to the `/services` Deploys default.
 */
export function redirectServiceRoot(
  serviceId: string,
  type?: string | null,
): never {
  throw redirect({
    to: primaryTab(type),
    params: { serviceId },
    replace: true,
  });
}

function primaryTab(type?: string | null) {
  if (type === "static_site") return "/static/$serviceId/events" as const;
  if (type && isCronType(type)) return "/services/$serviceId/builds" as const;
  return "/services/$serviceId/deploys" as const;
}

async function serviceType(
  client: RouterContext["client"],
  serviceId: string,
): Promise<string | null> {
  const result = await client.query({
    query: ServerDocument,
    variables: { id: serviceId },
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  return result.data?.server?.type ?? null;
}

/**
 * Keep a service's history tab the one its type has (w2/m360): a cron job's
 * Deploys URL lands on its Builds page, with its filters, and Builds and Runs
 * belong to cron jobs only, so another type's lands on its primary tab. An
 * unknown type renders the tab asked for.
 */
export async function redirectHistoryTabByType(
  client: RouterContext["client"],
  serviceId: string,
  tab: "deploys" | "builds" | "runs",
): Promise<void> {
  const type = await serviceType(client, serviceId);
  if (type === null) return;
  const cron = isCronType(type);
  if (tab === "deploys" && cron) {
    throw redirect({
      to: "/services/$serviceId/builds",
      params: { serviceId },
      search: (prev) => prev,
      replace: true,
    });
  }
  if (tab !== "deploys" && !cron) redirectServiceRoot(serviceId, type);
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
  return redirectServiceRoot(serviceId, await serviceType(client, serviceId));
}
