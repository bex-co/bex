import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  UpdateApiKeyPermissionsDocument,
  type ApiKeyPermissionsInput,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UseUpdateApiKeyPermissionsResult {
  /** Fires updateApiKeyPermissions; resolves true on success (toasted either way). */
  update: (
    id: string,
    name: string,
    permissions: ApiKeyPermissionsInput,
  ) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the permissions editor (a key row's gear, w9/m496) to bex-api's
 * `updateApiKeyPermissions`. The answer carries the key's id and new grant, so
 * Apollo refreshes the cached row in place. A refusal is the server's own
 * words: only the key's creator or a workspace Admin may change it
 * (`api_key_permissions_editor_forbidden`), and never past the creator's role
 * (`api_key_permission_above_role`).
 */
export function useUpdateApiKeyPermissions(): UseUpdateApiKeyPermissionsResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(UpdateApiKeyPermissionsDocument);
  const [busy, setBusy] = useState(false);

  const update = useCallback(
    async (id: string, name: string, permissions: ApiKeyPermissionsInput) => {
      // The key's own workspace (w6/m18), never a null ownerId the backend
      // would route to the caller's default workspace.
      if (currentWorkspaceId == null) {
        toast.error(t("apiKeys.permissionsSaveError", { name }));
        return false;
      }
      setBusy(true);
      try {
        const res = await mutate({
          variables: { id, ownerId: currentWorkspaceId, permissions },
        });
        if (!res.data?.updateApiKeyPermissions) {
          throw new Error("updateApiKeyPermissions returned no key");
        }
        toast.success(t("apiKeys.permissionsSaved", { name }));
        return true;
      } catch (err) {
        toast.error(
          mutationErrorMessage(
            err,
            t("apiKeys.permissionsSaveError", { name }),
          ),
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { update, busy };
}
