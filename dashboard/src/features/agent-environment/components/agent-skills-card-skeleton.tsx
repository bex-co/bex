import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * The Settings → Agent skills card while its list loads: header, the
 * empty-or-first skill row, and the New / Import actions with their help line,
 * at the ready card's geometry (dashboard/AGENTS.md skeleton rule). Further
 * skill rows are data-dependent and omitted.
 */
export function AgentSkillsCardSkeleton() {
  return (
    <Card data-testid="agent-skills-skeleton">
      <CardHeader>
        <Skeleton className="h-5 w-28" />
        <Skeleton className="h-4 w-[28rem] max-w-full" />
      </CardHeader>
      <CardContent className="space-y-4">
        <Skeleton className="h-14 w-full" data-skeleton-region="skill-row" />
        <div className="space-y-2 border-t pt-4">
          <div className="flex flex-wrap gap-2">
            <Skeleton className="h-9 w-28" />
            <Skeleton className="h-9 w-36" />
          </div>
          <Skeleton className="h-3 w-80 max-w-full" />
        </div>
      </CardContent>
    </Card>
  );
}
