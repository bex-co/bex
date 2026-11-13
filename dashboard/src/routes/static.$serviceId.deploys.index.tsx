import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import {
  DeployHistorySkeleton,
  DeployHistoryView,
} from "@/features/deploys/components/deploy-history-view";
import { parseDeployListSearch } from "@/features/deploys/lib/deploy-list-search";

/**
 * A static_site has no Deploys tab in its sidebar (Render parity, w5/m57), but
 * its deploy history stays reachable under /static so the Events feed's deploy
 * links and the deploy-detail back button resolve within the /static tree
 * (loop-free) instead of bouncing to /services.
 */
export const Route = createFileRoute("/static/$serviceId/deploys/")({
  head: serviceTabHead("services.navDeploys", "/static/$serviceId"),
  component: RouteComponent,
  pendingComponent: DeployHistorySkeleton,
  // The history's filters live in the URL, so Back from a deploy's detail
  // returns to the narrowed list (w4/284).
  validateSearch: parseDeployListSearch,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <DeployHistoryView
      base={"/static"}
      serviceId={serviceId}
      search={search}
      // The functional form commits the explicit undefineds, which is what
      // clears a key.
      replaceSearch={(next) =>
        void navigate({ search: () => next, replace: true })
      }
    />
  );
}
