import {
  OrganizationDocument,
  OrganizationInvitesDocument,
  OrganizationMembersDocument,
  OrganizationsDocument,
} from "@/graphql/definitions";
import { titleLoaderFetchPolicy } from "@/common/lib/document-head";
import { prefetchInParallel } from "@/common/lib/prefetch";
import type { RouterContext } from "@/common/types/router-context";
import {
  findOrganization,
  toOrganizationRole,
  type OrganizationSection,
} from "@/features/organizations/types";

/**
 * The org routes' loader (w7/m464): every page reads the org, which names the
 * page and fills its header. Members also primes its list (but for a Guest)
 * and an Owner's pending invites, by the role the switcher's cached orgs give
 * the caller, so the page's hooks (`PRIMED_FETCH_POLICY`) render from the
 * cache. Network on entry and preload, the cache on a retained match
 * (`titleLoaderFetchPolicy`).
 */
export function loadOrganizationRoute(
  {
    context: { client },
    params: { orgId },
    cause,
  }: {
    context: RouterContext;
    params: { orgId: string };
    cause: "preload" | "enter" | "stay";
  },
  section: OrganizationSection,
): Promise<void> {
  const fetchPolicy = titleLoaderFetchPolicy(cause);
  const reads: Array<() => Promise<unknown>> = [
    () =>
      client.query({
        query: OrganizationDocument,
        variables: { id: orgId },
        fetchPolicy,
        errorPolicy: "all",
      }),
  ];
  if (section === "members") {
    const variables = { organizationId: orgId };
    const role = toOrganizationRole(
      findOrganization(
        client.readQuery({ query: OrganizationsDocument })?.organizations,
        orgId,
      )?.viewerRole,
    );
    // A Guest is refused the list (FORBIDDEN), which the cache does not keep.
    if (role !== "GUEST") {
      reads.push(() =>
        client.query({
          query: OrganizationMembersDocument,
          variables,
          fetchPolicy,
          errorPolicy: "all",
        }),
      );
    }
    if (role === "OWNER") {
      reads.push(() =>
        client.query({
          query: OrganizationInvitesDocument,
          variables,
          fetchPolicy,
          errorPolicy: "all",
        }),
      );
    }
  }
  return prefetchInParallel(reads);
}
