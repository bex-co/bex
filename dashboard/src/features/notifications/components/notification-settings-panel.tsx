import { AlertTriangle } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { PanelCenteredState } from "@/common/components/panel-states";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useNotificationSettings } from "@/features/notifications/hooks/use-notification-settings";
import { useUpdateNotificationSettings } from "@/features/notifications/hooks/use-update-notification-settings";

type Field = "deploySucceeded" | "deployFailed";

/**
 * Settings → Notifications (w3/m9): the caller's own deploy-email
 * preferences — succeed/fail on any of their workspace's services, matching
 * Render's /notification-settings. Self-service (no per-role gate beyond
 * workspace membership), so every toggle acts on the signed-in caller alone.
 */
export function NotificationSettingsPanel() {
  const { t } = useTranslations();
  const { settings, loading, error } = useNotificationSettings();
  const { update, busy } = useUpdateNotificationSettings();

  const initialLoading = loading && !error;

  // The mutation writes its response straight into the query's cache entry
  // (see useUpdateNotificationSettings), so `settings` above already reflects
  // a successful toggle — no refetch needed here.
  function toggle(field: Field, value: boolean) {
    void update({ ...settings, [field]: value });
  }

  const rows: Array<{ field: Field; labelKey: string; hintKey: string }> = [
    {
      field: "deploySucceeded",
      labelKey: "notifications.deploySucceeded",
      hintKey: "notifications.deploySucceededHint",
    },
    {
      field: "deployFailed",
      labelKey: "notifications.deployFailed",
      hintKey: "notifications.deployFailedHint",
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("notifications.title")}</CardTitle>
        <CardDescription>{t("notifications.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {error ? (
          <PanelCenteredState
            icon={<AlertTriangle />}
            title={t("notifications.errorTitle")}
            body={t("notifications.errorBody")}
          />
        ) : initialLoading ? (
          <div className="space-y-4">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : (
          <div className="space-y-3">
            {rows.map(({ field, labelKey, hintKey }) => (
              <div
                key={field}
                className="flex items-center justify-between rounded-md border p-3"
              >
                <div className="space-y-0.5 pr-4">
                  <Label htmlFor={`notif-${field}`}>{t(labelKey)}</Label>
                  <p className="text-sm text-muted-foreground">
                    {t(hintKey)}
                  </p>
                </div>
                <Switch
                  id={`notif-${field}`}
                  checked={settings[field]}
                  disabled={busy}
                  onCheckedChange={(v) => toggle(field, v)}
                />
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
