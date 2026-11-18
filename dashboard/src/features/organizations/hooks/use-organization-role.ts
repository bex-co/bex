import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import {
  findOrganization,
  toOrganizationRole,
  type OrganizationRole,
} from "@/features/organizations/types";

/**
 * The caller's role in an org, from the switcher's `organizations` list the
 * chrome has already loaded (w7/m464); null for no org, or one the list does
 * not (yet) hold.
 */
export function useOrganizationRole(
  organizationId: string | null | undefined,
): OrganizationRole | null {
  const { organizations } = useOrganizations();
  return toOrganizationRole(
    findOrganization(organizations, organizationId)?.viewerRole,
  );
}
