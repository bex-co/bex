import { Link } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { serviceBaseForType } from "@/features/services/lib/service-base";
import { useNotificationOverrides } from "@/features/notifications/hooks/use-notification-overrides";
import { NotificationOverridesCardSkeleton } from "@/features/notifications/components/notification-overrides-card-skeleton";
import { SERVICE_NOTIFICATION_LEVEL_LABEL_KEYS } from "@/features/notifications/lib/notification-levels";

const LEVEL_LABEL_KEYS: Record<string, string> =
  SERVICE_NOTIFICATION_LEVEL_LABEL_KEYS;

/**
 * Workspace Settings → Notifications → Notification Overrides (w3/m305):
 * Render lists every service whose notification setting overrides the
 * workspace default (render.com/docs/notifications), each linking to the
 * service's own setting.
 */
export function NotificationOverridesCard({ ownerId }: { ownerId: string }) {
  const { t } = useTranslations();
  const { overrides, loading, error } = useNotificationOverrides(ownerId);
  if (loading) return <NotificationOverridesCardSkeleton />;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("notifications.overridesTitle")}</CardTitle>
        <CardDescription>
          {t("notifications.overridesDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {error ? (
          <p className="text-sm text-destructive">
            {t("notifications.overridesError")}
          </p>
        ) : overrides.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("notifications.overridesEmpty")}
          </p>
        ) : (
          <ul className="divide-y rounded-md border">
            {overrides.map((service) => {
              const label = LEVEL_LABEL_KEYS[service.notificationsToSend];
              const settings =
                serviceBaseForType(service.type) === "/static" ? (
                  <Link
                    to="/static/$serviceId/settings"
                    params={{ serviceId: service.id }}
                    className="font-medium hover:underline"
                  >
                    {service.name}
                  </Link>
                ) : (
                  <Link
                    to="/services/$serviceId/settings"
                    params={{ serviceId: service.id }}
                    className="font-medium hover:underline"
                  >
                    {service.name}
                  </Link>
                );
              return (
                <li
                  key={service.id}
                  className="flex items-center justify-between gap-4 px-3 py-2 text-sm"
                >
                  {settings}
                  <span className="text-muted-foreground">
                    {label ? t(label) : service.notificationsToSend}
                  </span>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
