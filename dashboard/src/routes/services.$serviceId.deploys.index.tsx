import { createFileRoute } from "@tanstack/react-router";
import { DeploysListPage } from "@/features/deploys/components/deploys-list-page";
import { DeploysListSkeleton } from "@/common/components/route-skeletons";
import {
  deployListFiltersFromSearch,
  deployListFiltersToSearch,
  parseDeployListSearch,
} from "@/features/deploys/lib/deploy-list-search";

export const Route = createFileRoute("/services/$serviceId/deploys/")({
  component: RouteComponent,
  pendingComponent: DeploysListSkeleton,
  // The history's filters live in the URL, so Back from a deploy's detail
  // returns to the narrowed list (w4/284).
  validateSearch: parseDeployListSearch,
});

// The dedicated Deploys tab (w9/002): Render's standalone deploy-history
// list, distinct from the Events feed. Nests under the `services.$serviceId`
// layout route like every other tab; its rows link into the sibling
// per-deploy route ($deployId).
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
