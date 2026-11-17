import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * The Recovery panel's content while its first read is in flight, shaped like
 * what it resolves to (dashboard/AGENTS.md § Navigation pending states). A
 * database with backups gets the window's two bounds, the plan's retention
 * line (w7/m453), the Restore and Export actions, one backup row, the export
 * scope line when the panel names the database, and the export list's empty
 * line (exports are on demand and expire, so most databases have none). One
 * without backups gets the single line its disabled state shows. Each bar has
 * the height of the text it stands for, measured in English against the ready
 * panel at 1280px and 375px: below `sm` the field labels and the retention
 * line take two lines, a backup row grows to 50px and the export scope takes
 * three lines, so the swap does not move the page at either width.
 */
export function RecoveryContentSkeleton({
  backupsEnabled,
  exportScope,
}: {
  backupsEnabled: boolean;
  exportScope: boolean;
}) {
  if (!backupsEnabled) {
    return (
      <div data-testid="recovery-skeleton">
        <Skeleton className="h-5 w-2/3" />
      </div>
    );
  }
  return (
    <div data-testid="recovery-skeleton" className="space-y-4">
      <div className="grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2">
        {[0, 1].map((i) => (
          <div key={i} className="flex justify-between gap-4 border-b pb-2">
            <Skeleton className="h-10 w-28 sm:h-5" />
            <Skeleton className="h-5 w-36" />
          </div>
        ))}
      </div>
      <Skeleton
        data-testid="recovery-skeleton-retention"
        className="h-10 w-3/4 sm:h-5"
      />
      <div className="flex flex-wrap gap-2">
        <Skeleton className="h-9 w-52" />
        <Skeleton className="h-9 w-[8.75rem]" />
      </div>
      <SkeletonList rowClassName="h-[3.125rem] w-full sm:h-10" />
      {exportScope ? <Skeleton className="h-[3.75rem] w-5/6 sm:h-5" /> : null}
      <SkeletonList rowClassName="h-5 w-1/3" />
    </div>
  );
}

function SkeletonList({ rowClassName }: { rowClassName: string }) {
  return (
    <div className="space-y-2">
      <Skeleton className="h-5 w-24" />
      <Skeleton className={rowClassName} />
    </div>
  );
}

/**
 * The whole Recovery card while its code loads (the route's Suspense
 * fallback): the title and description, then the same content skeleton.
 */
export function RecoveryPanelSkeleton(props: {
  backupsEnabled: boolean;
  exportScope: boolean;
}) {
  return (
    <Card>
      <CardHeader>
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-5 w-full max-w-xl" />
      </CardHeader>
      <CardContent>
        <RecoveryContentSkeleton {...props} />
      </CardContent>
    </Card>
  );
}
