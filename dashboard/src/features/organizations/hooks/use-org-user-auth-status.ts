import { useQuery } from "@apollo/client/react";
import { OrgUserAuthStatusDocument } from "@/graphql/definitions";

/**
 * Render's orgUserAuthStatus (w1/m657): a member's login methods and 2FA,
 * asked only while their details are open and only by an Owner (Render's
 * viewOrgUserAuthStatus). `status` is undefined while it loads, when it
 * could not be read, and for anyone else.
 */
export function useOrgUserAuthStatus(
  organizationId: string,
  userId: string,
  canView: boolean,
) {
  const { data, loading } = useQuery(OrgUserAuthStatusDocument, {
    variables: { userId, organizationId },
    skip: !canView,
    fetchPolicy: "network-only",
    errorPolicy: "all",
  });
  return {
    status: data?.orgUserAuthStatus,
    loading: canView && loading && !data,
  };
}
