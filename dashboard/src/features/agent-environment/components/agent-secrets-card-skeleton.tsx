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
 * The Settings → Agent secrets card while its secrets load: the static header,
 * the empty-or-first secret row, the name/value row, the scope/Save row and
 * the help line, at the ready card's geometry (dashboard/AGENTS.md skeleton
 * rule). Further secret rows are data-dependent and omitted, as is the
 * repository field the ready card shows only once the repo scope is picked.
 */
export function AgentSecretsCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="agent-secrets-skeleton">
      <CardHeader>
        <CardTitle>{t("agentEnvironment.secrets.title")}</CardTitle>
        <CardDescription>
          {t("agentEnvironment.secrets.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Skeleton className="h-9 w-full" />
        <div className="space-y-2 border-t pt-4">
          <div className="flex max-w-2xl flex-wrap gap-2">
            <Skeleton className="h-9 w-48" />
            <Skeleton className="h-9 min-w-48 flex-1" />
          </div>
          {/* The scope picker (w-44) and the Save button, whose label and
              padding come to 4rem. */}
          <div className="flex max-w-2xl flex-wrap gap-2">
            <Skeleton
              className="h-9 w-44"
              data-testid="secrets-scope-skeleton"
            />
            <Skeleton className="h-9 w-16" />
          </div>
          {/* The help line is real text (it wraps on narrow screens): both the
              admin and read-only variants share one cell, so the reserved
              height is the taller one's. */}
          <div
            className="text-muted-foreground grid text-xs"
            data-testid="secrets-help-skeleton"
            aria-hidden
          >
            <p className="col-start-1 row-start-1 text-transparent">
              {t("agentEnvironment.secrets.help")}
            </p>
            <p className="invisible col-start-1 row-start-1">
              {t("agentEnvironment.secrets.readOnly")}
            </p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
