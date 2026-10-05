import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SyncBlueprintDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { SyncBlueprintResult } from "@/features/blueprints/types";
import { toSyncBlueprintResult } from "@/features/blueprints/lib/views";
import {
  blueprintConfirmationFromError,
  type BlueprintConfirmationRequired,
} from "@/features/blueprints/lib/takeover";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";

export type BlueprintSyncActionResult =
  | { status: "success"; result: SyncBlueprintResult | null }
  | { status: "source_changed" }
  | BlueprintConfirmationRequired
  | { status: "error" };

/** Reviewed Git source pinned at confirm time (w8/m41). */
export interface ReviewedBlueprintSource {
  commitId: string;
  path: string;
  repo: string;
}

export interface UseSyncBlueprintResult {
  sync: (
    id: string,
    opts: { reviewed: ReviewedBlueprintSource; confirmation?: string },
  ) => Promise<BlueprintSyncActionResult>;
  busy: boolean;
}

export function useSyncBlueprint(): UseSyncBlueprintResult {
  const { t } = useTranslations();
  // A sync records a run and moves the blueprint's status: re-read both, so
  // Sync History gains its new top row without a reload (w4/m138).
  const [mutate] = useMutation(SyncBlueprintDocument, {
    refetchQueries: ["BlueprintSyncs", "Blueprint"],
  });
  const [busy, setBusy] = useState(false);
  const paymentGate = usePaymentRequiredGate();

  const sync = useCallback(
    async (
      id: string,
      opts: { reviewed: ReviewedBlueprintSource; confirmation?: string },
    ): Promise<BlueprintSyncActionResult> => {
      setBusy(true);
      try {
        const res = await paymentGate.run(() =>
          mutate({
            variables: {
              id,
              confirm: opts.confirmation,
              commitId: opts.reviewed.commitId,
              path: opts.reviewed.path,
              repo: opts.reviewed.repo,
            },
          }),
        );
        const result = res.data?.syncBlueprint
          ? toSyncBlueprintResult(res.data.syncBlueprint)
          : null;
        if (result?.detachedResources.length) {
          toast.warning(t("blueprints.detachSuccessTitle"), {
            description: `${t("blueprints.detachWarning")} ${result.detachedResources.map((resource) => `${resource.name} (${resource.id})`).join(", ")}`,
          });
        } else {
          toast.success(t("blueprints.syncSuccess"));
        }
        return { status: "success", result };
      } catch (err) {
        if (isPaymentOnboardingCancelled(err)) return { status: "error" };
        const confirmation = blueprintConfirmationFromError(err);
        if (confirmation) return confirmation;
        if (hasGraphQLErrorCode(err, "BLUEPRINT_SOURCE_CHANGED")) {
          toast.error(t("blueprints.syncSourceChanged"));
          return { status: "source_changed" };
        }
        // A fenced sync is actionable, not a failure of the manifest: tell the
        // caller to retry after the recorded run settles (w8/m37 t005).
        if (hasGraphQLErrorCode(err, "BLUEPRINT_SYNC_BUSY")) {
          toast.error(t("blueprints.syncBusy"));
        } else {
          toast.error(mutationErrorMessage(err, t("blueprints.syncError")));
        }
        return { status: "error" };
      } finally {
        setBusy(false);
      }
    },
    [mutate, paymentGate, t],
  );

  return { sync, busy };
}
