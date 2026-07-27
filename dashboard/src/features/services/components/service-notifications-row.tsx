import { useMemo } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServiceNotifications } from "@/features/services/hooks/use-service-notifications";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";

const OPTIONS = [
  { value: "default", labelKey: "services.notificationsOptionDefault" },
  { value: "all", labelKey: "services.notificationsOptionAll" },
  { value: "failure", labelKey: "services.notificationsOptionFailure" },
  { value: "none", labelKey: "services.notificationsOptionNone" },
] as const;

export interface ServiceNotificationsRowProps {
  serviceId: string;
  notificationsToSend: string | null | undefined;
}

/** The per-service deploy-notification override — the select variant of the
 *  shared edit-in-place row (Render's disabled `notificationsToSend` select). */
export function ServiceNotificationsRow({
  serviceId,
  notificationsToSend,
}: ServiceNotificationsRowProps) {
  const { t } = useTranslations();
  const { setNotificationsToSend, busy } = useServiceNotifications();
  const current = notificationsToSend || "default";
  // Stable identity so the row's focus effect doesn't re-run on every render.
  const options = useMemo(
    () => OPTIONS.map((o) => ({ value: o.value, label: t(o.labelKey) })),
    [t],
  );

  return (
    <EditableFieldRow
      label={t("services.notificationsLabel")}
      hint={t("services.notificationsHint")}
      value={current}
      editLabel={t("services.notificationsEdit")}
      busy={busy}
      options={options}
      onSave={(value) => setNotificationsToSend(serviceId, value)}
    />
  );
}
