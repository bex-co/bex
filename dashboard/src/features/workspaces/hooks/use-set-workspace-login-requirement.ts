import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import type { TypedDocumentNode } from "@apollo/client";
import { toast } from "sonner";
import {
  SetWorkspaceRequireGoogleLoginDocument,
  SetWorkspaceRequireTwoFactorDocument,
} from "@/graphql/definitions";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  graphQLErrorCodes,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { LOGIN_REQUIREMENT_SETTINGS } from "@/features/workspaces/lib/login-requirements";
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

export interface LoginRequirementRefusalText {
  /** Already in the user's language: locale copy for the coded refusals, else
   *  the server's own answer or a generic line (never a raw transport error). */
  text: string;
  /** The lock-out guard: the acting admin fixes it on their own account. */
  accountFix: boolean;
}

export interface UseSetWorkspaceLoginRequirementResult {
  /** Turns the workspace's requirement on or off. */
  setRequired: (enabled: boolean) => Promise<void>;
  busy: boolean;
  /** The refusal, shown inline beside the switch it came from. */
  error: LoginRequirementRefusalText | null;
}

/**
 * Wires a Workspace Settings login-requirement switch (2FA, w22/m402; Google
 * login, w7/m465) to its bex-api mutation, and reads the mutation's coded
 * refusals (`LOGIN_REQUIREMENT_SETTINGS`) into the card's own copy.
 */
export function useSetWorkspaceLoginRequirement(
  requirement: WorkspaceLoginRequirement,
  workspace: Pick<WorkspaceView, "id" | "name">,
): UseSetWorkspaceLoginRequirementResult {
  const { t } = useTranslations();
  const [mutate, { loading }] = useMutation(MUTATIONS[requirement]);
  const [error, setError] = useState<LoginRequirementRefusalText | null>(null);
  const { id, name } = workspace;
  const setting = LOGIN_REQUIREMENT_SETTINGS[requirement];

  const setRequired = useCallback(
    async (enabled: boolean) => {
      setError(null);
      try {
        await mutate({ variables: { workspaceId: id, enabled } });
        toast.success(
          t(enabled ? setting.enabledToastKey : setting.disabledToastKey, {
            name,
          }),
        );
      } catch (err) {
        const code = graphQLErrorCodes(err).find((c) =>
          Object.hasOwn(setting.refusals, c),
        );
        const coded = code ? setting.refusals[code] : undefined;
        setError(
          coded
            ? { text: t(coded.messageKey), accountFix: coded.accountFix }
            : {
                text: mutationErrorMessage(err, t(setting.errorKey)),
                accountFix: false,
              },
        );
      }
    },
    [mutate, id, name, t, setting],
  );

  return { setRequired, busy: loading, error };
}
