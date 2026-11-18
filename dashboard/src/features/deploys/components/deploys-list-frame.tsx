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

/**
 * The deploy history's frame: its title with the count line at the right, the
 * search and status filter, then the list. The page and its route skeleton
 * both draw it, so the two pending states share the ready geometry (w4/702).
 */
export function DeploysListFrame({
  title,
  count,
  controls,
  children,
}: {
  title: ReactNode;
  count: ReactNode;
  controls: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="@container/deploys space-y-4 max-sm:space-y-3">
      <div className="space-y-3 max-sm:space-y-2 @3xl/deploys:flex @3xl/deploys:items-center @3xl/deploys:justify-between @3xl/deploys:gap-6 @3xl/deploys:space-y-0">
        <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-2 @3xl/deploys:justify-start">
          <h2 className="text-base leading-6 font-semibold">{title}</h2>
          {count}
        </div>
        <div className="flex gap-2 @3xl/deploys:w-[32rem] @3xl/deploys:max-w-[60%]">
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

/** The whole list before the route's component mounts: the title, the count
 *  slot, the search and status filter's boxes, and the table frame. */
export function DeploysListFrameSkeleton({
  titleKey = "deploys.listTitle",
  commits,
}: {
  titleKey?: "deploys.listTitle" | "deploys.buildsTitle";
  commits?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <DeploysListFrame
      title={t(titleKey)}
      count={<DeploysCountSlot />}
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
