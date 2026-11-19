import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils";
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
// less settles upward into place rather than shoving the log down.
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
 * Skeleton-only, so the route's pending frame can import it.
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
