import type { ReactNode } from "react";
import { cn } from "@/common/lib/utils/utils";
import { Skeleton } from "@/common/components/ui/skeleton";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { CRON_RUNS_PAGE_SIZE } from "@/features/services/lib/cron-run-list";

/**
 * The cron Runs page's frame (w3/m556): the search above the list, then the
 * list's header and rows. The page and its route skeleton both draw it, so the
 * pending state has the ready geometry. Free of the runs read and the cancel
 * mutation, so the route skeletons can import it.
 *
 * As on Render, Trigger and Duration are columns once the list is wide
 * (Render's 50rem container); below that they fold into a line under the run.
 */
export function CronRunsFrame({
  search,
  children,
}: {
  search: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="@container/runs space-y-4 max-sm:space-y-3">
      <div className="relative">{search}</div>
      <div>{children}</div>
    </section>
  );
}

/** The search field's box, which the frame reserves. */
export const CRON_RUNS_SEARCH_CLASS = "h-9 max-lg:h-11 pointer-coarse:h-11";

/** One row's grid: the status icon, the run, and its action, with Trigger and
 *  Duration between them once the list is wide. Shared by the header, the
 *  rows and the frame's placeholders. */
export const cronRunRowGrid =
  "grid grid-cols-[auto_minmax(0,1fr)_auto] items-start gap-x-3 px-2 @3xl/runs:grid-cols-[auto_minmax(0,1fr)_10rem_6rem_7rem] @3xl/runs:items-center";

/** The list's header: RUN with the count, then TRIGGER and DURATION, which
 *  name columns only once the list is wide. */
export function CronRunsListHeader({ count }: { count: ReactNode }) {
  const { t } = useTranslations();
  return (
    <div
      className={cn(
        cronRunRowGrid,
        "items-center border-b py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground",
      )}
    >
      <div className="col-span-2 flex h-5 items-center gap-2">
        <span>{t("services.cronRunsColRun")}</span>
        {count}
      </div>
      <div aria-hidden="true" className="hidden @3xl/runs:block">
        {t("services.cronRunsColTrigger")}
      </div>
      <div aria-hidden="true" className="hidden @3xl/runs:block">
        {t("services.cronRunColDuration")}
      </div>
    </div>
  );
}

/** The count's badge box, kept by the frame while the count is unknown. */
export const CRON_RUNS_COUNT_CLASS =
  "inline-flex h-5 min-w-6 items-center justify-center rounded-full bg-muted px-1.5 text-xs tabular-nums text-foreground";

/** Classes the ready row and its placeholder share. */
export const CRON_RUN_ROW_CLASS = "relative py-3";
export const CRON_RUN_ICON_CELL = "flex h-5 items-center";
export const CRON_RUN_MAIN_CELL = "flex min-w-0 flex-col gap-y-1";
export const CRON_RUN_LINE = "flex min-h-4 items-center text-xs";

function CronRunRowPlaceholder() {
  return (
    <li
      data-testid="cron-run-row-placeholder"
      className={cn(cronRunRowGrid, CRON_RUN_ROW_CLASS)}
    >
      <div className={CRON_RUN_ICON_CELL}>
        <Skeleton className="size-[18px] rounded-full" />
      </div>
      <div className={CRON_RUN_MAIN_CELL}>
        <div className="flex h-5 items-center">
          <Skeleton className="h-3.5 w-28" />
        </div>
        <div className={CRON_RUN_LINE}>
          <Skeleton className="h-3 w-36" />
        </div>
        <div className={cn(CRON_RUN_LINE, "@3xl/runs:hidden")}>
          <Skeleton className="h-3 w-24" />
        </div>
      </div>
      <div className="hidden @3xl/runs:block">
        <Skeleton className="h-4 w-20" />
      </div>
      <div className="hidden @3xl/runs:block">
        <Skeleton className="h-4 w-12" />
      </div>
    </li>
  );
}

/**
 * The list while its first page loads: the header with the count's box, one
 * placeholder per run of a full page, and the Load more slot. A cron job that
 * has run 30 times fills that page, so the swap moves nothing; a shorter
 * history shrinks once, with the header and row geometry already in place
 * (the Deploys list's rule, w4/702).
 */
export function CronRunRowsFrame() {
  const { t } = useTranslations();
  return (
    <div aria-hidden="true" data-testid="cron-runs-rows-frame">
      <CronRunsListHeader
        count={<Skeleton className={cn(CRON_RUNS_COUNT_CLASS, "w-7")} />}
      />
      <ul className="divide-y">
        {Array.from({ length: CRON_RUNS_PAGE_SIZE }, (_, row) => (
          <CronRunRowPlaceholder key={row} />
        ))}
      </ul>
      <div className="mt-4 flex justify-center">
        <ReservedButton variant="outline" size="sm" className="max-lg:h-11">
          {t("services.cronRunsLoadMore")}
        </ReservedButton>
      </div>
    </div>
  );
}

/** The whole page before the route's component mounts: the search's box and
 *  the list's frame. */
export function CronRunsFrameSkeleton() {
  return (
    <CronRunsFrame
      search={<Skeleton className={cn("w-full", CRON_RUNS_SEARCH_CLASS)} />}
    >
      <CronRunRowsFrame />
    </CronRunsFrame>
  );
}
