import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetMaintenanceModeDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

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

  const setMaintenanceMode = useCallback(
    async (id: string, enabled: boolean, uri: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, maintenanceMode: { enabled, uri } } });
        toast.success(
          enabled
            ? t("services.maintenanceModeEnabledSuccess")
            : t("services.maintenanceModeDisabledSuccess"),
        );
        return true;
      } catch {
        toast.error(t("services.maintenanceModeError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setMaintenanceMode, busy };
}
