import { Card } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  APPEARANCE_ACTIONS_CLASS,
  APPEARANCE_CARD_CLASS,
  APPEARANCE_CONTROL_CLASS,
  APPEARANCE_SELECT_CLASS,
  APPEARANCE_SWITCH_CLASS,
  AppearanceRow,
} from "./appearance-row";

/** A theme row's control before the page is ready: the select's box over
 *  the ghost Edit's (w3/097). */
function ThemeChoicePreview() {
  return (
    <div className={APPEARANCE_CONTROL_CLASS}>
      <Skeleton className={APPEARANCE_SELECT_CLASS} />
      <div className={APPEARANCE_ACTIONS_CLASS}>
        <Skeleton className="h-8 w-17" />
      </div>
    </div>
  );
}

/**
 * The Appearance card while the settings page loads (w3/m528, w3/m531,
 * w3/097): the ready card's three rows, each with its own name (and High
 * Contrast Mode's description) beside the boxes its controls take, a theme
 * row's select over its Edit, which show the account's settings only once the
 * page is ready. Skeleton-only, so the route's pending component keeps it out
 * of the page's chunk.
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
        <ThemeChoicePreview />
      </AppearanceRow>
      <AppearanceRow label={t("auth.logExplorerTheme")}>
        <ThemeChoicePreview />
      </AppearanceRow>
      <AppearanceRow
        label={t("auth.highContrastMode")}
        description={t("auth.highContrastModeDescription")}
      >
        <div className={APPEARANCE_SWITCH_CLASS}>
          <Skeleton className="h-5 w-24" />
        </div>
      </AppearanceRow>
    </Card>
  );
}
