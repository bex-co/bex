import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * The waiting plan's card while its read is in flight: the panel's own
 * loading state, and the Syncs page's pending frame for a Blueprint the cache
 * knows is created (w3/m677). Skeleton-only, so the pending frame can draw it
 * without loading the panel.
 */
export function PlannedSyncPanelSkeleton() {
  return (
    <Card aria-hidden="true" data-skeleton-region="planned-sync">
      <CardHeader>
        <Skeleton className="h-5 w-40" />
      </CardHeader>
      <CardContent className="space-y-4">
        <Skeleton className="h-4 w-full max-w-md" />
        <div className="space-y-2">
          <Skeleton className="h-5 w-64" />
          <Skeleton className="h-5 w-56" />
        </div>
        <div className="flex gap-2">
          <Skeleton className="h-8 w-24" />
          <Skeleton className="h-8 w-28" />
        </div>
      </CardContent>
    </Card>
  );
}
