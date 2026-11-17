import type { ReactNode } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
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
 * The deploy history's card: its title with the count line at the right, the
 * search and status filter, then the list. The page and its route skeleton
 * both draw it, so the two pending states share the ready geometry (w4/702).
 */
export function DeploysListCard({
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
    <Card className="@container/deploys">
      <CardHeader className="space-y-3">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <CardTitle>{title}</CardTitle>
          {count}
        </div>
        <div className="flex flex-col gap-2 sm:flex-row">{controls}</div>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
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
    <TableHeader>
      <TableRow>
        <TableHead>{t("deploys.columnDeploy")}</TableHead>
        <TableHead className="hidden whitespace-nowrap @3xl/deploys:table-cell">
          {t("deploys.columnTrigger")}
        </TableHead>
        <TableHead className="hidden whitespace-nowrap @3xl/deploys:table-cell">
          {t("deploys.columnDuration")}
        </TableHead>
        <TableHead className="w-0 whitespace-nowrap text-right">
          <span className="sr-only">{t("deploys.columnActions")}</span>
        </TableHead>
      </TableRow>
    </TableHeader>
  );
}

/**
 * One deploy row's lines at their ready heights. Every line of a ready row is
 * a single line (w4/702): the status badge and id (22 px), the commit (a
 * text-sm line), the time (a text-xs line), and, below the card's @3xl width,
 * Trigger · Duration folded under them. Bars use percentage widths, so they
 * never widen the column past what the ready rows need.
 */
function DeployRowPlaceholder({ commit }: { commit: boolean }) {
  return (
    <TableRow data-testid="deploy-row-placeholder">
      <TableCell className="min-w-0 align-top">
        <div className="flex items-center gap-2">
          <Skeleton className="h-[22px] w-16 shrink-0" />
          <Skeleton className="h-4 w-full max-w-40" />
        </div>
        {commit ? <Skeleton className="mt-1 h-5 w-full max-w-72" /> : null}
        <Skeleton className="mt-1 h-4 w-full max-w-32" />
        <Skeleton className="mt-2 h-4 w-full max-w-28 @3xl/deploys:hidden" />
      </TableCell>
      <TableCell className="hidden align-top @3xl/deploys:table-cell">
        <Skeleton className="h-5 w-20" />
      </TableCell>
      <TableCell className="hidden align-top @3xl/deploys:table-cell">
        <Skeleton className="h-5 w-12" />
      </TableCell>
      <TableCell className="w-0" />
    </TableRow>
  );
}

/**
 * The first page's frame while it loads (w4/702): the column headers, one
 * placeholder per deploy of a full first page, and the Load more slot. An
 * active service fills that page, so the swap moves nothing; a shorter history
 * shrinks once, with the header and row geometry already in place. `commits`
 * is false for an image-backed service, whose deploys carry no commit line.
 */
export function DeploysTableFrame({ commits = true }: { commits?: boolean }) {
  const { t } = useTranslations();
  return (
    <div aria-hidden="true" data-testid="deploys-table-frame">
      <Table>
        <DeploysTableHeader />
        <TableBody>
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

/** The whole card before the route's component mounts: the title, the count
 *  slot, the search and status filter's boxes, and the table frame. */
export function DeploysListCardSkeleton({
  titleKey = "deploys.listTitle",
  commits,
}: {
  titleKey?: "deploys.listTitle" | "deploys.buildsTitle";
  commits?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <DeploysListCard
      title={t(titleKey)}
      count={<DeploysCountSlot />}
      controls={
        <>
          {/* As the search field's wrapper: flex-1 alone would collapse the
              bar to 0 px in the phone-width column. */}
          <div className="min-w-0 flex-1">
            <Skeleton className="h-9" />
          </div>
          <Skeleton className="h-8 w-full sm:w-44" />
        </>
      }
    >
      <DeploysTableFrame commits={commits} />
    </DeploysListCard>
  );
}
