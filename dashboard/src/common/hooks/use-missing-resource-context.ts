import { useFragment } from "@apollo/client/react";
import {
  EnvironmentFieldsFragmentDoc,
  MissingServiceContextFragmentDoc,
  SandboxGroupFieldsFragmentDoc,
} from "@/graphql/definitions";

/**
 * What the dashboard still knows about a resource whose id now reads as not
 * found, from Apollo's normalized cache (w3/m706). A read that comes back
 * empty leaves the entity it had cached in place, so a resource the dashboard
 * had read — its page open while it was deleted elsewhere, or a list that
 * showed it — still has its name, which makes the page read "<Label> deleted"
 * where an id it never saw reads "<Label> not found". That is Render's rule:
 * each of its callers reads a `missing…Context` fragment with `useFragment`
 * (docs/render-artifacts/resource-not-found.md).
 */
export interface MissingResourceContext {
  /** The cached name, or null when the dashboard never read the resource. */
  name: string | null;
  /** The project the cached resource sits in: Go to project. */
  projectId: string | null;
}

/**
 * The project of a cached environment: a service read names its environment
 * but not always its project, and a sandbox group names only its environment.
 * The breadcrumbs' environments read caches each one with its project.
 */
function useCachedEnvironmentProject(
  environmentId: string | null | undefined,
): string | null {
  const { data } = useFragment({
    fragment: EnvironmentFieldsFragmentDoc,
    from: environmentId
      ? { __typename: "Environment", id: environmentId }
      : null,
  });
  return (environmentId && data?.projectId) || null;
}

/** A missing service's cached name, project and type (`cron_job` reads "Cron job"). */
export function useMissingServiceContext(
  serviceId: string,
): MissingResourceContext & { type: string | null } {
  const { data } = useFragment({
    fragment: MissingServiceContextFragmentDoc,
    from: { __typename: "Service", id: serviceId },
  });
  const environmentProject = useCachedEnvironmentProject(data?.environmentId);
  return {
    name: data?.name?.trim() || null,
    projectId: data?.projectId || environmentProject,
    type: data?.type ?? null,
  };
}

/** A missing sandbox group's cached name, and its project through its cached
 *  environment. */
export function useMissingSandboxGroupContext(
  groupId: string,
): MissingResourceContext {
  const { data: group } = useFragment({
    fragment: SandboxGroupFieldsFragmentDoc,
    from: { __typename: "SandboxGroup", id: groupId },
  });
  return {
    name: group?.name?.trim() || null,
    projectId: useCachedEnvironmentProject(group?.environmentId),
  };
}
