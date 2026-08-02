import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { DisconnectBlueprintDocument } from "@/features/blueprints/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";

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
      } catch {
        toast.error(t("blueprints.disconnectError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { disconnect, busy };
}
