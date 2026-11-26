import { createFileRoute } from "@tanstack/react-router";
import { DeployDetailPage } from "@/features/deploys/components/deploy-detail-page";
import { parseLogRange, type LogRange } from "@/features/deploys/lib/log-range";
import { DeployDetailSkeleton } from "@/common/components/route-skeletons";
import { prefetchDeployPage } from "@/features/deploys/lib/deploy-page-prefetch";

export const Route = createFileRoute("/services/$serviceId/deploys/$deployId")({
  component: RouteComponent,
  pendingComponent: DeployDetailSkeleton,
  // The page mounts with the deploy and its Notices read (w1/m659).
  loader: ({ context, params }) =>
    prefetchDeployPage(context.client, params.serviceId, params.deployId),
  // `?r=<range>` (w9/003) is the log viewer's shareable relative time window
  // (Render's own deploy-page param): absent => the deploy's own
  // createdAt..finishedAt window. The annotated `r?` keeps it optional for
  // other navigations. An unrecognized value is undefined rather than omitted,
  // since the router merges this over the raw search: an omitted `?r=12h`
  // reached the log viewer and threw on its range (w5/078).
  validateSearch: (search: Record<string, unknown>): { r?: LogRange } => ({
    r: parseLogRange(search.r),
  }),
});

// The per-deploy page (w9/m1): Render's `/web/srv-…/deploys/dep-…` twin.
// Nests under the `services.$serviceId` layout route, so the sidebar's tab nav
// stays in place — only the deploy-specific header + log viewer render into
// the shared `<Outlet/>`. The layout leaves the service header out here, as
// Render does on a deploy's page (w3/106).
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
        // `replace` keeps range tweaks out of the back-button history — the
        // workspace.settings `?plan=change` precedent.
        void navigate({ search: range ? { r: range } : {}, replace: true })
      }
    />
  );
}
