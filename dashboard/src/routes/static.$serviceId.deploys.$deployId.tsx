import { createFileRoute } from "@tanstack/react-router";
import { DeployDetailPage } from "@/features/deploys/components/deploy-detail-page";
import { parseLogRange, type LogRange } from "@/features/deploys/lib/log-range";
import { DeployDetailSkeleton } from "@/common/components/route-skeletons";
import { prefetchDeployPage } from "@/features/deploys/lib/deploy-page-prefetch";

/**
 * Static-site per-deploy page under the /static base (w5/m57) — the twin of
 * `services.$serviceId.deploys.$deployId`, reached from a static site's Events
 * feed. Same `?r=<range>` log-window param.
 */
export const Route = createFileRoute("/static/$serviceId/deploys/$deployId")({
  component: RouteComponent,
  pendingComponent: DeployDetailSkeleton,
  // The page mounts with the deploy and its Notices read (w1/m659).
  loader: ({ context, params }) =>
    prefetchDeployPage(context.client, params.serviceId, params.deployId),
  validateSearch: (search: Record<string, unknown>): { r?: LogRange } => ({
    r: parseLogRange(search.r),
  }),
});

function RouteComponent() {
  const { serviceId, deployId } = Route.useParams();
  const { r } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <DeployDetailPage
      serviceId={serviceId}
      deployId={deployId}
      range={r}
      onRangeChange={(range) =>
        void navigate({ search: range ? { r: range } : {}, replace: true })
      }
    />
  );
}
