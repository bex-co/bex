import { useQuery } from "@apollo/client/react";
import { OrganizationLoginRequirementImpactDocument } from "@/graphql/definitions";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import {
  LOGIN_REQUIREMENT_ENUM,
  loginRequirementImpactState,
  type LoginRequirementImpactState,
} from "@/features/workspaces/hooks/use-login-requirement-impact";

/**
 * Who turning `requirement` on for the org would block in any of its
 * workspaces (w7/m471), through bex-api's
 * `organizationLoginRequirementImpact`, each API key naming its workspace:
 * read fresh each time the Security card's confirmation opens.
 */
export function useOrganizationLoginRequirementImpact(
  organizationId: string,
  requirement: WorkspaceLoginRequirement,
  open: boolean,
): LoginRequirementImpactState {
  const { data, error } = useQuery(OrganizationLoginRequirementImpactDocument, {
    variables: {
      organizationId,
      requirement: LOGIN_REQUIREMENT_ENUM[requirement],
    },
    skip: !open,
    fetchPolicy: "network-only",
  });
  return loginRequirementImpactState(
    data?.organizationLoginRequirementImpact,
    error,
  );
}
