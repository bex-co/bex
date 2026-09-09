import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DisconnectBlueprintDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";

export interface UseDisconnectBlueprintResult {
  disconnect: (id: string) => Promise<boolean>;
  busy: boolean;
}

export function useDisconnectBlueprint(): UseDisconnectBlueprintResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(DisconnectBlueprintDocument);
  const [busy, setBusy] = useState(false);

  const disconnect = useCallback(
    async (id: string): Promise<boolean> => {
      setBusy(true);
      try {
        await mutate({ variables: { id, ownerId: currentWorkspaceId } });
        toast.success(t("blueprints.disconnectSuccess"));
        return true;
      } catch (err) {
        // Disconnect refused while an apply owns the claim: retry after it
        // settles (w8/m37 t005).
        if (hasGraphQLErrorCode(err, "BLUEPRINT_SYNC_BUSY")) {
          toast.error(t("blueprints.disconnectBusy"));
        } else {
          toast.error(t("blueprints.disconnectError"));
        }
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { disconnect, busy };
}
