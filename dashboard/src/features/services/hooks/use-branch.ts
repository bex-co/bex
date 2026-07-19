import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetBranchDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseBranchResult {
  /** Fires setBranch; resolves true on success (toasted either way). */
  setBranch: (id: string, branch: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Build & Deploy section's Branch control to bex-api's
 * `setBranch` (w5/m48/t005) — Render's editable Branch field. The mutation
 * patches `spec.branch` through the shared source verb, so the next deploy
 * builds the new branch and push-to-deploy matches pushes against it; the
 * toast confirms the write rather than implying an instant rebuild.
 */
export function useBranch(): UseBranchResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetBranchDocument);
  const [busy, setBusy] = useState(false);

  const setBranch = useCallback(
    async (id: string, branch: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, branch } });
        toast.success(t("services.buildDeploySuccess"));
        return true;
      } catch {
        toast.error(t("services.buildDeployError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setBranch, busy };
}
