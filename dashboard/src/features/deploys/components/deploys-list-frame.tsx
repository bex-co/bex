import type { ReactNode } from "react";
import { cn } from "@/common/lib/utils/utils";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { DEPLOYS_PAGE_SIZE } from "../lib/deploy-list-search";

/** Manual Build's box, which the loading frame reserves. Kept here, not in
 *  the button's module, so the route skeletons stay free of its mutation. */
export const MANUAL_BUILD_BUTTON_CLASS = "shrink-0 max-lg:h-11";

/**
 * The deploy history's frame: the count line, the search and status filter,
 * then the list (the tab's name is the detail layout's heading). The page and
 * its route skeleton both draw it, so the two pending states share the ready
 * geometry (w4/702).
 *
 * `action` is a repo-backed cron job's Manual Build (w3/m555): beside the
 * search and filter once the list is wide, as on Render, and on the count's
 * line below that, so the search keeps its width on a phone.
 */
export function DeploysListFrame({
  count,
  controls,
  action,
  children,
}: {
  count: ReactNode;
  controls: ReactNode;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="@container/deploys space-y-4 max-sm:space-y-3">
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 gap-y-3 max-sm:gap-y-2 @3xl/deploys:flex @3xl/deploys:gap-6">
        <div className="min-w-0">{count}</div>
        {action ? (
          <div
            data-deploys-action=""
            className="col-start-2 row-start-1 flex @3xl/deploys:order-last @3xl/deploys:-ml-4"
          >
            {action}
          </div>
        ) : null}
        <div className="col-span-2 flex gap-2 @3xl/deploys:ml-auto @3xl/deploys:w-[32rem] @3xl/deploys:max-w-[60%]">
          {controls}
        </div>
      </div>
      <div>{children}</div>
    </section>
  );
}

/** The count line's slot until the first page arrives: the same text-xs line,
 *  so the title row keeps its height when the count appears. */
export function DeploysCountSlot() {
  const { t } = useTranslations();
  return (
    <p
      aria-hidden="true"
      className="relative text-xs text-muted-foreground"
      data-testid="deploys-count-slot"
    >
      <ReservedText>
        {t("deploys.listCountLoaded", { count: DEPLOYS_PAGE_SIZE })}
      </ReservedText>
    </p>
  );
}

/** The table's column headers, shared by the ready list and its frame. Trigger
 *  and Duration get their own columns once the card is wide enough. */
export function DeploysTableHeader() {
  const { t } = useTranslations();
  return (
    <TableHeader role="rowgroup" className="block">
      <TableRow
        role="row"
        className={cn(
          deployRowGrid,
          "hidden hover:bg-transparent @3xl/deploys:grid",
        )}
      >
        <TableHead
          role="columnheader"
          className="flex items-center text-xs font-medium text-muted-foreground"
        >
          {t("deploys.columnDeploy")}
        </TableHead>
        <TableHead
          role="columnheader"
          className="hidden items-center whitespace-nowrap text-xs font-medium text-muted-foreground @3xl/deploys:flex"
        >
          {t("deploys.columnTrigger")}
        </TableHead>
        <TableHead
          role="columnheader"
          className="hidden items-center whitespace-nowrap text-xs font-medium text-muted-foreground @3xl/deploys:flex"
        >
          {t("deploys.columnDuration")}
        </TableHead>
        <TableHead role="columnheader">
          <span className="sr-only">{t("deploys.columnActions")}</span>
        </TableHead>
      </TableRow>
    </TableHeader>
  );
}

/** The row grid belongs to both the history and its loading frame. */
export const deployRowGrid =
  "grid grid-cols-1 @3xl/deploys:grid-cols-[minmax(0,1fr)_9rem_6rem_7rem] @3xl/deploys:items-center @6xl/deploys:grid-cols-[minmax(0,1fr)_12rem_8rem_8rem]";

function DeployRowPlaceholder({ commit }: { commit: boolean }) {
  return (
    <TableRow
      role="row"
      data-testid="deploy-row-placeholder"
      className={cn(deployRowGrid, "relative")}
    >
      <TableCell
        role="cell"
        className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 px-2 py-3 whitespace-normal"
      >
        <Skeleton className="mt-px size-[18px] rounded-full" />
        <div
          className={cn(
            "flex min-h-5 flex-col justify-center gap-y-1.5",
            commit && "@max-md/deploys:min-h-10",
          )}
        >
          <Skeleton className="h-3.5 w-4/5 @3xl/deploys:w-2/5" />
          {commit ? (
            <Skeleton className="h-3.5 w-3/5 @md/deploys:hidden" />
          ) : null}
        </div>
        <div className="col-start-2 flex min-w-0 flex-col gap-y-1 @2xl/deploys:flex-row @2xl/deploys:gap-x-3">
          <Skeleton className="h-4 w-40 max-w-full" />
          <Skeleton className="mr-24 h-4 w-32 max-w-full @3xl/deploys:hidden" />
        </div>
      </TableCell>
      <TableCell role="cell" className="hidden px-2 @3xl/deploys:block">
        <Skeleton className="h-4 w-20" />
      </TableCell>
      <TableCell role="cell" className="hidden px-2 @3xl/deploys:block">
        <Skeleton className="h-4 w-14" />
      </TableCell>
      <TableCell
        role="cell"
        className="absolute right-2 bottom-3 p-0 @3xl/deploys:static @3xl/deploys:justify-self-end @3xl/deploys:px-2"
      >
        <Skeleton className="h-5 w-20" />
      </TableCell>
    </TableRow>
  );
}

/**
 * The first page's frame while it loads (w4/702): the column headers, one
 * placeholder per deploy of a full first page, and the Load more slot. An
 * active service fills that page, so the swap moves nothing; a shorter history
 * shrinks once, with the header and row geometry already in place. `commits`
 * is false for an image-backed service, whose rows reserve a single-line deploy ID.
 */
export function DeploysTableFrame({ commits = true }: { commits?: boolean }) {
  const { t } = useTranslations();
  return (
    <div aria-hidden="true" data-testid="deploys-table-frame">
      <Table role="table" className="block">
        <DeploysTableHeader />
        <TableBody role="rowgroup" className="block">
          {Array.from({ length: DEPLOYS_PAGE_SIZE }, (_, row) => (
            <DeployRowPlaceholder key={row} commit={commits} />
          ))}
        </TableBody>
      </Table>
      <div className="mt-4 flex justify-center">
        <ReservedButton variant="outline" size="sm">
          {t("deploys.listLoadMore")}
        </ReservedButton>
      </div>
    </div>
  );
}

/** The whole list before the route's component mounts: the count slot, the
 *  search and status filter's boxes, a repo-backed cron job's Manual Build,
 *  and the table frame. */
export function DeploysListFrameSkeleton({
  commits,
  manualBuild = false,
}: {
  commits?: boolean;
  manualBuild?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <DeploysListFrame
      count={<DeploysCountSlot />}
      action={
        manualBuild ? (
          <ReservedButton
            variant="outline"
            size="sm"
            className={MANUAL_BUILD_BUTTON_CLASS}
          >
            {t("deploys.manualBuild")}
          </ReservedButton>
        ) : undefined
      }
      controls={
        <>
          <Skeleton className="h-9 min-w-0 flex-1 max-lg:h-11 pointer-coarse:h-11" />
          <Skeleton className="h-8 w-32 shrink-0 max-lg:h-11 pointer-coarse:h-11 @2xl/deploys:w-44" />
        </>
      }
    >
      <DeploysTableFrame commits={commits} />
    </DeploysListFrame>
  );
}
