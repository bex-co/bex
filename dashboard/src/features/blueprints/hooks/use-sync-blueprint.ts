import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SyncBlueprintDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { SyncBlueprintResult } from "@/features/blueprints/types";
import { toSyncBlueprintResult } from "@/features/blueprints/lib/views";
import {
  protectedConfirmationFromError,
  type ProtectedActionResult,
} from "@/features/services/lib/protected-confirmation";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";

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
  const paymentGate = usePaymentRequiredGate();

  const sync = useCallback(
    async (
      id: string,
      confirmation?: string,
    ): Promise<BlueprintSyncActionResult> => {
      setBusy(true);
      try {
        const res = await paymentGate.run(() =>
          mutate({
            variables: {
              id,
              ownerId: currentWorkspaceId,
              confirm: confirmation,
            },
          }),
        );
        const result = res.data?.syncBlueprint
          ? toSyncBlueprintResult(res.data.syncBlueprint)
          : null;
        toast.success(t("blueprints.syncSuccess"));
        return { status: "success", result };
      } catch (err) {
        if (isPaymentOnboardingCancelled(err)) return { status: "error" };
        const requiredConfirmation = protectedConfirmationFromError(err);
        if (requiredConfirmation) {
          return {
            status: "confirmation_required",
            confirmation: requiredConfirmation,
          };
        }
        // A fenced sync is actionable, not a failure of the manifest: tell the
        // caller to retry after the recorded run settles (w8/m37 t005).
        if (hasGraphQLErrorCode(err, "BLUEPRINT_SYNC_BUSY")) {
          toast.error(t("blueprints.syncBusy"));
        } else {
          toast.error(t("blueprints.syncError"));
        }
        return { status: "error" };
      } finally {
        setBusy(false);
      }
    },
    [mutate, paymentGate, t, currentWorkspaceId],
  );

  return { sync, busy };
}
