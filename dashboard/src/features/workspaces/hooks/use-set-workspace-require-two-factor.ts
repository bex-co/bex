import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetWorkspaceRequireTwoFactorDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  graphQLErrorCodes,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import type { WorkspaceView } from "@/features/workspaces/types";

/**
 * The mutation's coded 409 refusals (docs/ADR012-auth.md 9a), each with its own
 * locale string: the server's wording is not what the dashboard decides by or
 * shows for these two.
 */
const CODED_REFUSALS: Record<
  string,
  { messageKey: string; needsEnrollment: boolean }
> = {
  WORKSPACE_2FA_ADMIN_NOT_ENROLLED: {
    messageKey: "workspaces.twoFactorErrorAdminNotEnrolled",
    needsEnrollment: true,
  },
  WORKSPACE_2FA_UNAVAILABLE: {
    messageKey: "workspaces.twoFactorErrorUnavailable",
    needsEnrollment: false,
  },
};

export interface RequireTwoFactorRefusal {
  /** Already in the user's language: locale copy for the coded refusals, else
   *  the server's own answer or a generic line (never a raw transport error). */
  text: string;
  /** The acting admin has no second factor: the fix is account security. */
  needsEnrollment: boolean;
}

export interface UseSetWorkspaceRequireTwoFactorResult {
  /** Turns the workspace's requirement on or off. */
  setRequired: (enabled: boolean) => Promise<void>;
  busy: boolean;
  /** The refusal, shown inline beside the switch it came from. */
  error: RequireTwoFactorRefusal | null;
}

/**
 * Wires Workspace Settings' two-factor switch to bex-api's
 * `setWorkspaceRequireTwoFactor` (admin-only). The returned workspace carries
 * `id` + `requireTwoFactor`, so Apollo's normalized cache updates the
 * `Workspaces` list the card reads from without a refetch.
 */
export function useSetWorkspaceRequireTwoFactor(
  workspace: Pick<WorkspaceView, "id" | "name">,
): UseSetWorkspaceRequireTwoFactorResult {
  const { t } = useTranslations();
  const [mutate, { loading }] = useMutation(
    SetWorkspaceRequireTwoFactorDocument,
  );
  const [error, setError] = useState<RequireTwoFactorRefusal | null>(null);
  const { id, name } = workspace;

  const setRequired = useCallback(
    async (enabled: boolean) => {
      setError(null);
      try {
        await mutate({ variables: { workspaceId: id, enabled } });
        toast.success(
          t(
            enabled
              ? "workspaces.twoFactorEnabledToast"
              : "workspaces.twoFactorDisabledToast",
            { name },
          ),
        );
      } catch (err) {
        const coded = graphQLErrorCodes(err)
          .map((code) => CODED_REFUSALS[code])
          .find(Boolean);
        setError(
          coded
            ? {
                text: t(coded.messageKey),
                needsEnrollment: coded.needsEnrollment,
              }
            : {
                text: mutationErrorMessage(err, t("workspaces.twoFactorError")),
                needsEnrollment: false,
              },
        );
      }
    },
    [mutate, id, name, t],
  );

  return { setRequired, busy: loading, error };
}
