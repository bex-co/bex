import { splatParts } from "@/common/lib/render-alias";
import {
  metricsPeriodSearch,
  type RangePresetID,
} from "@/features/metrics/lib/range";

/** The pages a sub-path of a service URL can land on (below). */
export type ServiceSubpathPage =
  "/disk" | "/metrics" | "/shell" | "/deploys/$deployId";

/** Where a sub-path of `/services/<id>/` with no page of its own lands. */
export interface ServiceSubpathLanding {
  /** The page; null is the service's root, its type's primary tab. */
  page: ServiceSubpathPage | null;
  /** The deploy, for `/deploys/$deployId`. */
  deployId?: string;
  /** Search keys the landing sets over the link's own query. */
  search?: { range?: RangePresetID; instance?: string };
}

/**
 * Render's service sub-paths whose name or shape is not bex's (w3/m707;
 * Render's service route table, docs/render-artifacts/dashboard-routes.md
 * § Service sub-tab map): `disks` is bex's Disk page; `metrics/<period>`
 * opens Metrics on the window Render's period table maps it to;
 * `shell/<instance>` opens Shell on that instance, which Render names by its
 * id's last segment (`Instance lqtnn`) and bex's `?instance=` by its whole
 * id, `<service id>-<suffix>`; a deploy's page answers any of its sub-paths,
 * as Render's `deploys/:deployId/*` does. Anything else lands on the
 * service's root, as Render's `*` route sends it there: its PR previews and
 * one-off jobs (DO_NOT_DO), its deprecated per-resource sharing, and a path
 * it does not route.
 */
export function serviceSubpathLanding(
  serviceId: string,
  splat: string | undefined,
): ServiceSubpathLanding {
  const [page, child, ...rest] = splatParts(splat);
  switch (page) {
    case "disks":
      if (child === undefined) return { page: "/disk" };
      break;
    case "metrics":
      if (child !== undefined && rest.length === 0) {
        return { page: "/metrics", search: metricsPeriodSearch(child) };
      }
      break;
    case "shell":
      if (child !== undefined && rest.length === 0) {
        const instance = child.startsWith(`${serviceId}-`)
          ? child
          : `${serviceId}-${child}`;
        return { page: "/shell", search: { instance } };
      }
      break;
    case "deploys":
      if (child !== undefined && rest.length > 0) {
        return { page: "/deploys/$deployId", deployId: child };
      }
      break;
  }
  return { page: null };
}

/** The landings a static site's own pages hold: it has no Disk or Shell. */
export const STATIC_SUBPATH_PAGES: ReadonlySet<ServiceSubpathPage> = new Set([
  "/metrics",
  "/deploys/$deployId",
]);
