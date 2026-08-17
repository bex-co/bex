import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CreateBlueprintDocument,
  type BlueprintEnvVarValueInput,
} from "@/features/blueprints/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { BlueprintView } from "@/features/blueprints/types";
import {
  protectedConfirmationFromError,
  type ProtectedActionResult,
} from "@/features/services/lib/protected-confirmation";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";

export type BlueprintCreateActionResult =
  | { status: "success"; blueprint: BlueprintView }
  | Exclude<ProtectedActionResult, { status: "success" }>;

export interface UseCreateBlueprintResult {
  create: (
    repo: string,
    branch: string,
    path: string,
    name: string,
    confirmation?: string,
    envVarValues?: BlueprintEnvVarValueInput[],
  ) => Promise<BlueprintCreateActionResult>;
  busy: boolean;
}

export function useCreateBlueprint(): UseCreateBlueprintResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(CreateBlueprintDocument);
  const [busy, setBusy] = useState(false);
  const paymentGate = usePaymentRequiredGate();

  const create = useCallback(
    async (
      repo: string,
      branch: string,
      path: string,
      name: string,
      confirmation?: string,
      envVarValues?: BlueprintEnvVarValueInput[],
    ): Promise<BlueprintCreateActionResult> => {
      setBusy(true);
      try {
        const res = await paymentGate.run(() =>
          mutate({
            variables: {
              repo,
              branch,
              path: path || "render.yaml",
              name: name || undefined,
              confirm: confirmation,
              ownerId: currentWorkspaceId,
              envVarValues: envVarValues?.length ? envVarValues : undefined,
            },
          }),
        );
        const blueprint = res.data?.createBlueprint;
        if (!blueprint) {
          toast.error(t("blueprints.createError"));
          return { status: "error" };
        }
        toast.success(t("blueprints.createSuccess"));
        return { status: "success", blueprint };
      } catch (err) {
        if (isPaymentOnboardingCancelled(err)) return { status: "error" };
        const requiredConfirmation = protectedConfirmationFromError(err);
        if (requiredConfirmation) {
          return {
            status: "confirmation_required",
            confirmation: requiredConfirmation,
          };
        }
        toast.error(t("blueprints.createError"));
        return { status: "error" };
      } finally {
        setBusy(false);
      }
    },
    [mutate, paymentGate, t, currentWorkspaceId],
  );

  return { create, busy };
}
