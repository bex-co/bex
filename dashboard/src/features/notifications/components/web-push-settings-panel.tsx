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
  const { status, failedAction, subscribe, unsubscribe } =
    useWebPushSubscription(vapidPublicKey ?? "", webPushAvailable ?? false);

  if (loading) {
    return null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("notifications.webPushTitle")}</CardTitle>
        <CardDescription>
          {t("notifications.webPushDescription")}
        </CardDescription>
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
        {status === "prompt" ||
        status === "busy" ||
        status === "subscribed" ||
        status === "error" ? (
          <div className="flex items-center justify-between gap-3">
            {status === "error" ? (
              // Panel-level failure (subscription setup), not a field
              // validation — role="alert" is enough (m101/t012 sibling
              // decision). The button below offers the failed action again.
              <p className="text-destructive text-sm" role="alert">
                {t("notifications.webPushError")}
              </p>
            ) : (
              <p className="text-sm">
                {status === "subscribed"
                  ? t("notifications.webPushOn")
                  : t("notifications.webPushOff")}
              </p>
            )}
            {status === "subscribed" || failedAction === "unsubscribe" ? (
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
