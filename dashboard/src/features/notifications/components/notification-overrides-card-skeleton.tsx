import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { ReservedText } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The card's frame while loading: its static title and description, which
 * wrap as the ready card's do, over the one line of its usual empty state.
 */
export function NotificationOverridesCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card aria-hidden="true" data-testid="notification-overrides-skeleton">
      <CardHeader>
        <CardTitle>{t("notifications.overridesTitle")}</CardTitle>
        <CardDescription>
          {t("notifications.overridesDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <p className="relative text-sm">
          <ReservedText>{t("notifications.overridesEmpty")}</ReservedText>
        </p>
      </CardContent>
    </Card>
  );
}
