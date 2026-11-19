import { useParams, useRouterState } from "@tanstack/react-router";

import { DeploysListSkeleton } from "@/common/components/route-skeletons";
import { AutoDeployOffBanner } from "@/features/deploys/components/auto-deploy-off-banner";
import {
  autoDeployOff,
  deploysManualOnly,
} from "@/features/deploys/lib/auto-deploy-off";
import { useServer } from "@/features/services/hooks/use-server";
import type { ServiceBase } from "@/features/services/lib/service-base";

/**
 * The deploy history's pending state: the list's skeleton, below the same
 * banner the ready page shows when the cached service has auto-deploy off
 * (the skeleton-matches-content rule, dashboard/AGENTS.md).
 *
 * Its own module, apart from `DeployHistoryView`: the routes keep their
 * `pendingComponent` in the route tree loaded up front (w3/066), and only the
 * skeleton, not the history page, should come with it.
 */
export function DeployHistorySkeleton() {
  const { serviceId = "" } = useParams({ strict: false }) as {
    serviceId?: string;
  };
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const base: ServiceBase = pathname.startsWith("/static/")
    ? "/static"
    : "/services";
  const { service } = useServer(serviceId, { poll: false });
  return (
    <DeploysListSkeleton
      commits={!service || Boolean(service.repo)}
      banner={
        autoDeployOff(service) ? (
          <AutoDeployOffBanner
            serviceId={serviceId}
            branch={service?.branch ?? null}
            base={base}
            manual={deploysManualOnly(service)}
          />
        ) : undefined
      }
    />
  );
}
