import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton, SkeletonSpan } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { SandboxGroupCredentialsPanelSkeleton } from "@/features/sandbox-groups/components/sandbox-group-skeletons";
import { DetailRowsFrame } from "./drawer-frames";

/**
 * The pending frames of a sandbox group's pages (w1/m658), each the ready
 * page's structure at the same breakpoint: the toolbar, the summary, a table
 * at md and up and stacked rows below it. Rows reserve the ready rows' line
 * boxes (a name over its id, a status badge), and static copy is laid out
 * invisibly under a bar, so nothing moves when the data lands (t009 measured
 * the first frames 14 to 30 px short on production). Skeleton-only, so the
 * routes' pending components stay out of the pages' lazy chunks (w3/066).
 */

function TablePreview({
  region,
  columns,
  checkbox = false,
  twoLine,
  rows = 5,
}: {
  region: string;
  /** The ready table's columns after its checkbox column. */
  columns: number;
  /** A leading select column, as the Sandboxes table has. */
  checkbox?: boolean;
  /** The first column shows a name over its id, as Sandboxes and Snapshots do. */
  twoLine: boolean;
  rows?: number;
}) {
  return (
    <div data-skeleton-region={region} aria-hidden="true">
      <div className="hidden overflow-hidden rounded-lg border md:block">
        <Table>
          <TableHeader>
            <TableRow>
              {checkbox ? (
                <TableHead className="w-10">
                  <Skeleton className="size-4 rounded-sm" />
                </TableHead>
              ) : null}
              {Array.from({ length: columns }).map((_, i) => (
                <TableHead key={i}>
                  <Skeleton className="h-3.5 w-16" />
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {Array.from({ length: rows }).map((_, r) => (
              <TableRow key={r}>
                {checkbox ? (
                  <TableCell>
                    <Skeleton className="size-4 rounded-sm" />
                  </TableCell>
                ) : null}
                {Array.from({ length: columns }).map((_, c) => (
                  <TableCell key={c}>
                    {c === 0 ? (
                      <>
                        <div className="flex h-5 items-center">
                          <Skeleton className="h-3.5 w-36" />
                        </div>
                        {twoLine ? (
                          <div className="flex h-4 items-center">
                            <Skeleton className="h-3 w-44" />
                          </div>
                        ) : null}
                      </>
                    ) : c === 1 ? (
                      // The status badge's 22 px box.
                      <Skeleton className="h-[22px] w-16 rounded-md" />
                    ) : (
                      <div className="flex h-5 items-center">
                        <Skeleton className="h-3.5 w-20" />
                      </div>
                    )}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <div className="space-y-2 md:hidden">
        {Array.from({ length: Math.min(rows, 3) }).map((_, r) => (
          <div key={r} className="flex items-start gap-3 rounded-lg border p-3">
            {checkbox ? (
              <Skeleton className="mt-0.5 size-4 shrink-0 rounded-sm" />
            ) : null}
            <div className="min-w-0 flex-1 space-y-1">
              <div className="flex h-[22px] items-center justify-between gap-2">
                <Skeleton className="h-3.5 w-32" />
                <Skeleton className="h-[22px] w-16 rounded-md" />
              </div>
              <div className="flex h-4 items-center">
                <Skeleton className="h-3 w-44" />
              </div>
              {twoLine ? (
                <div className="flex h-4 items-center">
                  <Skeleton className="h-3 w-20" />
                </div>
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function ToolbarSkeleton({
  region,
  range = true,
  actions = false,
}: {
  region: string;
  range?: boolean;
  actions?: boolean;
}) {
  return (
    <div
      data-skeleton-region={region}
      aria-hidden="true"
      className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center"
    >
      <Skeleton className="h-8 w-full rounded-md sm:max-w-xs" />
      {range ? <Skeleton className="h-8 w-full rounded-md sm:w-40" /> : null}
      {actions ? (
        <div className="flex items-center gap-2 sm:ml-auto">
          <Skeleton className="h-8 w-40 rounded-md" />
          <Skeleton className="size-8 rounded-md" />
        </div>
      ) : null}
    </div>
  );
}

export function SandboxesListSkeleton() {
  return <TablePreview region="sandboxes-table" checkbox columns={6} twoLine />;
}

/** RunSummary's cards and chart, its labels laid out invisibly. */
function RunSummaryFrame() {
  const { t } = useTranslations();
  const cards = [
    t("sandboxGroupPages.summaryTotal"),
    t("sandboxGroupPages.summaryErrored"),
    t("sandboxGroupPages.summaryRunning"),
  ];
  const legend = [
    t("sandboxGroupPages.runsSucceeded"),
    t("sandboxGroupPages.runsErrored"),
  ];
  return (
    <div
      data-skeleton-region="run-summary"
      aria-hidden="true"
      className="space-y-3"
    >
      <div className="grid grid-cols-3 gap-2 sm:gap-4">
        {cards.map((label) => (
          <Card key={label} className="gap-1 py-3">
            <CardHeader className="px-3 py-0 sm:px-4">
              <CardDescription className="truncate">
                <span className="relative">
                  <ReservedText>{label}</ReservedText>
                </span>
              </CardDescription>
              <CardTitle
                as="div"
                className="text-base font-semibold tabular-nums"
              >
                <span className="relative">
                  <ReservedText>—</ReservedText>
                </span>
              </CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>
      <div className="rounded-lg border p-3">
        <Skeleton className="h-[72px] w-full" />
        <div className="text-muted-foreground mt-2 flex gap-4 text-xs">
          {legend.map((label) => (
            <span
              key={label}
              className="relative inline-flex items-center gap-1"
            >
              <span className="inline-block size-2 rounded-sm" />
              <span className="invisible">{label}</span>
              <SkeletonSpan />
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

/** The Sandboxes page: toolbar, Total/Errored/Running, the chart, the list. */
export function SandboxesPageSkeleton() {
  return (
    <div className="space-y-6">
      <ToolbarSkeleton region="sandboxes-toolbar" actions />
      <RunSummaryFrame />
      <SandboxesListSkeleton />
    </div>
  );
}

export function SnapshotsListSkeleton() {
  return <TablePreview region="snapshots-table" columns={6} twoLine />;
}

export function SnapshotsPageSkeleton() {
  return (
    <div className="space-y-6">
      <ToolbarSkeleton region="snapshots-toolbar" />
      <SnapshotsListSkeleton />
    </div>
  );
}

export function ImagesListSkeleton() {
  return <TablePreview region="images-table" columns={5} twoLine={false} />;
}

export function ImagesPageSkeleton() {
  return (
    <div className="space-y-6">
      <ToolbarSkeleton region="images-toolbar" range={false} />
      <ImagesListSkeleton />
    </div>
  );
}

/** Settings: General, the brokered credentials and Delete sandbox group. */
export function SandboxGroupSettingsSkeleton() {
  const { t } = useTranslations();
  return (
    <div className="space-y-6" aria-hidden="true">
      <Card data-skeleton-region="group-general" className="gap-3 py-4">
        <CardHeader className="px-4">
          <CardTitle as="div" className="relative w-fit text-base">
            <ReservedText>{t("sandboxGroupPages.general")}</ReservedText>
          </CardTitle>
        </CardHeader>
        <CardContent className="px-4">
          <DetailRowsFrame
            rows={[
              { label: "sandboxGroups.fieldName" },
              { label: "sandboxGroups.fieldRegion" },
              // Its value carries Make default, a 32 px button.
              { label: "sandboxGroups.fieldDefault", tall: true },
            ]}
          />
          <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-end">
            <div className="flex flex-1 flex-col gap-1 text-sm">
              <span className="relative w-fit">
                <ReservedText>
                  {t("sandboxGroups.fieldConcurrencyLimit")}
                </ReservedText>
              </span>
              <Skeleton className="h-8 w-full rounded-md sm:max-w-40" />
            </div>
            <ReservedButton variant="default" size="sm">
              {t("sandboxGroupPages.save")}
            </ReservedButton>
          </div>
        </CardContent>
      </Card>
      <SandboxGroupCredentialsPanelSkeleton />
      <Card
        data-skeleton-region="danger-zone"
        className="border-destructive/40 gap-3 py-4"
      >
        <CardHeader className="px-4">
          <CardTitle as="div" className="relative w-fit text-base">
            <ReservedText>{t("sandboxGroupPages.deleteGroup")}</ReservedText>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 px-4">
          <p className="text-muted-foreground relative text-sm">
            <ReservedText>
              {t("sandboxGroupPages.deleteGroupHelp")}
            </ReservedText>
          </p>
          <ReservedButton variant="destructive" size="sm">
            {t("sandboxGroupPages.deleteGroup")}
          </ReservedButton>
        </CardContent>
      </Card>
    </div>
  );
}
