import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * The environment lists' geometry, read from the page's width rather than the
 * window's (`@container/page`): beside the sidebar a 768px tablet leaves the
 * cards about 460px, which is a phone's layout, not a table's.
 *
 * Narrow: each row is two lines, the name on its own line (a file name keeps
 * "service-account.json" whole) and the value under it with Reveal and copy
 * beside the value. From `@2xl` (672px of page) the list is
 * one table: a column-label row and every row share the parent's columns
 * through `subgrid`, so names, values and actions line up from row to row.
 * The row after the labels starts at the divider, as the first row does.
 */
export const ENV_LIST_CLASS =
  "divide-y [&>[data-env-head]+*]:pt-0 @2xl/page:grid @2xl/page:grid-cols-[minmax(0,2fr)_minmax(0,3fr)_auto] @2xl/page:gap-x-4 @2xl/page:[&>[data-env-head]+*]:pt-3";

/**
 * A card's header on the environment pages: the title over its description,
 * and the card's actions beside them once the page is wide enough
 * (`@2xl/page`, as the rows) or under them before that. The pending frames in
 * route-skeletons.tsx use the same classes.
 */
export const ENV_CARD_HEADER_CLASS =
  "grid-cols-1 grid-rows-none @2xl/page:grid-cols-[minmax(0,1fr)_auto] @2xl/page:grid-rows-[auto_auto]";
export const ENV_CARD_ACTION_CLASS =
  "col-start-1 row-start-2 mt-2 justify-self-stretch @2xl/page:col-start-2 @2xl/page:row-span-2 @2xl/page:row-start-1 @2xl/page:mt-0 @2xl/page:justify-self-end";
/** Card descriptions read at 16px on a phone, as body text should. */
export const ENV_CARD_DESCRIPTION_CLASS =
  "mt-1.5 max-w-prose text-pretty max-md:text-base";

/** A child of the list that does not use the columns: it spans them all. */
export const ENV_LIST_FULL_ROW_CLASS = "@2xl/page:col-span-full";

/** The column-label row; hidden while the list is narrow. */
export const ENV_LIST_HEAD_CLASS =
  "text-muted-foreground hidden pb-2 text-xs font-medium @2xl/page:col-span-full @2xl/page:grid @2xl/page:grid-cols-subgrid";

/**
 * An environment variable's or secret file's read row: name, value, then
 * Reveal and copy. Shared with the rows' loading frames so the two stay one
 * shape.
 */
export const ENV_ROW_CLASS =
  "grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 gap-y-2 py-4 first:pt-0 last:pb-0 @2xl/page:col-span-full @2xl/page:grid-cols-subgrid @2xl/page:gap-y-0 @2xl/page:py-3";
/** The row's name: a line of its own while narrow, the first column after. */
export const ENV_ROW_NAME_CLASS = "col-span-2 @2xl/page:col-span-1";
/**
 * The value, in a field-shaped box the width of its column: the same box the
 * draft's value input takes in Edit, so the switch keeps every edge.
 */
export const ENV_ROW_VALUE_CLASS =
  "bg-muted/50 flex min-h-9 min-w-0 items-center rounded-md border px-3 py-1.5";
export const ENV_ROW_ACTIONS_CLASS =
  "col-start-2 row-start-2 flex flex-wrap justify-end gap-1 @2xl/page:col-start-auto @2xl/page:row-start-auto";

/** The column labels over a list, for the read rows and their drafts alike. */
export function EnvListHead({ name, value }: { name: string; value: string }) {
  return (
    <div data-env-head="" className={ENV_LIST_HEAD_CLASS}>
      <span>{name}</span>
      <span>{value}</span>
    </div>
  );
}

/** Read rows while a list loads, at the controls' touch height below `lg`. */
export function EnvRowsSkeleton({ rows = 2 }: { rows?: number }) {
  return (
    <div className={ENV_LIST_CLASS} aria-hidden="true">
      <div data-env-head="" className={ENV_LIST_HEAD_CLASS}>
        <Skeleton className="h-4 w-10" />
        <Skeleton className="h-4 w-12" />
      </div>
      {Array.from({ length: rows }, (_, index) => (
        <div key={index} className={ENV_ROW_CLASS}>
          <Skeleton className={`h-5 w-32 ${ENV_ROW_NAME_CLASS}`} />
          <Skeleton className="h-9 w-full" />
          <div className={ENV_ROW_ACTIONS_CLASS}>
            <Skeleton className="h-8 w-20 max-lg:h-11 pointer-coarse:h-11" />
            <Skeleton className="size-8 max-lg:size-11 pointer-coarse:size-11" />
          </div>
        </div>
      ))}
    </div>
  );
}
