import { useTranslations } from "@/common/hooks/use-translations";
import { useHealthCheckPath } from "@/features/services/hooks/use-health-check-path";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";

export interface HealthCheckPathRowProps {
  serviceId: string;
  /** Current spec.healthCheckPath; null/empty means the TCP check (w7/m80). */
  healthCheckPath: string | null | undefined;
}

/**
 * Settings row for the health-check path (w1/m23/t001) — what the platform
 * polls before routing traffic to the service. Uses the shared edit-in-place
 * row. Only shown for web_service and private_service; the settings page gates
 * it.
 *
 * Empty is a real, reachable state (w7/m80): it clears the path and selects a
 * TCP check that only verifies the process is listening. This row must not
 * coerce empty back to "/" — it used to, which made the field one-way and left
 * the TCP mode unreachable from the dashboard even after the API supported it.
 */
export function HealthCheckPathRow({
  serviceId,
  healthCheckPath,
}: HealthCheckPathRowProps) {
  const { t } = useTranslations();
  const { setHealthCheckPath, busy } = useHealthCheckPath();
  const current = healthCheckPath ?? "";

  return (
    <EditableFieldRow
      label={t("services.settingsHealthCheckPath")}
      hint={t("services.settingsHealthCheckPathHint")}
      value={current}
      placeholder={t("services.settingsHealthCheckPathPlaceholder")}
      editLabel={t("services.settingsHealthCheckPathEdit")}
      mono
      busy={busy}
      // A path may carry meaningful characters, so compare the raw draft.
      // Empty is passed through, not coerced: it clears the path and selects
      // the TCP check.
      trim={false}
      onSave={(value) => setHealthCheckPath(serviceId, value)}
    />
  );
}
