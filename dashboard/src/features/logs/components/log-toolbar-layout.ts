/*
 * The Logs tab's filter bar geometry, shared by the bar and its loading
 * skeleton (route-skeletons), so the two keep one shape at every width. A
 * module of its own, so the skeleton chunk does not pull in the bar and its
 * popover, sheet and selects.
 */

/** The filter bar's rows, and on a phone its dock under the top bar: the
 *  page's own background behind it, out to the column's edges. A phone's
 *  grid is [type | search | Options] over [range | Filters], or
 *  [search | Options] for a service with no type dropdown (w3/122). */
export const LOG_FILTER_BAR_CLASS =
  "flex flex-wrap items-center gap-3 max-sm:sticky max-sm:top-0 max-sm:z-20 max-sm:-mx-4 max-sm:grid max-sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] max-sm:gap-2 max-sm:bg-background max-sm:px-4 max-sm:py-2";

/** The search box of a bar with no type dropdown (w3/122): on a phone it
 *  spans the type's column too, so its row still ends at Options. */
export const LOG_SEARCH_ALONE_CLASS = "max-sm:col-span-2";

/** The Filters button: in the bar's flow on a wide screen, the last two
 *  columns of a phone's second row, so its rows share their column edges.
 *  `w-auto` lets the pending frame's fixed-width slot stretch there too. */
export const LOG_FILTERS_BUTTON_CLASS =
  "gap-1.5 max-sm:col-span-2 max-sm:w-auto max-sm:min-w-0";

/** An explorer's Maximize and Options buttons (w3/m509), side by side. */
export const LOG_ACTIONS_CLASS = "flex shrink-0 items-center gap-2";

/** The service Logs tab's Maximize and Options: at the bar's end on a wide
 *  screen; a phone keeps Options at its first row's end, Maximize inside
 *  it. */
export const LOG_FILTER_ACTIONS_CLASS = `${LOG_ACTIONS_CLASS} max-sm:col-start-3 max-sm:row-start-1`;

/** A toolbar icon button's box: 32px, or the 44px touch target. */
export const LOG_ICON_BUTTON_CLASS =
  "size-8 max-lg:size-11 pointer-coarse:size-11";

/** The type and range selects: narrower until 1280, so search gets room. */
export const LOG_SELECT_WIDTH_CLASS =
  "w-32 xl:w-40 max-sm:w-auto max-sm:min-w-0";

/**
 * A log explorer maximized (Render's Maximize, `M`, w3/m509): a full-screen
 * overlay, its last child (the log) filling everything under the controls.
 * `m-0`: the page's stack spacing would otherwise give the overlay a bottom
 * margin, stopping it short of the screen's bottom edge. `rounded-none
 * shadow-none` undo a themed frame's corners and bleed (below).
 */
export const LOG_MAXIMIZED_CLASS =
  "fixed inset-0 z-50 m-0 flex flex-col rounded-none bg-background p-4 shadow-none sm:p-6 [&>*:last-child]:min-h-0 [&>*:last-child]:flex-1";

/**
 * An explorer under a Log Explorer Theme of its own (w3/m531): its frame
 * paints the theme's page colour 12px past its edges with a spread shadow,
 * so the controls and the log sit on a panel of that theme while the frame
 * keeps the geometry an unthemed one has (no padding, so the log still ends
 * at the fold and nothing around it moves).
 */
export const LOG_THEMED_FRAME_CLASS =
  "rounded-md shadow-[0_0_0_0.75rem_var(--background)]";

/** A maximized explorer's body, around its lines: a column whose list slot
 *  (`LOG_MAXIMIZED_LIST_CLASS`) takes the height the notices leave. The slot
 *  is always rendered, so maximizing never remounts the list. */
export const LOG_MAXIMIZED_BODY_CLASS = "flex flex-col";
export const LOG_MAXIMIZED_LIST_CLASS = "min-h-0 flex-1";

/** A datastore log's Maximize and Options: at its toolbar's end, and on a
 *  phone at the end of the range's row (w3/m509). */
export const DATASTORE_LOG_ACTIONS_CLASS = `${LOG_ACTIONS_CLASS} max-sm:order-first max-sm:ml-auto`;
