import { Skeleton } from "@/common/components/ui/skeleton";
import { EmptyState } from "@/common/components/empty-state";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeploy } from "../hooks/use-deploy";
import { DeployHeader } from "./deploy-header";
import { DeployLogPanel } from "./deploy-log-panel";

export interface DeployDetailPageProps {
  serviceId: string;
  deployId: string;
}

/**
 * The per-deploy page (w9/m1/t003): bex's twin of Render's
 * `/web/srv-…/deploys/dep-…` — a status header above a log viewer scoped to
 * the deploy's own time window. Rendered inside the `services.$serviceId`
 * layout route's `<Outlet/>`, so the service chrome (header + tab nav) is
 * already in place; this owns only the deploy-specific content.
 */
export function DeployDetailPage({
  serviceId,
  deployId,
}: DeployDetailPageProps) {
  const { t } = useTranslations();
  const { deploy, loading, notFound } = useDeploy(serviceId, deployId);

  if (notFound) {
    return (
      <EmptyState
        iconName="SearchX"
        title={t("deploys.notFoundTitle")}
        description={t("deploys.notFoundBody", { deployId })}
      />
    );
  }

  if (loading || !deploy) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <DeployHeader deploy={deploy} />
      <DeployLogPanel
        resource={serviceId}
        startTime={deploy.createdAt ?? undefined}
        endTime={deploy.finishedAt ?? undefined}
        hasPreDeploy={!!deploy.preDeployStatus}
      />
    </div>
  );
}
