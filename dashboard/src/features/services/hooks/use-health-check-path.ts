import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetHealthCheckPathDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseHealthCheckPathResult {
  setHealthCheckPath: (id: string, path: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings health-check path control to bex-api's `setHealthCheckPath`
 * (w1/m23/t001) — the HTTP path the ReadinessProbe polls before routing traffic.
 * Only valid for web_service and private_service; the settings page hides this
 * row for cron_job / background_worker / static_site.
 */
export function useHealthCheckPath(): UseHealthCheckPathResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetHealthCheckPathDocument);
  const [busy, setBusy] = useState(false);

  const setHealthCheckPath = useCallback(
    async (id: string, path: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, path } });
        toast.success(t("services.healthCheckPathSuccess"));
        return true;
      } catch {
        toast.error(t("services.healthCheckPathError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setHealthCheckPath, busy };
}
