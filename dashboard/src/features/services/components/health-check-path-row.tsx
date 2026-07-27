import { useTranslations } from "@/common/hooks/use-translations";
import { useHealthCheckPath } from "@/features/services/hooks/use-health-check-path";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";

export interface HealthCheckPathRowProps {
  serviceId: string;
  /** Current spec.healthCheckPath; null/empty means the platform default "/". */
  healthCheckPath: string | null | undefined;
}

/**
 * Settings row for the ReadinessProbe HTTP path (w1/m23/t001) — the path the
 * platform polls before routing traffic to the service. Uses the shared
 * edit-in-place row. Only shown for web_service and private_service; the
 * settings page gates it.
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
      // A path may carry meaningful characters, so compare the raw draft; an
      // empty path restores the platform default "/".
      trim={false}
      onSave={(value) => setHealthCheckPath(serviceId, value || "/")}
    />
  );
}
