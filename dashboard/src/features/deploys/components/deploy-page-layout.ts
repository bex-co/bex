// The deploy page's geometry — header, facts, log toolbar, timeline and the
// body around them — shared by the loaded page and its skeletons so the two
// cannot drift apart.

/** The back link to the deploys list, close over the header it leads. */
export const DEPLOY_PAGE_TOP_CLASS = "space-y-2";

/** Title row, then the facts grid; the reasons are Notices (w1/m659). */
export const DEPLOY_HEADER_CLASS = "space-y-5";

/** The title block on the left, the deploy's actions on the right. */
export const DEPLOY_TITLE_ROW_CLASS =
  "flex flex-wrap items-start justify-between gap-x-4 gap-y-3";

/** The deploy id under the title, at its copy button's height: 32px, or the
 *  44px touch target wherever the touch-region rule grows the button. */
export const DEPLOY_ID_ROW_CLASS =
  "flex h-8 items-center gap-2 font-mono text-xs text-muted-foreground max-lg:h-11 pointer-coarse:h-11";

/**
 * Render's deploy facts: two columns up to a tablet, three once the page
 * column has room for three icon tiles side by side (a 1024px screen).
 */
export const DEPLOY_FACTS_GRID_CLASS =
  "grid grid-cols-2 gap-x-4 gap-y-3 @md/page:gap-y-4 @2xl/page:grid-cols-3 @2xl/page:gap-x-6";

/** One fact: label over value, beside its icon tile from tablet width up. */
export const DEPLOY_FACT_CLASS =
  "relative min-w-0 @md/page:min-h-10 @md/page:pl-[3.25rem]";

/** The image reference's fact spans the grid, so the reference fits whole
 *  rather than being cut to its first letters in a column. */
export const DEPLOY_FACT_FULL_CLASS = "col-span-full";

/** The status timeline: a vertical list where it is narrow (a phone, or the
 *  column beside the log), one row of steps once it is wide enough to spread
 *  them across. Sized by its own frame, not the page. */
export const TIMELINE_LIST_CLASS = "@xl/timeline:flex";

/**
 * One timeline step. Each step but the last draws the connector to the next
 * dot — down on a phone, across from the wide layout up — so the line starts
 * and ends at dots rather than running past them. Across, the last step takes
 * only its own width at the row's end, so the line spans the whole row.
 */
export const TIMELINE_STEP_CLASS =
  "relative pb-5 pl-6 last:pb-0 not-last:before:absolute not-last:before:top-4 not-last:before:bottom-0 not-last:before:left-[5.5px] not-last:before:w-px not-last:before:bg-muted-foreground/40 not-last:before:content-[''] @xl/timeline:flex-1 @xl/timeline:pt-6 @xl/timeline:pr-4 @xl/timeline:pb-0 @xl/timeline:pl-0 @xl/timeline:last:flex-none @xl/timeline:last:pr-0 @xl/timeline:not-last:before:top-[5.5px] @xl/timeline:not-last:before:right-1 @xl/timeline:not-last:before:bottom-auto @xl/timeline:not-last:before:left-4 @xl/timeline:not-last:before:h-px @xl/timeline:not-last:before:w-auto";

/** A step's 12px dot, at the start of its connector. */
export const TIMELINE_DOT_CLASS =
  "absolute top-1 left-0 size-3 rounded-full @xl/timeline:top-0";

/** The 40px icon tile at the fact's top left; a phone drops it for width. */
export const DEPLOY_FACT_ICON_CLASS =
  "absolute top-0 left-0 hidden size-10 rounded-md @md/page:flex";

/** The status timeline's frame: the log panel's border and radius rather than
 *  a raised card, so the page's two sections read as one system. */
export const TIMELINE_SECTION_CLASS =
  "@container/timeline space-y-4 rounded-md border bg-card p-5";

/**
 * The log and the timeline: stacked, the log first, until the page column is
 * wide enough (a 1280px screen) to set the timeline beside the log, where it
 * stays in view instead of waiting under a screen-tall log.
 */
export const DEPLOY_BODY_CLASS =
  "space-y-6 @4xl/page:grid @4xl/page:grid-cols-[minmax(0,1fr)_17rem] @4xl/page:items-start @4xl/page:gap-6 @4xl/page:space-y-0";

/** The log toolbar: one row at every width; the narrowest phones close the
 *  gaps up so search keeps room for its placeholder. */
export const DEPLOY_LOG_TOOLBAR_CLASS =
  "flex items-center gap-2 max-[24rem]:gap-1.5";

/** The toolbar's trailing options and maximize buttons. */
export const DEPLOY_LOG_TOOLBAR_END_CLASS =
  "ml-auto flex shrink-0 items-center gap-1";
