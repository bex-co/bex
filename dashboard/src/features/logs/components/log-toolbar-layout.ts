/*
 * The Logs tab's filter bar geometry, shared by the bar and its loading
 * skeleton (route-skeletons), so the two keep one shape at every width. A
 * module of its own, so the skeleton chunk does not pull in the bar and its
 * popover, sheet and selects.
 */

/** The filter bar's rows, and on a phone its dock under the top bar: the
 *  page's own background behind it, out to the column's edges. */
export const LOG_FILTER_BAR_CLASS =
  "flex flex-wrap items-center gap-3 max-sm:sticky max-sm:top-0 max-sm:z-20 max-sm:-mx-4 max-sm:grid max-sm:grid-cols-[minmax(0,1fr)_minmax(10.75rem,1fr)] max-sm:gap-2 max-sm:bg-background max-sm:px-4 max-sm:py-2";

/** Filters and Live: in the bar's flow on a wide screen, one cell of a
 *  phone's two-column grid, so its rows share their column edges. That
 *  column keeps the room both need, so the word Live never wraps. */
export const LOG_FILTER_TAIL_CLASS =
  "contents max-sm:flex max-sm:min-w-0 max-sm:items-center max-sm:gap-2";

/** The type and range selects: narrower until 1280, so search gets room. */
export const LOG_SELECT_WIDTH_CLASS =
  "w-32 xl:w-40 max-sm:w-auto max-sm:min-w-0";

/** The Live control's row: 32px, or a 44px touch target. */
export const LIVE_TOGGLE_CLASS =
  "inline-flex min-h-8 shrink-0 cursor-pointer items-center gap-2 text-sm whitespace-nowrap max-lg:min-h-11 pointer-coarse:min-h-11";
