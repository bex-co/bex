import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SyncBlueprintDocument } from "@/features/blueprints/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { SyncBlueprintResult } from "@/features/blueprints/types";

export interface UseSyncBlueprintResult {
  sync: (id: string) => Promise<SyncBlueprintResult | null>;
  busy: boolean;
}

export function useSyncBlueprint(): UseSyncBlueprintResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(SyncBlueprintDocument);
  const [busy, setBusy] = useState(false);

  const sync = useCallback(
    async (id: string): Promise<SyncBlueprintResult | null> => {
      setBusy(true);
      try {
        const res = await mutate({
          variables: { id, ownerId: currentWorkspaceId },
        });
        const result = res.data?.syncBlueprint ?? null;
        toast.success(t("blueprints.syncSuccess"));
        return result;
      } catch {
        toast.error(t("blueprints.syncError"));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { sync, busy };
}
