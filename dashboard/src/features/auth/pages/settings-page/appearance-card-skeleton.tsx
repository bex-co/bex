import { Palette } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The Appearance card while the settings page loads (w3/m528): its own
 * heading and description over the Dashboard Theme select's box, which shows
 * the browser's theme only once the page is ready. Skeleton-only, so the
 * route's pending component keeps it out of the page's chunk.
 */
export function AppearanceCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card aria-hidden="true" data-testid="appearance-card-skeleton">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Palette className="size-4" />
          {t("auth.dashboardTheme")}
        </CardTitle>
        <CardDescription>{t("auth.dashboardThemeDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <Skeleton className="h-9 w-56" />
      </CardContent>
    </Card>
  );
}
