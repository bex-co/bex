import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SyncBlueprintDocument } from "@/features/blueprints/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { SyncBlueprintResult } from "@/features/blueprints/types";
import {
  protectedConfirmationFromError,
  type ProtectedActionResult,
} from "@/features/services/lib/protected-confirmation";

export type BlueprintSyncActionResult =
  | { status: "success"; result: SyncBlueprintResult | null }
  | Exclude<ProtectedActionResult, { status: "success" }>;

export interface UseSyncBlueprintResult {
  sync: (
    id: string,
    confirmation?: string,
  ) => Promise<BlueprintSyncActionResult>;
  busy: boolean;
}

export function useSyncBlueprint(): UseSyncBlueprintResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(SyncBlueprintDocument);
  const [busy, setBusy] = useState(false);

  const sync = useCallback(
    async (
      id: string,
      confirmation?: string,
    ): Promise<BlueprintSyncActionResult> => {
      setBusy(true);
      try {
        const res = await mutate({
          variables: {
            id,
            ownerId: currentWorkspaceId,
            confirm: confirmation,
          },
        });
        const result = res.data?.syncBlueprint ?? null;
        toast.success(t("blueprints.syncSuccess"));
        return { status: "success", result };
      } catch (err) {
        const requiredConfirmation = protectedConfirmationFromError(err);
        if (requiredConfirmation) {
          return {
            status: "confirmation_required",
            confirmation: requiredConfirmation,
          };
        }
        toast.error(t("blueprints.syncError"));
        return { status: "error" };
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { sync, busy };
}
