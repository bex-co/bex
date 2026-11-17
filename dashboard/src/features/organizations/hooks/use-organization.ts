import { useCallback, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { OrganizationDocument } from "@/graphql/definitions";
import { isNotFoundError } from "@/common/lib/graphql-error";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";
import {
  toOrganization,
  toOrganizationRole,
  type Organization,
  type OrganizationRole,
} from "@/features/organizations/types";

export interface UseOrganizationResult {
  organization: Organization | null;
  /** The caller's org role, null until the org is read. */
  role: OrganizationRole | null;
  /** True only while nothing has been read yet (a refetch keeps the page). */
  loading: boolean;
  /**
   * ORGANIZATION_NOT_FOUND: no such org, or the caller is not in it. With no
   * org and not `notFound`, the read failed.
   */
  notFound: boolean;
  /** The name of one of the org's workspaces the caller can see. */
  workspaceName: (id: string) => string | undefined;
}

/**
 * One organization as the caller sees it (ADR115 D8): its name, plan, the
 * caller's role, and its workspaces with privacy, the primary flag and whether
 * the caller is in each. The route loader primes it.
 */
export function useOrganization(organizationId: string): UseOrganizationResult {
  const { data, loading, error } = useQuery(OrganizationDocument, {
    variables: { id: organizationId },
    fetchPolicy: PRIMED_FETCH_POLICY,
    errorPolicy: "all",
  });
  const raw = data?.organization;
  const organization = useMemo(() => (raw ? toOrganization(raw) : null), [raw]);
  const workspaces = organization?.workspaces;
  const workspaceName = useCallback(
    (id: string) => workspaces?.find((w) => w.id === id)?.name,
    [workspaces],
  );
  const settled = !loading || data !== undefined;
  return {
    organization,
    role: toOrganizationRole(organization?.viewerRole),
    loading: !settled,
    notFound: settled && !organization && isNotFoundError(error),
    workspaceName,
  };
}
