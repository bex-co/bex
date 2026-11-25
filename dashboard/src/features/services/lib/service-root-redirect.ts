import { redirect, type ParsedLocation } from "@tanstack/react-router";
import { ServerDocument } from "@/graphql/definitions";
import type { RouterContext } from "@/common/types/router-context";
import { isCronType } from "@/features/services/lib/service-type";
import {
  COMPUTE_SECTION_ANCHOR,
  redirectToComputePage,
} from "@/features/services/lib/compute-page";

/**
 * Land the bare service URL on its type's primary tab, under its canonical base
 * (Render parity, w5/m57): a static_site lives at `/static/<id>` and lands on
 * Events (it has no Deploys tab — deploy history/detail is reached from the
 * Events feed); a cron job lands on Runs, the page Render shows at `/cron/<id>`
 * (w3/m554); every other type lives at `/services/<id>` and lands on Deploys.
 * `type` is the raw wire serviceType (`static_site`, `web_service`, …); an
 * unknown/absent type falls back to the `/services` Deploys default.
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
  if (type && isCronType(type)) return "/services/$serviceId/runs" as const;
  return "/services/$serviceId/deploys" as const;
}

async function serverRead(client: RouterContext["client"], serviceId: string) {
  const result = await client.query({
    query: ServerDocument,
    variables: { id: serviceId },
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  return result.data?.server ?? null;
}

async function serviceType(
  client: RouterContext["client"],
  serviceId: string,
): Promise<string | null> {
  return (await serverRead(client, serviceId))?.type ?? null;
}

/**
 * Runs belong to a cron job (w2/m360): another type's Runs URL lands on its
 * primary tab. An unknown type renders the page asked for.
 */
export async function redirectRunsByType(
  client: RouterContext["client"],
  serviceId: string,
): Promise<void> {
  const type = await serviceType(client, serviceId);
  if (type !== null && !isCronType(type)) redirectServiceRoot(serviceId, type);
}

/**
 * Render's `/cron/<id>/builds` replaces itself with `/cron/<id>/deploys`, and
 * a build's detail is `/cron/<id>/builds/<id>` (w3/m554,
 * docs/render-artifacts/cron-dashboard.md). A cron job's build is its deploy,
 * so its Builds URL lands on its Deploys page with its query (the filters
 * w4/284 keeps in the URL), and a build's on that deploy's detail. Another
 * type never had a Builds URL, so it lands on its primary tab; an unknown type
 * takes the cron job's way, since the Deploys page is what Builds became.
 */
export async function redirectBuildsByType(
  client: RouterContext["client"],
  serviceId: string,
  deployId: string | undefined,
): Promise<never> {
  const type = await serviceType(client, serviceId);
  if (type !== null && !isCronType(type)) redirectServiceRoot(serviceId, type);
  if (deployId) {
    throw redirect({
      to: "/services/$serviceId/deploys/$deployId",
      params: { serviceId, deployId },
      search: true,
      replace: true,
    });
  }
  throw redirect({
    to: "/services/$serviceId/deploys",
    params: { serviceId },
    search: true,
    replace: true,
  });
}

/**
 * A cron job has no Compute page (w3/m554): its plan is its Settings' Compute
 * section, where Render's plan badge links (`settings#compute`), so its
 * Compute URL lands there with its query. Every other type, and an unknown
 * one, renders the Compute page.
 */
export async function redirectCronCompute(
  client: RouterContext["client"],
  serviceId: string,
): Promise<void> {
  const type = await serviceType(client, serviceId);
  if (type === null || !isCronType(type)) return;
  throw redirect({
    to: "/services/$serviceId/settings",
    params: { serviceId },
    search: (prev) => prev,
    hash: COMPUTE_SECTION_ANCHOR,
    replace: true,
  });
}

/**
 * A cron job's Plan URL renders its plan page, Render's `/cron/<id>/plan`
 * (w3/105, docs/render-artifacts/cron-dashboard.md), and a suspended one's
 * lands on Settings with its query, as Render's redirects there. Every other
 * type's lands on its Compute page with its query and hash (w6/m483), and so
 * does an unknown one, whose Compute URL then decides.
 */
export async function redirectPlanByType(
  client: RouterContext["client"],
  serviceId: string,
  location: Pick<ParsedLocation, "hash">,
): Promise<void> {
  const server = await serverRead(client, serviceId);
  if (!server?.type || !isCronType(server.type)) {
    redirectToComputePage(serviceId, location);
  }
  if (server.suspended) {
    throw redirect({
      to: "/services/$serviceId/settings",
      params: { serviceId },
      search: (prev) => prev,
      replace: true,
    });
  }
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
