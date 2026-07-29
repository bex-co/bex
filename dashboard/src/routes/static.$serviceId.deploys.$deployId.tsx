import { createFileRoute } from "@tanstack/react-router";
import { DeployDetailPage } from "@/features/deploys/components/deploy-detail-page";
import { parseLogRange, type LogRange } from "@/features/deploys/lib/log-range";

/**
 * Static-site per-deploy page under the /static base (w5/m57) — the twin of
 * `services.$serviceId.deploys.$deployId`, reached from a static site's Events
 * feed. Same `?r=<range>` log-window param.
 */
export const Route = createFileRoute("/static/$serviceId/deploys/$deployId")({
  component: RouteComponent,
  validateSearch: (search: Record<string, unknown>): { r?: LogRange } => {
    const r = parseLogRange(search.r);
    return r ? { r } : {};
  },
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
