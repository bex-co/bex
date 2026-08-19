import { Bell, BellOff } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { usePushNotificationSettings } from "@/features/notifications/hooks/use-push-notification-settings";
import { useWebPushSubscription } from "@/features/notifications/hooks/use-web-push-subscription";

export function WebPushSettingsPanel() {
  const { t } = useTranslations();
  const { webPushAvailable, vapidPublicKey, loading } =
    usePushNotificationSettings();
  const { status, subscribe, unsubscribe } = useWebPushSubscription(
    vapidPublicKey ?? "",
    webPushAvailable ?? false,
  );

  if (loading) {
    return null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("notifications.webPushTitle")}</CardTitle>
        <CardDescription>{t("notifications.webPushDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {status === "unsupported" ? (
          <p className="text-muted-foreground text-sm">
            {t("notifications.webPushUnsupported")}
          </p>
        ) : null}
        {status === "unconfigured" ? (
          <p className="text-muted-foreground text-sm">
            {t("notifications.webPushUnconfigured")}
          </p>
        ) : null}
        {status === "denied" ? (
          <p className="text-muted-foreground text-sm">
            {t("notifications.webPushDenied")}
          </p>
        ) : null}
        {status === "error" ? (
          <p className="text-destructive text-sm">
            {t("notifications.webPushError")}
          </p>
        ) : null}
        {status === "prompt" || status === "busy" || status === "subscribed" ? (
          <div className="flex items-center justify-between gap-3">
            <p className="text-sm">
              {status === "subscribed"
                ? t("notifications.webPushOn")
                : t("notifications.webPushOff")}
            </p>
            {status === "subscribed" ? (
              <Button
                type="button"
                variant="outline"
                onClick={() => void unsubscribe()}
              >
                <BellOff className="size-4" />
                {t("notifications.webPushDisable")}
              </Button>
            ) : (
              <Button
                type="button"
                disabled={status === "busy"}
                onClick={() => void subscribe()}
              >
                <Bell className="size-4" />
                {t("notifications.webPushEnable")}
              </Button>
            )}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
