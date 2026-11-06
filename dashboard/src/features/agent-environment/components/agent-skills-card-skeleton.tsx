import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The Settings → Agent skills card while its list loads: the static header, the
 * empty-or-first skill row, and the New / Import actions with their help line,
 * at the ready card's geometry (dashboard/AGENTS.md skeleton rule). Further
 * skill rows are data-dependent and omitted.
 */
export function AgentSkillsCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="agent-skills-skeleton">
      <CardHeader>
        <CardTitle>{t("agentEnvironment.skills.title")}</CardTitle>
        <CardDescription>
          {t("agentEnvironment.skills.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Skeleton className="h-14 w-full" data-skeleton-region="skill-row" />
        <div className="space-y-2 border-t pt-4">
          <div className="flex flex-wrap gap-2">
            <Skeleton className="h-9 w-28" />
            <Skeleton className="h-9 w-36" />
          </div>
          {/* The help line is real text (it wraps on narrow screens): both the
              admin and read-only variants share one cell, so the reserved
              height is the taller one's. */}
          <div
            className="text-muted-foreground grid text-xs"
            data-testid="skills-help-skeleton"
            aria-hidden
          >
            <p className="col-start-1 row-start-1 text-transparent">
              {t("agentEnvironment.skills.help")}
            </p>
            <p className="invisible col-start-1 row-start-1">
              {t("agentEnvironment.skills.readOnly")}
            </p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
