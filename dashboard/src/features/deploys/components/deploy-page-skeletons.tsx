import { Info, TriangleAlert } from "lucide-react";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import {
  DEPLOY_FACT_CLASS,
  DEPLOY_FACT_ICON_CLASS,
  DEPLOY_FACT_FULL_CLASS,
  DEPLOY_FACTS_GRID_CLASS,
  DEPLOY_HEADER_CLASS,
  DEPLOY_ID_ROW_CLASS,
  DEPLOY_LOG_TOOLBAR_CLASS,
  DEPLOY_LOG_TOOLBAR_END_CLASS,
  DEPLOY_TITLE_ROW_CLASS,
  TIMELINE_DOT_CLASS,
  TIMELINE_LIST_CLASS,
  TIMELINE_SECTION_CLASS,
  TIMELINE_STEP_CLASS,
} from "@/features/deploys/components/deploy-page-layout";

// Status, Duration, Deployed, Trigger, Source, Pre-deploy and the full-width
// Image at their values' usual widths. The skeleton reserves the fullest
// common header (a repo-built image with a pre-deploy step), so a deploy with
// less settles upward into place rather than shoving the log down. A service
// that scales out puts its Scaling fact (w1/m661) in the sixth place, at the
// same 36px; only one that also runs a pre-deploy step adds a row.
const FACTS = [
  { value: "w-12" },
  { value: "w-14" },
  { value: "w-40" },
  { value: "w-24" },
  { value: "w-16" },
  { value: "w-20" },
  { value: "w-72", full: true },
];

/**
 * The deploy page's header while its deploy loads: the title and id lines,
 * then the facts grid at the loaded grid's columns, tile size and line
 * heights, so the log viewer under it does not move when the deploy lands.
 * Skeleton-only, so the route's pending frame can import it. The Notices
 * between the header and the log (w1/m659) stand on the deploy's own facts,
 * so the route loader reads them before the page mounts
 * (deploy-page-prefetch.ts) and the page mounts with them drawn. The route's
 * frame shows only while a navigation's
 * reads outlast the router's hold, before anything says whether the deploy
 * has any Notices, so it reserves none, as on a plain live deploy and as
 * Render's page shows no Notices heading over an empty list.
 */
export function DeployHeaderSkeleton() {
  return (
    <div className={DEPLOY_HEADER_CLASS}>
      <div className={DEPLOY_TITLE_ROW_CLASS}>
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex h-7 items-center">
            <Skeleton className="h-5 w-full max-w-md" />
          </div>
          <div className={DEPLOY_ID_ROW_CLASS}>
            <Skeleton className="h-3 w-28" />
          </div>
        </div>
      </div>
      <div className={DEPLOY_FACTS_GRID_CLASS}>
        {FACTS.map((fact, i) => (
          <div
            key={i}
            className={cn(
              DEPLOY_FACT_CLASS,
              fact.full && DEPLOY_FACT_FULL_CLASS,
            )}
          >
            <Skeleton className={DEPLOY_FACT_ICON_CLASS} />
            <div className="flex h-4 items-center">
              <Skeleton className="h-2.5 w-14" />
            </div>
            <div className="flex h-5 items-center">
              <Skeleton className={`h-3.5 max-w-full ${fact.value}`} />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

/** The status timeline's steps while they load: three steps at the loaded
 *  steps' dot, line heights and spacing. */
export function DeployTimelineStepsSkeleton() {
  return (
    <div className={TIMELINE_LIST_CLASS}>
      {Array.from({ length: 3 }, (_, i) => (
        <div key={i} className={TIMELINE_STEP_CLASS}>
          <Skeleton className={TIMELINE_DOT_CLASS} />
          <div className="flex h-5 items-center">
            <Skeleton className="h-3.5 w-28" />
          </div>
          <div className="flex h-4 items-center">
            <Skeleton className="h-3 w-44" />
          </div>
        </div>
      ))}
    </div>
  );
}

// The log toolbar's controls grow to the 44px touch target wherever the
// touch-region rule does (below 64rem, or a coarse pointer).
const TOOLBAR_CONTROL = "h-9 max-lg:h-11 pointer-coarse:h-11";
const TOOLBAR_ICON_BUTTON = `w-9 max-lg:w-11 pointer-coarse:w-11 ${TOOLBAR_CONTROL}`;

/**
 * The deploy log's toolbar while the page loads: type menu, search taking
 * the spare width, then the options and maximize buttons.
 */
export function DeployLogToolbarSkeleton() {
  return (
    <div className={DEPLOY_LOG_TOOLBAR_CLASS}>
      <Skeleton className={`w-24 ${TOOLBAR_CONTROL}`} />
      <Skeleton className={`min-w-0 flex-1 ${TOOLBAR_CONTROL}`} />
      <div className={DEPLOY_LOG_TOOLBAR_END_CLASS}>
        <Skeleton className={TOOLBAR_ICON_BUTTON} />
        <Skeleton className={TOOLBAR_ICON_BUTTON} />
      </div>
    </div>
  );
}

/** The status timeline while it loads: its title, then the steps. */
export function DeployTimelineCardSkeleton() {
  return (
    <div className={TIMELINE_SECTION_CLASS}>
      <div className="flex h-5 items-center">
        <Skeleton className="h-3.5 w-32" />
      </div>
      <DeployTimelineStepsSkeleton />
    </div>
  );
}

/** The back link over the deploy's title, while the page loads. */
export function DeployBackLinkSkeleton() {
  return (
    <div className="flex h-5 items-center">
      <Skeleton className="h-3 w-16" />
    </div>
  );
}

// The rollback page's buttons grow to the 44px touch target wherever the
// touch-region rule does (below 64rem, or a coarse pointer).
const ROLLBACK_BUTTON = "max-lg:h-11 pointer-coarse:h-11";

/**
 * The Confirm Rollback section while the page loads (w1/m646): the block's
 * title and lead over the comparison's 9.25rem frame, then the notices, the
 * two buttons and the gate status line, at the ready section's spacing. The
 * title, lead, notices and buttons are the ready page's own copy laid out
 * invisibly, so the lead wraps onto a second line on a phone exactly where
 * the ready one does. The notices are the ones the ready section will show:
 * a static site's always (its redirects, rewrites and headers stay), and the
 * Auto-Deploy warning when the cached service has auto-deploy on, so the
 * buttons land where the ready ones do (w1/m660). The comparison's rows are
 * the deploy's own, so the frame reserves the empty comparison's height.
 */
export function DeployRollbackSectionSkeleton({
  staticSite = false,
  autoDeployOn = false,
}: {
  staticSite?: boolean;
  autoDeployOn?: boolean;
} = {}) {
  const { t } = useTranslations();
  const [noticeBefore, noticeAfter] = aroundPlaceholder(
    t("deploys.rollbackStaticNotice", { what: PLACEHOLDER_SENTINEL }),
  );
  return (
    <div className="space-y-4">
      <div className="rounded-md border bg-card">
        <div className="space-y-1 border-b px-5 py-4">
          <p className="relative text-base font-semibold">
            <ReservedText>{t("deploys.rollbackTitle")}</ReservedText>
          </p>
          <p className="relative text-sm">
            <ReservedText>
              {t("deploys.rollbackLead")}{" "}
              <span className="font-medium">
                {t("deploys.rollbackLearnMore")}
              </span>
            </ReservedText>
          </p>
        </div>
        <div className="min-h-[9.25rem]" />
      </div>
      {staticSite || autoDeployOn ? (
        <div className="space-y-2" data-skeleton-region="rollback-notices">
          {staticSite ? (
            <Alert role="none">
              <Info className="invisible" />
              <AlertDescription>
                <p className="relative">
                  <ReservedText>
                    {noticeBefore}
                    <span className="font-semibold">
                      {t("deploys.rollbackStaticNoticeWhat")}
                    </span>
                    {noticeAfter}
                  </ReservedText>
                </p>
              </AlertDescription>
            </Alert>
          ) : null}
          {autoDeployOn ? (
            <Alert role="none">
              <TriangleAlert className="invisible" />
              <AlertTitle className="relative">
                <ReservedText>
                  {t("deploys.rollbackAutoDeployTitle")}
                </ReservedText>
              </AlertTitle>
              <AlertDescription>
                <p className="relative">
                  <ReservedText>
                    {t("deploys.rollbackAutoDeployBody")}
                  </ReservedText>
                </p>
              </AlertDescription>
            </Alert>
          ) : null}
        </div>
      ) : null}
      <div className="flex flex-wrap items-center gap-2">
        <ReservedButton variant="default" className={ROLLBACK_BUTTON}>
          {t("deploys.rollbackConfirm")}
        </ReservedButton>
        <ReservedButton variant="outline" className={ROLLBACK_BUTTON}>
          {t("deploys.rollbackCancel")}
        </ReservedButton>
      </div>
      <div className="min-h-5" />
    </div>
  );
}
