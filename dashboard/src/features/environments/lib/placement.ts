import type { Translate } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
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
 * bex-api's `aclBearing`: protected, isolated, or carrying an inbound IP
 * allow-list that restricts something (the allow-all default does not; an
 * emptied list is deny-all and does, w4/355). Taking a resource out of such an environment needs its admin
 * (w4/m232), so the dialog disables that move for anyone else instead of
 * letting Save meet a 403.
 */
export function aclBearing(e: EnvironmentView): boolean {
  return (
    e.protectedStatus === "protected" ||
    e.networkIsolationEnabled ||
    restrictsInbound(e)
  );
}

const ALLOW_ALL = ["0.0.0.0/0", "::/0"];

/** An allow-list restricts unless some entry admits every address. */
function restrictsInbound(e: EnvironmentView): boolean {
  return !e.ipAllowListEntries.some((entry) =>
    ALLOW_ALL.includes(entry.cidrBlock),
  );
}

/**
 * The restrictions a resource sheds by leaving e, as marker locale keys. The
 * seeded allow-all pair restricts nothing, so it is not shown.
 */
export function restrictionMarkers(e: EnvironmentView): string[] {
  const markers: string[] = [];
  if (e.protectedStatus === "protected")
    markers.push("environments.markerProtected");
  if (e.networkIsolationEnabled) markers.push("environments.markerIsolated");
  if (restrictsInbound(e)) markers.push("environments.markerAllowList");
  return markers;
}

/** bex-api's refusal of a non-Admin's protected placement (w3/m281). */
export const PROTECTED_PLACEMENT_CODE = "PROTECTED_ENVIRONMENT_ADMIN_REQUIRED";

/** Whether e is a protected environment (Render's `protectedStatus`). */
export function protectedEnvironment(
  e: { protectedStatus?: string } | null | undefined,
): boolean {
  return e?.protectedStatus === "protected";
}

/**
 * mutationErrorMessage for a create or move: a raced protected-placement 403
 * reads as the same localized reason the disabled control shows (w3/m281);
 * anything else keeps the server's words or the caller's generic copy.
 */
export function placementErrorMessage(
  err: unknown,
  generic: string,
  t: Translate,
): string {
  return hasGraphQLErrorCode(err, PROTECTED_PLACEMENT_CODE)
    ? t("environments.protectedPlacementAdminOnly")
    : mutationErrorMessage(err, generic);
}
