import { blueprintResourceHref } from "@/features/blueprints/lib/resource-href";
import type { LogResource } from "../types";

/**
 * How a project view's log line names its resource (w3/m505): Render's colored
 * dot and `[name]`, one color per resource, the name linking to the resource's
 * own page (`null` for a type with no page).
 */
export interface LogResourceLabel {
  id: string;
  name: string;
  href: string | null;
  /** Tailwind background class of the resource's dot. */
  dotClass: string;
}

// Distinct hues that read on both themes; a view reads at most 20 resources
// (bex-api's cap), so a project past eight shares colors in turn.
export const RESOURCE_DOT_CLASSES = [
  "bg-sky-500",
  "bg-violet-500",
  "bg-amber-500",
  "bg-emerald-500",
  "bg-rose-500",
  "bg-cyan-500",
  "bg-fuchsia-500",
  "bg-lime-500",
] as const;

/**
 * The labels of the resources a page read, keyed by id. Colors follow the
 * server's resource order (services by name, then datastores), which is the
 * same on every page and the tail, so a resource keeps its color while the
 * view is open.
 */
export function resourceLabels(
  resources: readonly LogResource[] | undefined,
): ReadonlyMap<string, LogResourceLabel> {
  const out = new Map<string, LogResourceLabel>();
  (resources ?? []).forEach((r, i) => {
    out.set(r.id, {
      id: r.id,
      name: r.name || r.id,
      // An empty service type is a web service from before types were stored.
      href: blueprintResourceHref(r.type || "web_service", r.id),
      dotClass: RESOURCE_DOT_CLASSES[i % RESOURCE_DOT_CLASSES.length],
    });
  });
  return out;
}
