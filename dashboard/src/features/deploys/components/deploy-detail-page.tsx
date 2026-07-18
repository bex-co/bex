import { useMemo } from "react";
import { Skeleton } from "@/common/components/ui/skeleton";
import { EmptyState } from "@/common/components/empty-state";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeploy } from "../hooks/use-deploy";
import { rangeStart, type LogRange } from "../lib/log-range";
import { DeployHeader } from "./deploy-header";
import { DeployLogPanel } from "./deploy-log-panel";
import { DeployTimeline } from "./deploy-timeline";
import { DeployActions } from "./deploy-actions";

export interface DeployDetailPageProps {
  serviceId: string;
  deployId: string;
  /** The `?r=` relative log window (w9/003); undefined = the deploy's own. */
  range?: LogRange;
  /** Writes the range back to the URL (undefined clears `?r=`). */
  onRangeChange?: (range: LogRange | undefined) => void;
}

/**
 * The per-deploy page (w9/m1/t003): bex's twin of Render's
 * `/web/srv-…/deploys/dep-…` — a status header above a log viewer scoped to
 * the deploy's own time window, or to the URL's `?r=<range>` relative window
 * when one is set (w9/003). Rendered inside the `services.$serviceId` layout
 * route's `<Outlet/>`, so the service chrome (header + tab nav) is already in
 * place; this owns only the deploy-specific content.
 */
export function DeployDetailPage({
  serviceId,
  deployId,
  range,
  onRangeChange,
}: DeployDetailPageProps) {
  const { t } = useTranslations();
  const { deploy, loading, error, notFound } = useDeploy(serviceId, deployId);

  // A dead deploy id under a live service redirects to that service's Deploys
  // tab — the nearest live parent — not all the way home (w9/m55). A dead
  // SERVICE id never reaches here: the services.$serviceId layout redirects
  // before mounting its Outlet. `useDeploy` already excludes query errors
  // from `notFound`, so outages keep the inline error state below.
  const deploysTab = useMemo(
    () => ({
      to: "/services/$serviceId/deploys" as const,
      params: { serviceId },
      replace: true,
    }),
    [serviceId],
  );
  useNotFoundRedirect(notFound, deploysTab);

  // The log window: `?r=` overrides with [now - range, now) — open-ended so
  // the viewer keeps polling live, exactly like a still-running deploy.
  // Memoized on the range value so re-renders don't shift the window (and
  // re-fire the three log queries) every second.
  const window = useMemo(() => {
    if (range) return { start: rangeStart(range), end: undefined };
    return undefined;
  }, [range]);

  if (error) {
    return (
      <EmptyState
        iconName="AlertCircle"
        title={t("logs.errorTitle")}
        description={error.message}
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
      <DeployHeader
        deploy={deploy}
        actions={
          <DeployActions
            serviceId={serviceId}
            deployId={deploy.id}
            status={deploy.status}
          />
        }
      />
      <DeployTimeline serviceId={serviceId} deploy={deploy} />
      <DeployLogPanel
        resource={serviceId}
        startTime={window ? window.start : (deploy.createdAt ?? undefined)}
        endTime={window ? window.end : (deploy.finishedAt ?? undefined)}
        hasPreDeploy={!!deploy.preDeployStatus}
        followBuild={deploy.status === "build_in_progress"}
        range={range}
        onRangeChange={onRangeChange}
      />
    </div>
  );
}
