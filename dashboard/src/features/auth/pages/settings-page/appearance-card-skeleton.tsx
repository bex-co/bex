import { Card } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  APPEARANCE_CARD_CLASS,
  APPEARANCE_SELECT_CLASS,
  AppearanceRow,
} from "./appearance-row";

/**
 * The Appearance card while the settings page loads (w3/m528, w3/m531): the
 * ready card's three rows, each with its own name (and High Contrast Mode's
 * description) over the box its control takes, which shows the account's
 * setting only once the page is ready. Skeleton-only, so the route's pending
 * component keeps it out of the page's chunk.
 */
export function AppearanceCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card
      aria-hidden="true"
      data-testid="appearance-card-skeleton"
      className={APPEARANCE_CARD_CLASS}
    >
      <AppearanceRow label={t("auth.dashboardTheme")}>
        <Skeleton className={`h-9 ${APPEARANCE_SELECT_CLASS}`} />
      </AppearanceRow>
      <AppearanceRow label={t("auth.logExplorerTheme")}>
        <Skeleton className={`h-9 ${APPEARANCE_SELECT_CLASS}`} />
      </AppearanceRow>
      <AppearanceRow
        label={t("auth.highContrastMode")}
        description={t("auth.highContrastModeDescription")}
      >
        <Skeleton className="h-5 w-24" />
      </AppearanceRow>
    </Card>
  );
}
