import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { ProjectResourceFamily } from "@/features/projects/lib/resource-filter";

const MEMBERS: Record<
  ProjectResourceFamily,
  keyof Pick<
    EnvironmentView,
    "serviceIds" | "databaseIds" | "keyValueIds" | "envGroupIds"
  >
> = {
  services: "serviceIds",
  databases: "databaseIds",
  keyvalues: "keyValueIds",
  envgroups: "envGroupIds",
};

/**
 * A lookup of the environment each resource is in now, built once from the
 * workspace's member lists.
 */
export function placementIndex(
  environments: EnvironmentView[],
): (kind: ProjectResourceFamily, id: string) => EnvironmentView | undefined {
  const index = new Map<string, EnvironmentView>();
  for (const e of environments) {
    for (const [kind, members] of Object.entries(MEMBERS)) {
      for (const id of e[members]) index.set(`${kind}:${id}`, e);
    }
  }
  return (kind, id) => index.get(`${kind}:${id}`);
}

/**
 * bex-api's `aclBearing`: protected, isolated, or carrying any inbound IP
 * allow-list. Taking a resource out of such an environment needs its admin
 * (w4/m232), so the dialog disables that move for anyone else instead of
 * letting Save meet a 403.
 */
export function aclBearing(e: EnvironmentView): boolean {
  return (
    e.protectedStatus === "protected" ||
    e.networkIsolationEnabled ||
    e.ipAllowListEntries.length > 0
  );
}

const DEFAULT_ALLOW_ALL = ["0.0.0.0/0", "::/0"];

/**
 * The restrictions a resource sheds by leaving e, as marker locale keys. The
 * seeded allow-all pair restricts nothing, so it is not shown.
 */
export function restrictionMarkers(e: EnvironmentView): string[] {
  const markers: string[] = [];
  if (e.protectedStatus === "protected")
    markers.push("environments.markerProtected");
  if (e.networkIsolationEnabled) markers.push("environments.markerIsolated");
  const cidrs = e.ipAllowListEntries.map((entry) => entry.cidrBlock);
  const allowAll =
    cidrs.length === DEFAULT_ALLOW_ALL.length &&
    DEFAULT_ALLOW_ALL.every((cidr) => cidrs.includes(cidr));
  if (cidrs.length > 0 && !allowAll)
    markers.push("environments.markerAllowList");
  return markers;
}
