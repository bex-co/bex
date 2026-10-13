import { createFileRoute } from "@tanstack/react-router";
import { DeploysListPage } from "@/features/deploys/components/deploys-list-page";
import { DeploysListSkeleton } from "@/common/components/route-skeletons";
import {
  deployListFiltersFromSearch,
  deployListFiltersToSearch,
  parseDeployListSearch,
} from "@/features/deploys/lib/deploy-list-search";

/**
 * A static_site has no Deploys tab in its sidebar (Render parity, w5/m57), but
 * its deploy history stays reachable under /static so the Events feed's deploy
 * links and the deploy-detail back button resolve within the /static tree
 * (loop-free) instead of bouncing to /services.
 */
export const Route = createFileRoute("/static/$serviceId/deploys/")({
  component: RouteComponent,
  pendingComponent: DeploysListSkeleton,
  // The history's filters live in the URL, so Back from a deploy's detail
  // returns to the narrowed list (w4/284).
  validateSearch: parseDeployListSearch,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <DeploysListPage
      // Filters belong to one service's history.
      key={serviceId}
      serviceId={serviceId}
      urlFilters={deployListFiltersFromSearch(search)}
      onFiltersChange={(filters) =>
        // Replace, so typing adds no history entries; the functional form
        // commits the explicit undefineds, which is what clears a key.
        void navigate({
          search: () => deployListFiltersToSearch(filters),
          replace: true,
        })
      }
    />
  );
}
