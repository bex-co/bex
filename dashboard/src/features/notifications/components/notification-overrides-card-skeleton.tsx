import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";

/** The card's frame while loading: one line, like its usual empty state. */
export function NotificationOverridesCardSkeleton() {
  return (
    <Card aria-hidden="true">
      <CardHeader className="gap-2">
        <Skeleton className="h-5 w-44" />
        <Skeleton className="h-4 w-80 max-w-full" />
      </CardHeader>
      <CardContent>
        <Skeleton className="h-5 w-72 max-w-full" />
      </CardContent>
    </Card>
  );
}
