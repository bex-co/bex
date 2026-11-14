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
 * The Settings → Repository environments card while its list loads: the
 * static header, the empty-or-first repository row, and the Add action with
 * its help line, at the ready card's geometry (dashboard/AGENTS.md skeleton
 * rule). Further repository rows are data-dependent and omitted.
 */
export function AgentRepoEnvironmentsCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="agent-repo-environments-skeleton">
      <CardHeader>
        <CardTitle>{t("agentEnvironment.repoEnvs.title")}</CardTitle>
        <CardDescription>
          {t("agentEnvironment.repoEnvs.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* One repository row inside the list's 1px border (or the empty
            line, which keeps the same box). */}
        <Skeleton
          className="box-content h-14 border border-transparent"
          data-skeleton-region="repo-environment-row"
        />
        <div className="space-y-2 border-t pt-4">
          {/* The Add button: its border, icon, label and padding. The label
              is real text kept invisible, so the bar takes the button's width
              in every language. */}
          <div className="flex flex-wrap gap-2">
            <Skeleton
              className="h-9 border border-transparent px-3 text-sm font-medium"
              data-testid="repo-envs-add-skeleton"
            >
              <span className="invisible flex items-center gap-2">
                <span className="size-4" />
                {t("agentEnvironment.repoEnvs.add")}
              </span>
            </Skeleton>
          </div>
          {/* The help line is real text (it wraps on narrow screens): both the
              admin and read-only variants share one cell, so the reserved
              height is the taller one's. */}
          <div
            className="text-muted-foreground grid text-xs"
            data-testid="repo-envs-help-skeleton"
            aria-hidden
          >
            <p className="col-start-1 row-start-1 text-transparent">
              {t("agentEnvironment.repoEnvs.help")}
            </p>
            <p className="invisible col-start-1 row-start-1">
              {t("agentEnvironment.repoEnvs.readOnly")}
            </p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
