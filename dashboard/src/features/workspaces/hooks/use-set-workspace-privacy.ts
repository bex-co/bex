import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetWorkspacePrivacyDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import type { WorkspacePrivacy } from "@/features/workspaces/types";

export interface UseSetWorkspacePrivacyResult {
  /** Sets the workspace's privacy; resolves true once it is saved. */
  save: (privacy: WorkspacePrivacy) => Promise<boolean>;
  saving: boolean;
}

/**
 * Wires Workspace Settings' privacy card to bex-api's `setWorkspacePrivacy`
 * (w7/m464, ADR115: a workspace admin's setting, `can_manage`). The mutation
 * answers the Workspace's id and privacy, which the cache merges into the
 * workspaces list, so the switcher and every other reader of the workspace
 * agree on the new setting. A refusal is the server's own reason, else a
 * generic line, as a toast.
 */
export function useSetWorkspacePrivacy(
  workspaceId: string,
): UseSetWorkspacePrivacyResult {
  const { t } = useTranslations();
  const [mutate, { loading }] = useMutation(SetWorkspacePrivacyDocument);

  const save = useCallback(
    async (privacy: WorkspacePrivacy) => {
      try {
        await mutate({ variables: { workspaceId, privacy } });
      } catch (err) {
        toast.error(mutationErrorMessage(err, t("workspaces.privacyError")));
        return false;
      }
      toast.success(t("workspaces.privacySaved"));
      return true;
    },
    [mutate, t, workspaceId],
  );

  return { save, saving: loading };
}
