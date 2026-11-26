import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";

/**
 * The pending frames of a sandbox group's pages and drawers (w1/m658), each
 * the ready page's structure at the same breakpoint: the toolbar, the
 * summary, a table at md and up and stacked rows below it, and the drawer's
 * header, tabs and rows. Skeleton-only, so the routes' pending components
 * stay out of the pages' lazy chunks (w3/066).
 */

function TablePreview({
  columns,
  rows = 5,
  region,
}: {
  columns: number;
  rows?: number;
  region: string;
}) {
  return (
    <div data-skeleton-region={region} aria-hidden="true">
      <div className="hidden overflow-hidden rounded-lg border md:block">
        <Table>
          <TableHeader>
            <TableRow>
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
                {Array.from({ length: columns }).map((_, c) => (
                  <TableCell key={c} className="py-3">
                    <Skeleton className={c === 0 ? "h-4 w-36" : "h-4 w-20"} />
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <div className="space-y-2 md:hidden">
        {Array.from({ length: Math.min(rows, 3) }).map((_, r) => (
          <div key={r} className="space-y-2 rounded-lg border p-3">
            <div className="flex items-center justify-between gap-2">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-5 w-16 rounded-md" />
            </div>
            <Skeleton className="h-3 w-44" />
            <Skeleton className="h-3 w-20" />
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
  return <TablePreview region="sandboxes-table" columns={7} />;
}

/** The Sandboxes page: toolbar, Total/Errored/Running, the chart, the list. */
export function SandboxesPageSkeleton() {
  return (
    <div className="space-y-6">
      <ToolbarSkeleton region="sandboxes-toolbar" actions />
      <div
        data-skeleton-region="run-summary"
        aria-hidden="true"
        className="space-y-3"
      >
        <div className="grid grid-cols-3 gap-2 sm:gap-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <Card key={i} className="gap-1 py-3">
              <CardHeader className="px-3 py-0 sm:px-4">
                <Skeleton className="h-3.5 w-14" />
                <Skeleton className="h-5 w-10" />
              </CardHeader>
            </Card>
          ))}
        </div>
        <div className="rounded-lg border p-3">
          <Skeleton className="h-[72px] w-full" />
          <Skeleton className="mt-2 h-3 w-40" />
        </div>
      </div>
      <SandboxesListSkeleton />
    </div>
  );
}

export function SnapshotsListSkeleton() {
  return <TablePreview region="snapshots-table" columns={6} />;
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
  return <TablePreview region="images-table" columns={5} />;
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
  return (
    <div className="space-y-6" aria-hidden="true">
      <Card data-skeleton-region="group-general" className="gap-3 py-4">
        <CardHeader className="px-4">
          <Skeleton className="h-5 w-20" />
        </CardHeader>
        <CardContent className="space-y-3 px-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="flex justify-between py-1">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-28" />
            </div>
          ))}
          <Skeleton className="h-8 w-full sm:w-52" />
        </CardContent>
      </Card>
      <Card data-skeleton-region="credentials-panel" className="gap-3 py-4">
        <CardHeader className="px-4">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-4 w-64 max-w-full" />
        </CardHeader>
        <CardContent className="space-y-2 px-4">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </CardContent>
      </Card>
      <Card data-skeleton-region="danger-zone" className="gap-3 py-4">
        <CardHeader className="px-4">
          <Skeleton className="h-5 w-44" />
        </CardHeader>
        <CardContent className="space-y-3 px-4">
          <Skeleton className="h-4 w-72 max-w-full" />
          <Skeleton className="h-8 w-40" />
        </CardContent>
      </Card>
    </div>
  );
}

/** A drawer's body while its resource loads: header, tabs, rows. */
export function DrawerBodySkeleton({ tabs = 0 }: { tabs?: number }) {
  return (
    <div aria-hidden="true" data-skeleton-region="drawer-body">
      <div className="space-y-2 border-b p-4 pr-12">
        <div className="flex items-center gap-2">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-5 w-16 rounded-md" />
        </div>
        <Skeleton className="h-4 w-56" />
      </div>
      {tabs > 0 ? (
        <div className="flex gap-1 border-b px-4 py-2">
          {Array.from({ length: tabs }).map((_, i) => (
            <Skeleton key={i} className="h-5 w-16" />
          ))}
        </div>
      ) : null}
      <div className="space-y-3 p-4">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="flex justify-between gap-4">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-4 w-32" />
          </div>
        ))}
      </div>
    </div>
  );
}

/** The sandbox drawer: its header, the Events, Snapshots and Details tabs. */
export function SandboxDrawerSkeleton() {
  return <DrawerBodySkeleton tabs={3} />;
}
