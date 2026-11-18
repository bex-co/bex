import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import type { TypedDocumentNode } from "@apollo/client";
import {
  SetWorkspaceRequireGoogleLoginDocument,
  SetWorkspaceRequireTwoFactorDocument,
} from "@/graphql/definitions";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { LOGIN_REQUIREMENT_SETTINGS } from "@/features/workspaces/lib/login-requirements";
import {
  useLoginRequirementSetter,
  type UseLoginRequirementSetterResult,
} from "@/features/workspaces/hooks/use-login-requirement-setter";
import type { WorkspaceView } from "@/features/workspaces/types";

/**
 * Each requirement's admin-only mutation (`setWorkspaceRequireTwoFactor`,
 * `setWorkspaceRequireGoogleLogin`). Both take `{workspaceId, enabled}` and
 * return the workspace's `id` and the setting, so Apollo's normalized cache
 * updates the `Workspaces` list the card reads from without a refetch.
 */
const MUTATIONS: Record<
  WorkspaceLoginRequirement,
  TypedDocumentNode<unknown, { workspaceId: string; enabled: boolean }>
> = {
  twoFactor: SetWorkspaceRequireTwoFactorDocument,
  googleLogin: SetWorkspaceRequireGoogleLoginDocument,
};

/**
 * Wires a Workspace Settings login-requirement switch (2FA, w22/m402; Google
 * login, w7/m465) to its bex-api mutation, and reads the mutation's coded
 * refusals (`LOGIN_REQUIREMENT_SETTINGS`) into the card's own copy.
 */
export function useSetWorkspaceLoginRequirement(
  requirement: WorkspaceLoginRequirement,
  workspace: Pick<WorkspaceView, "id" | "name">,
): UseLoginRequirementSetterResult {
  const [mutate, { loading }] = useMutation(MUTATIONS[requirement]);
  const { id } = workspace;
  const run = useCallback(
    (enabled: boolean) => mutate({ variables: { workspaceId: id, enabled } }),
    [mutate, id],
  );
  return useLoginRequirementSetter({
    run,
    busy: loading,
    copy: LOGIN_REQUIREMENT_SETTINGS[requirement],
    name: workspace.name,
  });
}
