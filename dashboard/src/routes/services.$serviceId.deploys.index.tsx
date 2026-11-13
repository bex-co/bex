import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import {
  DeployHistorySkeleton,
  DeployHistoryView,
} from "@/features/deploys/components/deploy-history-view";
import { redirectHistoryTabByType } from "@/features/services/lib/service-root-redirect";
import { parseDeployListSearch } from "@/features/deploys/lib/deploy-list-search";

export const Route = createFileRoute("/services/$serviceId/deploys/")({
  head: serviceTabHead("services.navDeploys", "/services/$serviceId"),
  // A cron job's deploy history is its Builds page (w2/m360).
  beforeLoad: ({ context, params }) =>
    redirectHistoryTabByType(context.client, params.serviceId, "deploys"),
  component: RouteComponent,
  pendingComponent: DeployHistorySkeleton,
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
    <DeployHistoryView
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
