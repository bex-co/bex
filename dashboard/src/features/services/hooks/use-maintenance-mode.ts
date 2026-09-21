import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetMaintenanceModeDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useAskForProtectedConfirmation } from "@/common/providers/protected-retry-context";
import {
  ProtectedConfirmationDismissed,
  withProtectedRetry,
} from "@/features/services/lib/protected-confirmation";

export interface UseMaintenanceModeResult {
  setMaintenanceMode: (
    id: string,
    enabled: boolean,
    uri: string,
  ) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Maintenance Mode toggle + custom-page field to bex-api's
 * `setMaintenanceMode` (w1/m37, Render's maintenanceMode object).
 */
export function useMaintenanceMode(): UseMaintenanceModeResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetMaintenanceModeDocument);
  const [busy, setBusy] = useState(false);
  const askForConfirmation = useAskForProtectedConfirmation();

  const setMaintenanceMode = useCallback(
    async (id: string, enabled: boolean, uri: string) => {
      setBusy(true);
      try {
        // Turning maintenance mode ON takes the service offline, which a
        // protected environment guards; turning it off restores it and does
        // not (w4/m126).
        await withProtectedRetry(askForConfirmation, (confirm) =>
          mutate({
            variables: { id, maintenanceMode: { enabled, uri }, confirm },
          }),
        );
        toast.success(
          enabled
            ? t("services.maintenanceModeEnabledSuccess")
            : t("services.maintenanceModeDisabledSuccess"),
        );
        return true;
      } catch (err) {
        if (err instanceof ProtectedConfirmationDismissed) return false;
        toast.error(
          mutationErrorMessage(err, t("services.maintenanceModeError")),
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, askForConfirmation],
  );

  return { setMaintenanceMode, busy };
}
