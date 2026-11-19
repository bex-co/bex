// The Events feed row's geometry, shared by the loaded rows and their
// skeleton so the two cannot drift apart.
//
// On a phone or tablet a row is icon | content | chevron, its time on the
// line under the title. Once the page column is wide enough (a 1024px
// screen) the time takes a column of its own at the row's start, beside the
// event it dates — not at the far edge of a wide row, a screen's width from
// what it belongs to — and the row's actions move up beside the content.

export const EVENT_ROW_CLASS =
  "relative grid grid-cols-[2.25rem_minmax(0,1fr)_auto] gap-x-3 px-5 py-4 sm:px-6 @max-md/page:grid-cols-[1.75rem_minmax(0,1fr)_auto] @max-md/page:gap-x-2.5 @max-md/page:px-3 @2xl/page:grid-cols-[10rem_2.25rem_minmax(0,1fr)_auto_auto] @2xl/page:gap-x-4";

/** The icon disc: 36px, or 28px on a phone, whose text needs the width. */
export const EVENT_ICON_CELL =
  "col-start-1 row-span-4 row-start-1 size-9 rounded-full @max-md/page:size-7 @2xl/page:col-start-2 @2xl/page:row-span-2";

/** The title and its status badges, on a 24px line. */
export const EVENT_TITLE_CELL =
  "col-start-2 row-start-1 flex min-h-6 min-w-0 flex-wrap items-center gap-x-2 gap-y-1 @2xl/page:col-start-3";

/**
 * When it happened: one line under the title ("Nov 17, 11:53 PM · 23 hours
 * ago"), or the row's first column in two set lines — the date, then the
 * clock reading and how long ago — so every row's time reads the same way.
 */
export const EVENT_TIME_CELL =
  "col-start-2 row-start-2 text-xs leading-5 text-muted-foreground tabular-nums @2xl/page:col-start-1 @2xl/page:row-span-2 @2xl/page:row-start-1 @2xl/page:pt-0.5";

/** The detail lines: who and how, then what changed. */
export const EVENT_BODY_CELL =
  "col-start-2 row-start-3 mt-1 space-y-0.5 text-xs leading-5 text-muted-foreground @2xl/page:col-start-3 @2xl/page:row-start-2";

/** A deploy row's Cancel or Rollback: under the details on a phone, beside
 *  them from the wide layout up. */
export const EVENT_ACTIONS_CELL =
  "col-start-2 row-start-4 mt-3 @2xl/page:col-start-4 @2xl/page:row-span-2 @2xl/page:row-start-1 @2xl/page:mt-0 @2xl/page:self-center";

/** The chevron that marks a row opening its deploy. */
export const EVENT_CHEVRON_CELL =
  "col-start-3 row-span-4 row-start-1 self-center @2xl/page:col-start-5 @2xl/page:row-span-2";

/** The feed's toolbar: what is shown on the left, the filter on the right. */
export const EVENT_TOOLBAR_CLASS =
  "flex min-h-14 items-center justify-between gap-3 border-b px-5 py-3 sm:px-6 @max-md/page:px-3";

/** The feed's last line: Load older, or the start of the history. */
export const EVENT_FOOTER_CLASS =
  "flex min-h-16 items-center justify-center border-t px-5 py-4 sm:px-6";
