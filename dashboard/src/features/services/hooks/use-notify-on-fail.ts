import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetNotifyOnFailDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseNotifyOnFailResult {
  setNotifyOnFail: (id: string, value: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Notifications control to bex-api's `setNotifyOnFail`
 * (w4/m21) — the per-service deploy-failure notification override (Render's
 * exact notifyOnFail field/enum: default | notify | ignore,
 * docs/render-artifacts/notify-on-fail.md).
 */
export function useNotifyOnFail(): UseNotifyOnFailResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetNotifyOnFailDocument);
  const [busy, setBusy] = useState(false);

  const setNotifyOnFail = useCallback(
    async (id: string, value: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, value } });
        toast.success(t("services.notifyOnFailSuccess"));
        return true;
      } catch {
        toast.error(t("services.notifyOnFailError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setNotifyOnFail, busy };
}
