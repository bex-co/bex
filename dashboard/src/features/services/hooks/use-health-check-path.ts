import { SetHealthCheckPathDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

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
  const { run, busy } = useFieldMutation(
    SetHealthCheckPathDocument,
    (id: string, path: string) => ({ id, path }),
    {
      success: "services.healthCheckPathSuccess",
      error: "services.healthCheckPathError",
    },
  );

  return { setHealthCheckPath: run, busy };
}
