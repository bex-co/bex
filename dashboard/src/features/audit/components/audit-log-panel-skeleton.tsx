import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { PanelTableSkeleton } from "@/common/components/panel-states";

/** AuditLogPanel's frame while it loads: title, description, the export's
 *  two dates, button and hint, then the table. */
export function AuditLogPanelSkeleton() {
  return (
    <Card aria-hidden="true">
      <CardHeader className="gap-2">
        <Skeleton className="h-5 w-28" />
        <Skeleton className="h-4 w-96 max-w-full" />
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-2">
          <div className="flex flex-wrap items-end gap-3">
            {[0, 1].map((field) => (
              <div key={field} className="space-y-1">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-9 w-40" />
              </div>
            ))}
            <Skeleton className="h-9 w-36" />
          </div>
          <Skeleton className="h-4 w-80 max-w-full" />
        </div>
        <PanelTableSkeleton />
      </CardContent>
    </Card>
  );
}
