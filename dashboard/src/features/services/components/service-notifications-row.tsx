import { useMemo } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServiceNotifications } from "@/features/services/hooks/use-service-notifications";
import { useOwnerNotificationSettings } from "@/features/notifications/hooks/use-owner-notification-settings";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { SERVICE_NOTIFICATION_LEVEL_LABEL_KEYS } from "@/features/notifications/lib/notification-levels";

const LEVEL_LABEL_KEYS = SERVICE_NOTIFICATION_LEVEL_LABEL_KEYS;

const OPTIONS = [
  { value: "default", labelKey: null },
  { value: "all", labelKey: LEVEL_LABEL_KEYS.all },
  { value: "failure", labelKey: LEVEL_LABEL_KEYS.failure },
  { value: "none", labelKey: LEVEL_LABEL_KEYS.none },
] as const;

export interface ServiceNotificationsRowProps {
  serviceId: string;
  /** The service's own workspace, whose notification policy "default" follows. */
  ownerId: string | null | undefined;
  notificationsToSend: string | null | undefined;
}

/** The per-service deploy-notification override — the select variant of the
 *  shared edit-in-place row (Render's disabled `notificationsToSend` select).
 *  "Use workspace default" names the workspace's saved level (w3/m271). */
export function ServiceNotificationsRow({
  serviceId,
  ownerId,
  notificationsToSend,
}: ServiceNotificationsRowProps) {
  const { t } = useTranslations();
  const { setNotificationsToSend, busy } = useServiceNotifications();
  const { settings } = useOwnerNotificationSettings(ownerId);
  const current = notificationsToSend || "default";
  const workspaceLevel = settings?.notificationsToSend;
  // Stable identity so the row's focus effect doesn't re-run on every render.
  const options = useMemo(() => {
    const levelKey =
      workspaceLevel && workspaceLevel in LEVEL_LABEL_KEYS
        ? LEVEL_LABEL_KEYS[workspaceLevel as keyof typeof LEVEL_LABEL_KEYS]
        : null;
    const defaultLabel = levelKey
      ? t("services.notificationsOptionDefault", { level: t(levelKey) })
      : t("services.notificationsOptionDefaultUnknown");
    return OPTIONS.map((o) => ({
      value: o.value,
      label: o.labelKey ? t(o.labelKey) : defaultLabel,
    }));
  }, [t, workspaceLevel]);

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
