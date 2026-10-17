import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SetAutoDeployDocument,
  SetAutoDeployTriggerDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

/** Render's Auto-Deploy choices (autoDeployTrigger, w1/m181). */
export type AutoDeployTrigger = "commit" | "checksPass" | "off";

export interface UseAutoDeployResult {
  /** Fires setAutoDeploy; resolves true on success (toasted either way). */
  setAutoDeploy: (id: string, enabled: boolean) => Promise<boolean>;
  /** Fires setAutoDeployTrigger; resolves true on success (toasted either way). */
  setAutoDeployTrigger: (
    id: string,
    trigger: AutoDeployTrigger,
  ) => Promise<boolean>;
  busy: boolean;
}

const TRIGGER_SUCCESS = {
  commit: "services.autoDeployOnSuccess",
  checksPass: "services.autoDeployChecksSuccess",
  off: "services.autoDeployOffSuccess",
} as const;

/**
 * Wires the Build & Deploy Auto-Deploy toggle to bex-api's `setAutoDeploy`
 * (spec.autoDeploy). Flipping it changes whether a future git push redeploys the
 * service; it does not itself redeploy. The mutation returns `{ id autoDeploy }`,
 * so the normalized Service carries the saved value before this resolves.
 */
export function useAutoDeploy(): UseAutoDeployResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetAutoDeployDocument);
  const [mutateTrigger] = useMutation(SetAutoDeployTriggerDocument);
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
      } catch (err) {
        toast.error(mutationErrorMessage(err, t("services.autoDeployError")));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  const setAutoDeployTrigger = useCallback(
    async (id: string, trigger: AutoDeployTrigger) => {
      setBusy(true);
      try {
        await mutateTrigger({ variables: { id, trigger } });
        toast.success(t(TRIGGER_SUCCESS[trigger]));
        return true;
      } catch (err) {
        // A refused checksPass names what the GitHub App must grant.
        toast.error(mutationErrorMessage(err, t("services.autoDeployError")));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutateTrigger, t],
  );

  return { setAutoDeploy, setAutoDeployTrigger, busy };
}
