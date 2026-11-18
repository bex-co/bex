import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import type { TypedDocumentNode } from "@apollo/client";
import {
  SetOrganizationRequireGoogleLoginDocument,
  SetOrganizationRequireTwoFactorDocument,
} from "@/graphql/definitions";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { LOGIN_REQUIREMENT_SETTINGS } from "@/features/workspaces/lib/login-requirements";
import {
  useLoginRequirementSetter,
  type UseLoginRequirementSetterResult,
} from "@/features/workspaces/hooks/use-login-requirement-setter";
import { ORGANIZATION_LOGIN_REQUIREMENT_SETTINGS } from "@/features/organizations/lib/organization-login-requirements";
import { organizationErrorMessage } from "@/features/organizations/lib/organization-errors";
import type { Organization } from "@/features/organizations/types";

/**
 * Each org requirement's Owner-only mutation (w7/m466):
 * `setOrganizationRequireTwoFactor`, `setOrganizationRequireGoogleLogin`.
 * Both take `{organizationId, enabled}` and answer the whole org, which the
 * cache merges into the page and the switcher's entry for it.
 */
const MUTATIONS: Record<
  WorkspaceLoginRequirement,
  TypedDocumentNode<unknown, { organizationId: string; enabled: boolean }>
> = {
  twoFactor: SetOrganizationRequireTwoFactorDocument,
  googleLogin: SetOrganizationRequireGoogleLoginDocument,
};

/**
 * Wires an Org Settings -> Security switch to its mutation, with the
 * workspace switches' outcome handling (`useLoginRequirementSetter`) in the
 * org's own words; a failure that is not the setting's own is worded as every
 * org action's (`organizationErrorMessage`: FORBIDDEN for a caller no longer
 * an Owner). The workspaces list carries the setting too, on each
 * workspace's org (`WorkspaceOrganization`), where it locks that workspace's
 * own switch; a change writes it there as well, so Workspace Settings agrees
 * without a refetch.
 */
export function useSetOrganizationLoginRequirement(
  requirement: WorkspaceLoginRequirement,
  organization: Pick<Organization, "id" | "name">,
): UseLoginRequirementSetterResult {
  const [mutate, { loading }] = useMutation(MUTATIONS[requirement]);
  const { id } = organization;
  const field = LOGIN_REQUIREMENT_SETTINGS[requirement].field;
  const run = useCallback(
    (enabled: boolean) =>
      mutate({
        variables: { organizationId: id, enabled },
        update: (cache) => {
          cache.modify({
            id: cache.identify({ __typename: "WorkspaceOrganization", id }),
            fields: { [field]: () => enabled },
          });
        },
      }),
    [mutate, id, field],
  );
  return useLoginRequirementSetter({
    run,
    busy: loading,
    copy: ORGANIZATION_LOGIN_REQUIREMENT_SETTINGS[requirement],
    name: organization.name,
    refusalMessage: organizationErrorMessage,
  });
}
