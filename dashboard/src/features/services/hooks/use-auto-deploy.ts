import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetAutoDeployDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseAutoDeployResult {
  /** Fires setAutoDeploy; resolves true on success (toasted either way). */
  setAutoDeploy: (id: string, enabled: boolean) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Build & Deploy Auto-Deploy toggle to bex-api's `setAutoDeploy`
 * (spec.autoDeploy). Flipping it changes whether a future git push redeploys the
 * service; it does not itself redeploy. On failure the caller reverts the
 * optimistic switch state.
 */
export function useAutoDeploy(): UseAutoDeployResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetAutoDeployDocument);
  const [busy, setBusy] = useState(false);

  const setAutoDeploy = useCallback(
    async (id: string, enabled: boolean) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, enabled } });
        toast.success(
          enabled
            ? t("services.autoDeployOnSuccess")
            : t("services.autoDeployOffSuccess"),
        );
        return true;
      } catch {
        toast.error(t("services.autoDeployError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setAutoDeploy, busy };
}
