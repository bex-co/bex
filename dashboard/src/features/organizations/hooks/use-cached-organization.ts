import { useQuery } from "@apollo/client/react";
import {
  OrganizationDocument,
  OrganizationsDocument,
} from "@/graphql/definitions";
import type { RouterContext } from "@/common/types/router-context";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import {
  findOrganization,
  toOrganization,
  type Organization,
} from "@/features/organizations/types";

/**
 * What the client already knows about an org before its page loads (w4/706):
 * the org's own `organization(id)` entry, which every org page reads (so a
 * move between org tabs always has it), else its entry in the caller's
 * `organizations` list, which the dashboard shell fetches at every width.
 * Read from the cache only; undefined until either source holds the org.
 */
export function useCachedOrganization(
  organizationId: string,
): Organization | undefined {
  const { data } = useQuery(OrganizationDocument, {
    variables: { id: organizationId },
    fetchPolicy: "cache-only",
    errorPolicy: "all",
  });
  const { organizations } = useOrganizations();
  const own = data?.organization;
  return own
    ? toOrganization(own)
    : findOrganization(organizations, organizationId);
}

/** {@link useCachedOrganization}'s read, for a route loader. */
export function readCachedOrganization(
  client: RouterContext["client"],
  organizationId: string,
) {
  return (
    client.readQuery({
      query: OrganizationDocument,
      variables: { id: organizationId },
    })?.organization ??
    findOrganization(
      client.readQuery({ query: OrganizationsDocument })?.organizations,
      organizationId,
    )
  );
}
