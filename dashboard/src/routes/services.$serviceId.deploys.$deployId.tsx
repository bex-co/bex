import { createFileRoute } from "@tanstack/react-router";
import { DeployDetailPage } from "@/features/deploys/components/deploy-detail-page";
import {
  parseLogRange,
  type LogRange,
} from "@/features/deploys/lib/log-range";

export const Route = createFileRoute(
  "/services/$serviceId/deploys/$deployId",
)({
  component: RouteComponent,
  // `?r=<range>` (w9/003) is the log viewer's shareable relative time window
  // (Render's own deploy-page param): absent => the deploy's own
  // createdAt..finishedAt window. Optional, not `r: undefined`, so other
  // navigations to this route never have to supply it; an unrecognized value
  // just falls back to the deploy window.
  validateSearch: (search: Record<string, unknown>): { r?: LogRange } => {
    const r = parseLogRange(search.r);
    return r ? { r } : {};
  },
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Deploy · bex dashboard` }],
  }),
});

// The per-deploy page (w9/m1): Render's `/web/srv-…/deploys/dep-…` twin.
// Nests under the `services.$serviceId` layout route, so the service header +
// tab nav stay in place — only the deploy-specific header + log viewer render
// into the shared `<Outlet/>`.
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
