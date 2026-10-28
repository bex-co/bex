import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils";

/** One network-tier option's frame, shared by the ready card and this skeleton. */
export const TIER_OPTION_CLASS =
  "flex min-h-20 w-full flex-col gap-1 rounded-lg border p-4";

/**
 * The Settings → Agents card while its environment loads: header, the three
 * network-tier options, the registry-list disclosure, the default-model field
 * and the Save footer, at the ready card's geometry (dashboard/AGENTS.md
 * skeleton rule). The Custom hostname list is data-dependent and omitted.
 */
export function AgentEnvironmentCardSkeleton() {
  return (
    <Card data-testid="agent-environment-skeleton">
      <CardHeader>
        <Skeleton className="h-5 w-24" />
        <Skeleton className="h-4 w-96 max-w-full" />
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-3">
          <div className="space-y-1">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-4 w-80 max-w-full" />
          </div>
          <div className="grid grid-cols-1 gap-3">
            {[0, 1, 2].map((i) => (
              <div key={i} className={cn(TIER_OPTION_CLASS, "border-border")}>
                <Skeleton className="h-4 w-48" />
                <Skeleton className="h-4 w-full" />
              </div>
            ))}
          </div>
          <Skeleton className="h-4 w-56" />
        </div>
        <div className="space-y-2 border-t pt-6">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-9 w-full max-w-sm" />
          <Skeleton className="h-3 w-72 max-w-full" />
        </div>
      </CardContent>
      <CardFooter className="justify-end border-t">
        <Skeleton className="h-9 w-16" />
      </CardFooter>
    </Card>
  );
}
