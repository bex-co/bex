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
 * The Settings → Agent secrets card while its names load: the static header, the
 * empty-or-first name row, and the name/value/Save row with its help line, at
 * the ready card's geometry (dashboard/AGENTS.md skeleton rule). Further name
 * rows are data-dependent and omitted.
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
            <Skeleton className="h-9 w-20" />
          </div>
          <Skeleton className="h-3 w-80 max-w-full" />
        </div>
      </CardContent>
    </Card>
  );
}
