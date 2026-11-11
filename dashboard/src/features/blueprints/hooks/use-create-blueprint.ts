import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CreateBlueprintDocument,
  type BlueprintEnvVarValueInput,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { BlueprintView } from "@/features/blueprints/types";
import { toBlueprintView } from "@/features/blueprints/lib/views";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import {
  blueprintConfirmationFromError,
  type BlueprintConfirmationRequired,
} from "@/features/blueprints/lib/takeover";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";

export type BlueprintCreateActionResult =
  | { status: "success"; blueprint: BlueprintView }
  | BlueprintConfirmationRequired
  | { status: "error" };

/**
 * Render's existingResources "create_new" (w8/m153): the Blueprint gets its
 * own set of resources, each name carrying the suffix the preview reviewed, and
 * the workspace's existing resources are left as they are.
 */
interface BlueprintCreateAsNew {
  existingResources: "create_new";
  /** null lets bex-api choose the suffix. */
  resourceSuffix: string | null;
}

export interface UseCreateBlueprintResult {
  create: (
    repo: string,
    branch: string,
    path: string,
    name: string,
    confirmation?: string,
    envVarValues?: BlueprintEnvVarValueInput[],
    /** Omitted: the Blueprint manages the names its file gives. */
    asNew?: BlueprintCreateAsNew,
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
      asNew?: BlueprintCreateAsNew,
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
              existingResources: asNew?.existingResources,
              resourceSuffix: asNew?.resourceSuffix ?? undefined,
            },
          }),
        );
        const blueprint = res.data?.createBlueprint;
        if (!blueprint) {
          toast.error(t("blueprints.createError"));
          return { status: "error" };
        }
        toast.success(t("blueprints.createSuccess"));
        return { status: "success", blueprint: toBlueprintView(blueprint) };
      } catch (err) {
        if (isPaymentOnboardingCancelled(err)) return { status: "error" };
        const confirmation = blueprintConfirmationFromError(err);
        if (confirmation) return confirmation;
        toast.error(mutationErrorMessage(err, t("blueprints.createError")));
        return { status: "error" };
      } finally {
        setBusy(false);
      }
    },
    [mutate, paymentGate, t, currentWorkspaceId],
  );

  return { create, busy };
}
